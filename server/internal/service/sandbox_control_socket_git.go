package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/controlsocket"
	servergit "github.com/obot-platform/discobot/server/internal/git"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/store"
)

type serverGitStream struct {
	body *io.PipeWriter
	done chan struct{}
}

type gitHTTPRequest struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type gitHTTPResponse struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type controlSocketRun struct {
	cancel context.CancelFunc
}

// SandboxControlSocketService owns server-initiated control socket lifecycle
// and frame handling for sandbox sessions.
type SandboxControlSocketService struct {
	store          *store.Store
	sandboxService *SandboxService

	runs   map[string]*controlSocketRun
	runsMu sync.Mutex
}

func NewSandboxControlSocketService(store *store.Store, sandboxService *SandboxService) *SandboxControlSocketService {
	return &SandboxControlSocketService{
		store:          store,
		sandboxService: sandboxService,
		runs:           make(map[string]*controlSocketRun),
	}
}

func (s *SandboxControlSocketService) HandleSandboxEvent(ctx context.Context, event sandbox.StateEvent, session *model.Session) {
	if session == nil {
		return
	}
	switch event.Status {
	case sandbox.StatusRunning:
		if !s.sessionSupportsGitControlSocket(ctx, session) {
			s.StopControlSocket(event.SessionID)
			return
		}
		s.StartControlSocket(session.ProjectID, event.SessionID)
	case sandbox.StatusStopped, sandbox.StatusFailed, sandbox.StatusRemoved:
		s.StopControlSocket(event.SessionID)
	}
}

func (s *SandboxControlSocketService) sessionSupportsGitControlSocket(ctx context.Context, session *model.Session) bool {
	if s == nil || s.store == nil || session == nil {
		return false
	}
	workspacePath := ""
	if session.WorkspacePath != nil {
		workspacePath = strings.TrimSpace(*session.WorkspacePath)
	}
	workspace, err := s.store.GetWorkspaceByID(ctx, session.WorkspaceID)
	if err != nil {
		log.Printf("skip sandbox control socket for session %s: load workspace: %v", session.ID, err)
		return false
	}
	return sandboxGitControlSocketEnabled(workspace, workspacePath)
}

// StartControlSocket keeps the server-initiated sandbox control socket running
// for a ready session. It is idempotent per session and retries transient
// disconnects until StopControlSocket is called.
func (s *SandboxControlSocketService) StartControlSocket(projectID, sessionID string) {
	if s == nil || projectID == "" || sessionID == "" {
		return
	}

	s.runsMu.Lock()
	if s.runs == nil {
		s.runs = make(map[string]*controlSocketRun)
	}
	if _, ok := s.runs[sessionID]; ok {
		s.runsMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &controlSocketRun{cancel: cancel}
	s.runs[sessionID] = run
	s.runsMu.Unlock()

	go func() {
		defer func() {
			s.runsMu.Lock()
			if s.runs[sessionID] == run {
				delete(s.runs, sessionID)
			}
			s.runsMu.Unlock()
		}()

		delay := time.Second
		for {
			if err := s.runControlSocketOnce(ctx, projectID, sessionID); err != nil {
				if ctx.Err() == nil {
					log.Printf("sandbox control socket for session %s disconnected: %v", sessionID, err)
				}
			} else {
				return
			}
			if ctx.Err() != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay < 30*time.Second {
				delay *= 2
			}
		}
	}()
}

// StopControlSocket stops the background control socket loop for a session.
func (s *SandboxControlSocketService) StopControlSocket(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}
	s.runsMu.Lock()
	run := s.runs[sessionID]
	delete(s.runs, sessionID)
	s.runsMu.Unlock()
	if run != nil {
		run.cancel()
	}
}

