package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/service"
	"github.com/obot-platform/discobot/server/internal/terminal"
)

// Minimum terminal dimensions to prevent zero-size PTY
const (
	minTermRows = 20
	minTermCols = 80

	terminalWorkspaceDir = "/home/discobot/workspace"
)

// TerminalWebSocket handles WebSocket terminal connections.
//
// Each (sandboxSession, user) pair has one persistent PTY managed by
// h.terminalManager. Navigating away and returning reconnects to the same
// shell — the output buffer is replayed so the client sees recent history.
func (h *Handler) TerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "session ID is required")
		return
	}

	if h.sandboxService == nil {
		h.Error(w, http.StatusServiceUnavailable, "sandbox provider not available")
		return
	}

	// Get terminal dimensions from query params, enforcing minimum size
	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	if rows < minTermRows {
		rows = minTermRows
	}
	if cols < minTermCols {
		cols = minTermCols
	}

	// Check if root access is requested
	runAsRoot := r.URL.Query().Get("root") == "true"
	workDir := terminalWorkDir(r)

	ctx := r.Context()

	// Get sandbox client (ensures sandbox is ready and container is running)
	client, err := h.sandboxService.GetClient(ctx, sessionID)
	if err != nil {
		log.Printf("failed to ensure sandbox ready for session %s: %v", sessionID, err)
		h.Error(w, http.StatusInternalServerError, "failed to start sandbox")
		return
	}

	// Determine user for terminal session
	var user string
	if runAsRoot {
		user = "root"
	} else {
		// Get default user from sandbox (uses UID:GID format for compatibility)
		userInfo, err := client.GetUserInfo(ctx)
		if err != nil {
			log.Printf("failed to get user info, falling back to root: %v", err)
			user = "root"
		} else {
			user = strconv.Itoa(userInfo.UID) + ":" + strconv.Itoa(userInfo.GID)
		}
	}

	// Fetch environment variables for the terminal session.
	// Visible credentials are exposed to the terminal. Failures are non-fatal;
	// the terminal still opens without them.
	envVars := map[string]string{}
	if h.credentialService != nil {
		credentialVars, err := h.credentialService.GetVisibleEnvVarsForSession(ctx, sessionID, service.CredentialVisibilityContextConsole)
		if err != nil {
			log.Printf("failed to get visible credential env vars for session %s: %v", sessionID, err)
		} else {
			maps.Copy(envVars, credentialVars)
		}
	}
	// Get or create the persistent terminal session for this (sandbox, user) pair.
	// If one already exists (from a previous WebSocket connection) it is reused —
	// the caller never sees the PTY directly, only a subscriber channel.
	termKey := terminalReuseKey(sessionID, user, workDir)
	termSession, err := h.terminalManager.GetOrCreate(ctx, termKey, func(ctx context.Context) (sandbox.PTY, error) {
		consoleSudoToken, err := secureRandomHex(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate console sudo token: %w", err)
		}

		terminalEnv := maps.Clone(envVars)
		terminalEnv["DISCOBOT_SUDO_RUNTIME"] = "console"
		terminalEnv["DISCOBOT_SUDO_TOKEN"] = consoleSudoToken

		execUser := ""
		if runAsRoot {
			execUser = "root"
		}
		return h.sandboxService.AttachTerminal(ctx, sessionID, rows, cols, execUser, workDir, termKey, terminalEnv)
	})
	if err != nil {
		log.Printf("failed to attach to sandbox PTY: %v", err)
		h.Error(w, http.StatusInternalServerError, "failed to attach to terminal")
		return
	}

	// Resize to the connecting client's viewport. On first connect this is a
	// no-op (PTY was just created with these dimensions). On reconnect it
	// ensures the shell matches the current browser window size.
	if err := termSession.Resize(ctx, rows, cols); err != nil {
		log.Printf("PTY resize on connect: %v", err)
	}

	// Upgrade to WebSocket (after all validation so we don't upgrade then fail)
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("failed to upgrade websocket: %v", err)
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	// Subscribe to the session. This returns a channel pre-loaded with the
	// output buffer (recent history) followed by live PTY output.
	sub := termSession.Subscribe()
	defer termSession.Unsubscribe(sub)

	handlePersistentTerminalSession(ctx, termSession, sub, conn)
}

func terminalWorkDir(r *http.Request) string {
	if r.URL.Query().Get("workdir") == "workspace" {
		return terminalWorkspaceDir
	}
	return ""
}

func terminalReuseKey(sessionID, user, workDir string) string {
	key := sessionID + ":" + user
	if workDir != "" {
		key += ":" + workDir
	}
	return key
}

