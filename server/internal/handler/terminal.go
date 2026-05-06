package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/service"
	"github.com/obot-platform/discobot/server/internal/terminal"
)

// Minimum terminal dimensions to prevent zero-size PTY
const (
	minTermRows = 20
	minTermCols = 80
)

// upgrader configures the WebSocket upgrader.
// Origin checking is handled by the CORS middleware in the router,
// so we allow all origins here to avoid duplicate validation.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true // CORS middleware handles origin validation
	},
}

// TerminalMessage represents a message sent over the WebSocket
type TerminalMessage struct {
	Type string          `json:"type"` // "input", "output", "resize", "error"
	Data json.RawMessage `json:"data,omitempty"`
}

// ResizeData contains terminal resize dimensions
type ResizeData struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

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
	termKey := sessionID + ":" + user
	termSession, err := h.terminalManager.GetOrCreate(ctx, termKey, func(ctx context.Context) (sandbox.PTY, error) {
		execUser := ""
		if runAsRoot {
			execUser = "root"
		}
		return h.attachAgentExec(ctx, sessionID, rows, cols, execUser, termKey, envVars)
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// Subscribe to the session. This returns a channel pre-loaded with the
	// output buffer (recent history) followed by live PTY output.
	sub := termSession.Subscribe()
	defer termSession.Unsubscribe(sub)

	handlePersistentTerminalSession(ctx, termSession, sub, conn)
}

type agentExecCreateRequest struct {
	Kind     string            `json:"kind,omitempty"`
	Name     string            `json:"name,omitempty"`
	ReuseKey string            `json:"reuseKey,omitempty"`
	Cmd      []string          `json:"cmd,omitempty"`
	WorkDir  string            `json:"workDir,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	User     string            `json:"user,omitempty"`
	TTY      bool              `json:"tty,omitempty"`
	Rows     int               `json:"rows,omitempty"`
	Cols     int               `json:"cols,omitempty"`
}

type agentExecSession struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exitCode,omitempty"`
}

type agentExecResizeRequest struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

type agentExecPTY struct {
	sessionID string
	execID    string
	lease     *sandbox.HTTPClientLease
	conn      *websocket.Conn
	readBuf   []byte
	readMu    sync.Mutex
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func (h *Handler) attachAgentExec(ctx context.Context, sessionID string, rows, cols int, user, reuseKey string, env map[string]string) (sandbox.PTY, error) {
	lease, err := h.sandboxService.AcquireHTTPClient(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	execSession, err := createAgentExec(ctx, lease.Client, agentExecCreateRequest{
		Kind:     "user",
		Name:     "terminal",
		ReuseKey: "terminal:" + reuseKey,
		Env:      env,
		User:     user,
		TTY:      true,
		Rows:     rows,
		Cols:     cols,
	})
	if err != nil {
		lease.Release()
		return nil, err
	}

	conn, _, err := agentExecDialer(lease.Client).DialContext(ctx, "ws://sandbox/exec/"+url.PathEscape(execSession.ID)+"/attach", nil)
	if err != nil {
		lease.Release()
		return nil, err
	}

	return &agentExecPTY{
		sessionID: sessionID,
		execID:    execSession.ID,
		lease:     lease,
		conn:      conn,
	}, nil
}

func createAgentExec(ctx context.Context, client *http.Client, payload agentExecCreateRequest) (*agentExecSession, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://sandbox/exec", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("agent exec create failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var session agentExecSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func agentExecDialer(client *http.Client) *websocket.Dialer {
	dialer := *websocket.DefaultDialer
	if transport, ok := client.Transport.(*http.Transport); ok && transport.DialContext != nil {
		dialer.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return transport.DialContext(ctx, network, addr)
		}
	}
	return &dialer
}

func (p *agentExecPTY) Read(buf []byte) (int, error) {
	p.readMu.Lock()
	defer p.readMu.Unlock()
	if len(p.readBuf) > 0 {
		n := copy(buf, p.readBuf)
		p.readBuf = p.readBuf[n:]
		return n, nil
	}
	for {
		msgType, payload, err := p.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
			continue
		}
		n := copy(buf, payload)
		p.readBuf = append(p.readBuf[:0], payload[n:]...)
		return n, nil
	}
}

func (p *agentExecPTY) Write(data []byte) (int, error) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if err := p.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (p *agentExecPTY) Resize(ctx context.Context, rows, cols int) error {
	payload, err := json.Marshal(agentExecResizeRequest{Rows: rows, Cols: cols})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://sandbox/exec/"+url.PathEscape(p.execID)+"/resize", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.lease.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("agent exec resize failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func (p *agentExecPTY) Close() error {
	var err error
	p.closeOnce.Do(func() {
		err = p.conn.Close()
		req, reqErr := http.NewRequest(http.MethodPost, "http://sandbox/exec/"+url.PathEscape(p.execID)+"/kill", nil)
		if reqErr == nil {
			if resp, doErr := p.lease.Client.Do(req); doErr == nil {
				_ = resp.Body.Close()
			}
		}
		p.lease.Release()
	})
	return err
}

func (p *agentExecPTY) Wait(ctx context.Context) (int, error) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		session, err := getAgentExec(ctx, p.lease.Client, p.execID)
		if err != nil {
			return -1, err
		}
		switch session.Status {
		case "exited", "killed", "failed":
			if session.ExitCode != nil {
				return *session.ExitCode, nil
			}
			return 0, nil
		}
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-ticker.C:
		}
	}
}

func getAgentExec(ctx context.Context, client *http.Client, execID string) (*agentExecSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://sandbox/exec/"+url.PathEscape(execID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("agent exec get failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var session agentExecSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, err
	}
	return &session, nil
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
			data, err := json.Marshal(string(chunk))
			if err != nil {
				log.Printf("terminal: JSON marshal error: %v", err)
				return
			}
			msg := TerminalMessage{Type: "output", Data: json.RawMessage(data)}
			if err := conn.WriteJSON(msg); err != nil {
				// WebSocket write failed (client disconnected); stop sending.
				return
			}
		}
		// sub channel closed → PTY exited; send a clean close frame.
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shell exited")
		_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	}()

	// WebSocket → session input.
	// Reads input and resize messages from the client. Exits when the client
	// closes the WebSocket or a network error occurs (including a read deadline).
	// Does NOT close the PTY — only the subscriber is cleaned up (via defer).
	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		for {
			var msg TerminalMessage
			if err := conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure) {
					log.Printf("terminal: WebSocket read error: %v", err)
				}
				return
			}

			switch msg.Type {
			case "input":
				var input string
				if err := json.Unmarshal(msg.Data, &input); err != nil {
					log.Printf("terminal: failed to unmarshal input: %v", err)
					continue
				}
				if err := sess.Write([]byte(input)); err != nil {
					log.Printf("terminal: PTY write error: %v", err)
					return
				}

			case "resize":
				var resize ResizeData
				if err := json.Unmarshal(msg.Data, &resize); err != nil {
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
		// PTY exited: the close frame has been sent. Set a short read deadline so
		// the input goroutine unblocks (either after the client sends a close ack
		// or after the deadline expires) and then wait for it to finish.
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		<-inputDone
	case <-inputDone:
		// Client disconnected: wait for the output goroutine to finish.
		<-wsWriteDone
	}
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
