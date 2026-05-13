package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/obot-platform/discobot/controlsocket"
	"github.com/obot-platform/discobot/server/internal/model"
)

func TestGitHTTPBackendEnvRestrictsPushRef(t *testing.T) {
	env := gitHTTPBackendEnv("/tmp/workspace/repo", "session-123", gitHTTPRequest{
		Method: "POST",
		Path:   "/workspace.git/git-receive-pack",
		Query:  "service=git-receive-pack",
		Headers: map[string][]string{
			"Content-Type":     {"application/x-git-receive-pack-request"},
			"Content-Length":   {"123"},
			"Content-Encoding": {"gzip"},
			"Git-Protocol":     {"version=2"},
		},
	})

	values := envValues(env)
	want := map[string]string{
		"GIT_PROJECT_ROOT":      "/tmp/workspace",
		"GIT_HTTP_EXPORT_ALL":   "1",
		"REQUEST_METHOD":        "POST",
		"PATH_INFO":             "/repo/git-receive-pack",
		"QUERY_STRING":          "service=git-receive-pack",
		"REMOTE_USER":           "discobot-agent",
		"CONTENT_TYPE":          "application/x-git-receive-pack-request",
		"CONTENT_LENGTH":        "123",
		"HTTP_CONTENT_ENCODING": "gzip",
		"HTTP_GIT_PROTOCOL":     "version=2",
		"GIT_CONFIG_COUNT":      "5",
		"GIT_CONFIG_KEY_0":      "http.receivepack",
		"GIT_CONFIG_VALUE_0":    "true",
		"GIT_CONFIG_KEY_1":      "receive.hideRefs",
		"GIT_CONFIG_VALUE_1":    "refs/",
		"GIT_CONFIG_KEY_2":      "receive.hideRefs",
		"GIT_CONFIG_VALUE_2":    "!refs/heads/discobot/session-123",
		"GIT_CONFIG_KEY_3":      "receive.denyDeletes",
		"GIT_CONFIG_VALUE_3":    "true",
		"GIT_CONFIG_KEY_4":      "receive.denyNonFastForwards",
		"GIT_CONFIG_VALUE_4":    "true",
	}
	for key, wantValue := range want {
		if got := values[key]; got != wantValue {
			t.Fatalf("%s = %q, want %q", key, got, wantValue)
		}
	}
}

func TestGitConfigEnvExposesOnlySessionRef(t *testing.T) {
	env := gitHTTPBackendEnv("/tmp/workspace/repo", "session-123", gitHTTPRequest{})
	cmd := exec.Command("git", "config", "--get-all", "receive.hideRefs")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config --get-all receive.hideRefs: %v", err)
	}
	got := strings.Fields(string(out))
	want := []string{"refs/", "!refs/heads/discobot/session-123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("receive.hideRefs = %q, want %q", got, want)
	}
}

func TestParseCGIResponse(t *testing.T) {
	status, headers, body, err := parseCGIResponse([]byte("Status: 403 Forbidden\r\nContent-Type: text/plain\r\nX-Test: a\r\nX-Test: b\r\n\r\ndenied"))
	if err != nil {
		t.Fatal(err)
	}
	if status != 403 {
		t.Fatalf("status = %d, want 403", status)
	}
	if got := headers["Content-Type"]; !reflect.DeepEqual(got, []string{"text/plain"}) {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := headers["X-Test"]; !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("X-Test = %q", got)
	}
	if !bytes.Equal(body, []byte("denied")) {
		t.Fatalf("body = %q", body)
	}
}