func secureRandomHex(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		return "", fmt.Errorf("random byte length must be positive")
	}
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// handlePersistentTerminalSession relays data between a persistent terminal
// session and a WebSocket connection.
//
// The function returns (and the WebSocket is closed) when:
//   - The WebSocket client disconnects (input goroutine exits first).
//   - The PTY process exits (output goroutine exits first, sets a read deadline
//     so the input goroutine unblocks within one second).
//
// The PTY itself is NOT closed when the WebSocket disconnects; it keeps
// running so the next WebSocket connection can reattach to the same shell.
func handlePersistentTerminalSession(ctx context.Context, sess *terminal.Session, sub terminal.Subscriber, conn *websocket.Conn) {
	wsWriteDone := make(chan struct{})

	// Session output → WebSocket.
	// Sends the buffered history first, then streams live output until the
	// subscriber channel is closed (PTY exited) or a WebSocket write fails.
	go func() {
		defer close(wsWriteDone)
		for chunk := range sub {
			msg := api.TerminalMessage{Type: "output", Data: string(chunk)}
			if err := wsjson.Write(ctx, conn, msg); err != nil {
				// WebSocket write failed (client disconnected); stop sending.
				return
			}
		}
		// sub channel closed → PTY exited; send a clean close frame.
		_ = conn.Close(websocket.StatusNormalClosure, "shell exited")
	}()

	// WebSocket → session input.
	// Reads input and resize messages from the api. Exits when the client
	// closes the WebSocket or a network error occurs (including a read deadline).
	// Does NOT close the PTY — only the subscriber is cleaned up (via defer).
	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		for {
			var msg api.TerminalMessage
			if err := wsjson.Read(ctx, conn, &msg); err != nil {
				status := websocket.CloseStatus(err)
				if status != websocket.StatusNormalClosure &&
					status != websocket.StatusGoingAway &&
					status != websocket.StatusAbnormalClosure &&
					ctx.Err() == nil {
					log.Printf("terminal: WebSocket read error: %v", err)
				}
				return
			}

			switch msg.Type {
			case "input":
				var input string
				if err := unmarshalTerminalMessageData(msg.Data, &input); err != nil {
					log.Printf("terminal: failed to unmarshal input: %v", err)
					continue
				}
				if err := sess.Write([]byte(input)); err != nil {
					log.Printf("terminal: PTY write error: %v", err)
					return
				}

			case "resize":
				var resize api.ResizeData
				if err := unmarshalTerminalMessageData(msg.Data, &resize); err != nil {
					log.Printf("terminal: failed to unmarshal resize: %v", err)
					continue
				}
				if resize.Cols < minTermCols {
					resize.Cols = minTermCols
				}
				if resize.Rows < minTermRows {
					resize.Rows = minTermRows
				}
				if err := sess.Resize(ctx, resize.Rows, resize.Cols); err != nil {
					log.Printf("terminal: PTY resize error: %v", err)
				}
			}
		}
	}()

	// Wait for either side to finish.
	select {
	case <-wsWriteDone:
		// PTY exited: the close frame has been sent, so wait for the input
		// goroutine to observe the closed websocket and stop.
		<-inputDone
	case <-inputDone:
		// Client disconnected: wait for the output goroutine to finish.
		<-wsWriteDone
	}
}

func unmarshalTerminalMessageData(data any, target any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

// GetTerminalHistory returns terminal history for a session
func (h *Handler) GetTerminalHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "session ID is required")
		return
	}

	// Get limit from query params, default to 100
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}

	ctx := r.Context()
	history, err := h.store.ListTerminalHistory(ctx, sessionID, limit)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "failed to get terminal history")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"history": history})
}

// GetTerminalStatus returns the sandbox status
func (h *Handler) GetTerminalStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "session ID is required")
		return
	}

	if h.sandboxService == nil {
		h.JSON(w, http.StatusOK, map[string]string{
			"status": "unavailable",
			"error":  "sandbox provider not configured",
		})
		return
	}

	ctx := r.Context()
	sb, err := h.sandboxService.GetForSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sandbox.ErrNotFound) {
			h.JSON(w, http.StatusOK, map[string]string{"status": "not_created"})
			return
		}
		h.Error(w, http.StatusInternalServerError, "failed to get sandbox status")
		return
	}

	response := map[string]any{
		"status":    string(sb.Status),
		"image":     sb.Image,
		"createdAt": sb.CreatedAt.Format(time.RFC3339),
	}
	if sb.StartedAt != nil {
		response["startedAt"] = sb.StartedAt.Format(time.RFC3339)
	}
	if sb.StoppedAt != nil {
		response["stoppedAt"] = sb.StoppedAt.Format(time.RFC3339)
	}
	if sb.Error != "" {
		response["error"] = sb.Error
	}

	h.JSON(w, http.StatusOK, response)
}