func (s *SandboxControlSocketService) runControlSocketOnce(ctx context.Context, projectID, sessionID string) error {
	if s.sandboxService == nil {
		return fmt.Errorf("sandbox service is not configured")
	}
	session, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if !s.sessionSupportsGitControlSocket(ctx, session) {
		return nil
	}
	conn, err := s.sandboxService.dialControlSocket(ctx, sessionID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	return s.HandleControlSocketFrames(ctx, conn.Conn, projectID, sessionID)
}

// HandleControlSocketFrames processes frames on a server-initiated sandbox
// control WebSocket connected to the agent-api /control/ws endpoint.
func (s *SandboxControlSocketService) HandleControlSocketFrames(ctx context.Context, conn *controlsocket.Conn, projectID, sessionID string) error {
	conn.CloseOnDone(ctx)

	streams := map[string]*serverGitStream{}
	var streamsMu sync.Mutex
	defer func() {
		streamsMu.Lock()
		defer streamsMu.Unlock()
		for _, stream := range streams {
			_ = stream.body.CloseWithError(context.Canceled)
		}
	}()

	for {
		frame, err := conn.ReadFrame(ctx)
		if err != nil {
			return err
		}
		if frame.Channel == controlsocket.ChannelControl {
			continue
		}
		if !strings.HasPrefix(frame.Channel, "git:") {
			continue
		}

		switch frame.Type {
		case controlsocket.TypeStreamOpen:
			var start gitHTTPRequest
			if err := json.Unmarshal(frame.Payload, &start); err != nil {
				_ = conn.WriteFrame(ctx, controlsocket.Frame{Channel: frame.Channel, Type: controlsocket.TypeError, Payload: controlsocket.Payload(map[string]string{"error": err.Error()})})
				continue
			}
			reader, writer := io.Pipe()
			stream := &serverGitStream{body: writer, done: make(chan struct{})}
			streamsMu.Lock()
			streams[frame.Channel] = stream
			streamsMu.Unlock()

			go func(channel string) {
				defer close(stream.done)
				if err := s.serveTunneledGitHTTP(ctx, conn, projectID, sessionID, start, reader, channel); err != nil {
					_ = conn.WriteFrame(ctx, controlsocket.Frame{Channel: channel, Type: controlsocket.TypeError, Payload: controlsocket.Payload(map[string]string{"error": err.Error()})})
				}
				streamsMu.Lock()
				delete(streams, channel)
				streamsMu.Unlock()
			}(frame.Channel)

		case controlsocket.TypeStreamData:
			streamsMu.Lock()
			stream := streams[frame.Channel]
			streamsMu.Unlock()
			if stream != nil && len(frame.Data) > 0 {
				_, _ = stream.body.Write(frame.Data)
			}

		case controlsocket.TypeStreamCloseWrite:
			streamsMu.Lock()
			stream := streams[frame.Channel]
			streamsMu.Unlock()
			if stream != nil {
				_ = stream.body.Close()
			}

		case controlsocket.TypeStreamClose:
			streamsMu.Lock()
			stream := streams[frame.Channel]
			delete(streams, frame.Channel)
			streamsMu.Unlock()
			if stream != nil {
				_ = stream.body.CloseWithError(context.Canceled)
			}
		}
	}
}

func (s *SandboxControlSocketService) serveTunneledGitHTTP(ctx context.Context, conn *controlsocket.Conn, projectID, sessionID string, start gitHTTPRequest, body io.Reader, channel string) error {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if sess.ProjectID != projectID {
		return fmt.Errorf("session does not belong to project")
	}
	workspace, err := s.store.GetWorkspaceByID(ctx, sess.WorkspaceID)
	if err != nil {
		return fmt.Errorf("load workspace: %w", err)
	}
	if !isLocalWorkspaceSourceType(workspace.SourceType) || servergit.IsGitURL(workspace.Path) {
		return fmt.Errorf("git control socket only supports local filesystem workspaces")
	}
	workspacePath := ""
	if sess.WorkspacePath != nil {
		workspacePath = strings.TrimSpace(*sess.WorkspacePath)
	}
	if workspacePath == "" {
		workspacePath = workspace.Path
	}
	if workspacePath == "" {
		return fmt.Errorf("workspace path is empty")
	}

	err = runGitHTTPBackend(ctx, workspacePath, sessionID, start, body,
		func(status int, headers map[string][]string) error {
			return conn.WriteFrame(ctx, controlsocket.Frame{Channel: channel, Type: controlsocket.TypeStreamOpen, Payload: controlsocket.Payload(gitHTTPResponse{Status: status, Headers: headers})})
		},
		func(chunk []byte) error {
			if len(chunk) == 0 {
				return nil
			}
			return conn.WriteFrame(ctx, controlsocket.Frame{Channel: channel, Type: controlsocket.TypeStreamData, Data: chunk})
		},
	)
	if err != nil {
		return err
	}
	return conn.WriteFrame(ctx, controlsocket.Frame{Channel: channel, Type: controlsocket.TypeStreamCloseWrite})
}

func runGitHTTPBackend(ctx context.Context, workspacePath, sessionID string, start gitHTTPRequest, body io.Reader, onStart func(int, map[string][]string) error, onBody func([]byte) error) error {
	workspacePath, err := filepath.Abs(workspacePath)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "http-backend")
	cmd.Stdin = body
	cmd.Env = gitHTTPBackendEnv(workspacePath, sessionID, start)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git http-backend: %w", err)
	}

	reader := bufio.NewReader(stdout)
	status, headers, err := readCGIHeaders(reader)
	if err == nil {
		err = onStart(status, headers)
	}
	if err == nil {
		buf := make([]byte, 64*1024)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				if err = onBody(buf[:n]); err != nil {
					break
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				err = readErr
				break
			}
		}
	}

	waitErr := cmd.Wait()
	if err != nil {
		return err
	}
	if waitErr != nil {
		return fmt.Errorf("git http-backend: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitHTTPBackendEnv(workspacePath, sessionID string, start gitHTTPRequest) []string {
	projectRoot := filepath.ToSlash(filepath.Dir(workspacePath))
	repoName := filepath.Base(workspacePath)
	pathInfo := "/" + repoName + gitSuffixPath(start.Path)

	env := append(os.Environ(),
		"GIT_PROJECT_ROOT="+projectRoot,
		"GIT_HTTP_EXPORT_ALL=1",
		"REQUEST_METHOD="+start.Method,
		"PATH_INFO="+pathInfo,
		"QUERY_STRING="+start.Query,
		"REMOTE_USER=discobot-agent",
		"GIT_CONFIG_COUNT=5",
		"GIT_CONFIG_KEY_0=http.receivepack",
		"GIT_CONFIG_VALUE_0=true",
		"GIT_CONFIG_KEY_1=receive.hideRefs",
		"GIT_CONFIG_VALUE_1=refs/",
		"GIT_CONFIG_KEY_2=receive.hideRefs",
		"GIT_CONFIG_VALUE_2=!"+sessionPushRef(sessionID),
		"GIT_CONFIG_KEY_3=receive.denyDeletes",
		"GIT_CONFIG_VALUE_3=true",
		"GIT_CONFIG_KEY_4=receive.denyNonFastForwards",
		"GIT_CONFIG_VALUE_4=true",
	)
	if contentType := firstHeader(start.Headers, "Content-Type"); contentType != "" {
		env = append(env, "CONTENT_TYPE="+contentType)
	}
	if contentLength := firstHeader(start.Headers, "Content-Length"); contentLength != "" {
		env = append(env, "CONTENT_LENGTH="+contentLength)
	}
	if protocol := firstHeader(start.Headers, "Git-Protocol"); protocol != "" {
		env = append(env, "HTTP_GIT_PROTOCOL="+protocol)
	}
	if contentEncoding := firstHeader(start.Headers, "Content-Encoding"); contentEncoding != "" {
		env = append(env, "HTTP_CONTENT_ENCODING="+contentEncoding)
	}
	return env
}

func readCGIHeaders(reader *bufio.Reader) (int, map[string][]string, error) {
	status := http.StatusOK
	headers := map[string][]string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, nil, fmt.Errorf("git backend response missing CGI headers: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return status, headers, nil
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.EqualFold(key, "Status") {
			code, _, _ := strings.Cut(value, " ")
			if parsed, err := strconv.Atoi(code); err == nil {
				status = parsed
			}
			continue
		}
		headers[key] = append(headers[key], value)
	}
}

func parseCGIResponse(output []byte) (int, map[string][]string, []byte, error) {
	sep := bytes.Index(output, []byte("\r\n\r\n"))
	sepLen := 4
	if sep < 0 {
		sep = bytes.Index(output, []byte("\n\n"))
		sepLen = 2
	}
	if sep < 0 {
		return 0, nil, nil, errors.New("git backend response missing CGI headers")
	}
	status, headers, err := readCGIHeaders(bufio.NewReader(bytes.NewReader(output[:sep+sepLen])))
	if err != nil {
		return 0, nil, nil, err
	}
	return status, headers, output[sep+sepLen:], nil
}

func gitSuffixPath(path string) string {
	if i := strings.Index(path, ".git"); i >= 0 {
		path = path[i+len(".git"):]
	}
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	return path
}

func sessionPushRef(sessionID string) string {
	return "refs/heads/discobot/" + sessionID
}

func firstHeader(headers map[string][]string, name string) string {
	for key, values := range headers {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