func TestHandleControlSocketFramesEndToEndGitHTTP(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspacePath := initGitRepo(t, filepath.Join(t.TempDir(), "workspace"))
	if err := os.WriteFile(filepath.Join(workspacePath, "large.bin"), bytes.Repeat([]byte("large response data\n"), 256*1024), 0o600); err != nil {
		t.Fatal(err)
	}
	runControlSocketGit(t, workspacePath, "add", "large.bin")
	runControlSocketGit(t, workspacePath, "commit", "-m", "large response")
	sessionID := "session-e2e"
	projectID := "project-e2e"

	st := setupTestStoreForIdleMonitor(t)
	createControlSocketGitFixtures(ctx, t, st, projectID, sessionID, workspacePath)

	svc := NewSandboxControlSocketService(st, nil)
	controlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("control socket accept: %v", err)
			return
		}
		_ = svc.HandleControlSocketFrames(r.Context(), controlsocket.NewConn(ws), projectID, sessionID)
	}))
	defer controlServer.Close()

	client := newTestControlSocketClient(ctx, t, wsURL(controlServer.URL))
	defer client.Close()

	tunnel := newTestGitTunnel(t, client)
	remoteURL := tunnel.Start(ctx)

	out := runControlSocketGit(t, "", "ls-remote", remoteURL, "HEAD")
	if !strings.Contains(out, "HEAD") {
		t.Fatalf("ls-remote output %q does not contain HEAD", out)
	}

	clonePath := filepath.Join(t.TempDir(), "clone")
	runControlSocketGit(t, "", "clone", remoteURL, clonePath)
	if _, err := os.Stat(filepath.Join(clonePath, "README.md")); err != nil {
		t.Fatalf("clone README.md: %v", err)
	}

	clientRepo := initGitRepo(t, filepath.Join(t.TempDir(), "client"))
	if err := os.WriteFile(filepath.Join(clientRepo, "client.txt"), []byte("from sandbox\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runControlSocketGit(t, clientRepo, "add", "client.txt")
	runControlSocketGit(t, clientRepo, "commit", "-m", "client change")
	clientHead := strings.TrimSpace(runControlSocketGit(t, clientRepo, "rev-parse", "HEAD"))

	runControlSocketGit(t, clientRepo, "push", remoteURL, "HEAD:"+sessionPushRef(sessionID))
	serverRef := strings.TrimSpace(runControlSocketGit(t, workspacePath, "rev-parse", sessionPushRef(sessionID)))
	if serverRef != clientHead {
		t.Fatalf("server ref = %q, want client HEAD %q", serverRef, clientHead)
	}

	cmd := gitCommand(ctx, clientRepo, "push", remoteURL, "HEAD:refs/heads/not-"+sessionID)
	outBytes, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("disallowed push unexpectedly succeeded: %s", outBytes)
	}
	verifyCmd := gitCommand(ctx, workspacePath, "rev-parse", "--verify", "--quiet", "refs/heads/not-"+sessionID)
	if outBytes, err := verifyCmd.CombinedOutput(); err == nil {
		got := strings.TrimSpace(string(outBytes))
		t.Fatalf("disallowed push created ref %q", got)
	}
}

func TestHandleControlSocketFramesAllowsManagedWorkspace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspacePath := initGitRepo(t, filepath.Join(t.TempDir(), "managed-workspace"))
	sessionID := "session-managed"
	projectID := "project-managed"

	st := setupTestStoreForIdleMonitor(t)
	createControlSocketGitFixturesWithSource(ctx, t, st, projectID, sessionID, workspacePath, model.WorkspaceSourceTypeManaged)

	svc := NewSandboxControlSocketService(st, nil)
	controlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("control socket accept: %v", err)
			return
		}
		_ = svc.HandleControlSocketFrames(r.Context(), controlsocket.NewConn(ws), projectID, sessionID)
	}))
	defer controlServer.Close()

	client := newTestControlSocketClient(ctx, t, wsURL(controlServer.URL))
	defer client.Close()

	tunnel := newTestGitTunnel(t, client)
	remoteURL := tunnel.Start(ctx)

	out := runControlSocketGit(t, "", "ls-remote", remoteURL, "HEAD")
	if !strings.Contains(out, "HEAD") {
		t.Fatalf("ls-remote output %q does not contain HEAD", out)
	}
}

func TestHandleControlSocketFramesRejectsRemoteGitWorkspace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workspacePath := initGitRepo(t, filepath.Join(t.TempDir(), "workspace"))
	sessionID := "session-remote"
	projectID := "project-remote"

	st := setupTestStoreForIdleMonitor(t)
	createControlSocketGitFixturesWithSource(ctx, t, st, projectID, sessionID, workspacePath, model.WorkspaceSourceTypeGit)

	svc := NewSandboxControlSocketService(st, nil)
	controlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("control socket accept: %v", err)
			return
		}
		_ = svc.HandleControlSocketFrames(r.Context(), controlsocket.NewConn(ws), projectID, sessionID)
	}))
	defer controlServer.Close()

	client := newTestControlSocketClient(ctx, t, wsURL(controlServer.URL))
	defer client.Close()

	tunnel := newTestGitTunnel(t, client)
	remoteURL := tunnel.Start(ctx)

	cmd := gitCommand(ctx, "", "ls-remote", remoteURL, "HEAD")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("remote git workspace unexpectedly served over control socket: %s", out)
	}
	if !strings.Contains(string(out), "not valid") && !strings.Contains(string(out), "502") {
		t.Fatalf("expected rejected git request output, got: %s", out)
	}
}

func envValues(env []string) map[string]string {
	values := map[string]string{}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func createControlSocketGitFixtures(ctx context.Context, t *testing.T, st interface {
	CreateProject(context.Context, *model.Project) error
	CreateWorkspace(context.Context, *model.Workspace) error
	CreateSession(context.Context, *model.Session) error
}, projectID, sessionID, workspacePath string) {
	t.Helper()
	createControlSocketGitFixturesWithSource(ctx, t, st, projectID, sessionID, workspacePath, model.WorkspaceSourceTypeLocal)
}

func createControlSocketGitFixturesWithSource(ctx context.Context, t *testing.T, st interface {
	CreateProject(context.Context, *model.Project) error
	CreateWorkspace(context.Context, *model.Workspace) error
	CreateSession(context.Context, *model.Session) error
}, projectID, sessionID, workspacePath, sourceType string) {
	t.Helper()

	workspaceID := "workspace-e2e"
	if err := st.CreateProject(ctx, &model.Project{ID: projectID, Name: "E2E"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWorkspace(ctx, &model.Workspace{
		ID:         workspaceID,
		ProjectID:  projectID,
		Path:       workspacePath,
		SourceType: sourceType,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, &model.Session{
		ID:            sessionID,
		ProjectID:     projectID,
		WorkspaceID:   workspaceID,
		WorkspacePath: &workspacePath,
		Name:          sessionID,
		Status:        model.SessionStatusReady,
	}); err != nil {
		t.Fatal(err)
	}
}

func initGitRepo(t *testing.T, dir string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	runControlSocketGit(t, dir, "init")
	runControlSocketGit(t, dir, "config", "user.email", "discobot@example.com")
	runControlSocketGit(t, dir, "config", "user.name", "Discobot Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runControlSocketGit(t, dir, "add", "README.md")
	runControlSocketGit(t, dir, "commit", "-m", "initial")
	return dir
}

func runControlSocketGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := gitCommand(t.Context(), dir, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

type testControlSocketClient struct {
	conn *controlsocket.Conn

	mu      sync.Mutex
	streams map[string]chan controlsocket.Frame
}

func newTestControlSocketClient(ctx context.Context, t *testing.T, url string) *testControlSocketClient {
	t.Helper()

	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &testControlSocketClient{
		conn:    controlsocket.NewConn(ws),
		streams: map[string]chan controlsocket.Frame{},
	}
	go client.readLoop(ctx)
	return client
}

func (c *testControlSocketClient) Close() {
	_ = c.conn.Close()
}

func (c *testControlSocketClient) OpenStream(ctx context.Context, channel string, payload any) (*testControlStream, error) {
	frames := make(chan controlsocket.Frame, 32)
	c.mu.Lock()
	if _, ok := c.streams[channel]; ok {
		c.mu.Unlock()
		return nil, fmt.Errorf("stream %q is already open", channel)
	}
	c.streams[channel] = frames
	c.mu.Unlock()

	if err := c.conn.WriteFrame(ctx, controlsocket.Frame{Channel: channel, Type: controlsocket.TypeStreamOpen, Payload: controlsocket.Payload(payload)}); err != nil {
		c.removeStream(channel)
		return nil, err
	}
	return &testControlStream{client: c, channel: channel, frames: frames}, nil
}

func (c *testControlSocketClient) removeStream(channel string) {
	c.mu.Lock()
	delete(c.streams, channel)
	c.mu.Unlock()
}

func (c *testControlSocketClient) readLoop(ctx context.Context) {
	for {
		frame, err := c.conn.ReadFrame(ctx)
		if err != nil {
			return
		}
		c.mu.Lock()
		frames := c.streams[frame.Channel]
		if frame.Type == controlsocket.TypeStreamClose {
			delete(c.streams, frame.Channel)
		}
		c.mu.Unlock()
		if frames == nil {
			continue
		}
		select {
		case frames <- frame:
		default:
		}
	}
}

type testControlStream struct {
	client  *testControlSocketClient
	channel string
	frames  <-chan controlsocket.Frame
}

func (s *testControlStream) Write(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	return s.client.conn.WriteFrame(context.Background(), controlsocket.Frame{
		Channel: s.channel,
		Type:    controlsocket.TypeStreamData,
		Data:    append([]byte(nil), p...),
	})
}

func (s *testControlStream) CloseWrite() error {
	return s.client.conn.WriteFrame(context.Background(), controlsocket.Frame{Channel: s.channel, Type: controlsocket.TypeStreamCloseWrite})
}

func (s *testControlStream) Close() {
	s.client.removeStream(s.channel)
	_ = s.client.conn.WriteFrame(context.Background(), controlsocket.Frame{Channel: s.channel, Type: controlsocket.TypeStreamClose})
}

type testGitTunnel struct {
	t      *testing.T
	client *testControlSocketClient

	mu  sync.Mutex
	seq uint64
}

func newTestGitTunnel(t *testing.T, client *testControlSocketClient) *testGitTunnel {
	t.Helper()
	return &testGitTunnel{t: t, client: client}
}

func (g *testGitTunnel) Start(ctx context.Context) string {
	g.t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		g.t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(g.handleHTTP)}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			g.t.Logf("git tunnel test server: %v", err)
		}
	}()
	return "http://" + listener.Addr().String() + "/workspace.git"
}

func (g *testGitTunnel) handleHTTP(w http.ResponseWriter, r *http.Request) {
	channel := "git:" + strconv.FormatUint(g.nextSeq(), 10)
	stream, err := g.client.OpenStream(r.Context(), channel, gitHTTPRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.RawQuery,
		Headers: testGitHeaders(r),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer stream.Close()

	bodyErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 64*1024)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				if writeErr := stream.Write(buf[:n]); writeErr != nil {
					bodyErr <- writeErr
					return
				}
			}
			if err == io.EOF {
				bodyErr <- stream.CloseWrite()
				return
			}
			if err != nil {
				bodyErr <- err
				return
			}
		}
	}()

	started := false
	for {
		select {
		case err := <-bodyErr:
			if err != nil && !started {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
		case frame := <-stream.frames:
			switch frame.Type {
			case controlsocket.TypeStreamOpen:
				var resp gitHTTPResponse
				if err := json.Unmarshal(frame.Payload, &resp); err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				for key, values := range resp.Headers {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(resp.Status)
				started = true
			case controlsocket.TypeStreamData:
				_, _ = w.Write(frame.Data)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			case controlsocket.TypeStreamCloseWrite, controlsocket.TypeStreamClose:
				return
			case controlsocket.TypeError:
				if !started {
					http.Error(w, string(frame.Payload), http.StatusBadGateway)
				}
				return
			}
		case <-r.Context().Done():
			return
		case <-time.After(10 * time.Second):
			http.Error(w, "timed out waiting for git response", http.StatusGatewayTimeout)
			return
		}
	}
}

func (g *testGitTunnel) nextSeq() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return g.seq
}

func testGitHeaders(r *http.Request) map[string][]string {
	headers := map[string][]string{}
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Encoding", "Git-Protocol", "Accept", "User-Agent"} {
		if values := r.Header.Values(key); len(values) > 0 {
			headers[key] = values
		}
	}
	if r.ContentLength >= 0 {
		headers["Content-Length"] = []string{strconv.FormatInt(r.ContentLength, 10)}
	}
	return headers
}
