package browser

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/obot-platform/discobot/agent-go/internal/files"
	"github.com/obot-platform/discobot/agent-go/thread"
)

const devToolsStartupTimeout = 10 * time.Second
const cdpReadLimit = 16 << 20

// ErrChromiumNotFound indicates that no supported Chromium executable is on PATH.
var ErrChromiumNotFound = errors.New("chromium executable not found")

// ErrStoreUnavailable indicates browser persistence is not configured.
var ErrStoreUnavailable = errors.New("browser store unavailable")

// Info describes the session-scoped browser runtime exposed by agent-go.
type Info struct {
	SessionID     string `json:"sessionId"`
	Running       bool   `json:"running"`
	WebSocketPath string `json:"webSocketPath"`
	WebSocketURL  string `json:"webSocketUrl"`
	Token         string `json:"token"`
	UserDataDir   string `json:"userDataDir"`
	LastError     string `json:"lastError,omitempty"`
}

// Manager owns the session-scoped Chromium runtime and loopback CDP endpoint.
type Manager struct {
	sessionID string
	port      int
	stateDir  string

	mu            sync.Mutex
	token         string
	userDataDir   string
	chromiumPath  string
	cmd           *exec.Cmd
	processDone   chan struct{}
	upstreamWSURL string
	lastError     string
	store         *Store
	currentTurn   func(threadID string) (*thread.TurnState, error)
}

const (
	EnvCDPURL          = "DISCOBOT_BROWSER_CDP_URL"
	EnvHarnessCDPURL   = "BROWSER_HARNESS_CDP_URL"
	EnvBrowserUseCDPWS = "BU_CDP_WS"
	EnvBrowserThreadID = "DISCOBOT_BROWSER_THREAD_ID"
)

func NewManager(sessionID string, dataDir string, port int) (*Manager, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session ID is required")
	}
	token, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("generate browser token: %w", err)
	}
	stateDir := filepath.Join(dataDir, "browser", sessionID)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create browser state dir: %w", err)
	}
	return &Manager{
		sessionID:   sessionID,
		port:        port,
		stateDir:    stateDir,
		token:       token,
		userDataDir: filepath.Join(stateDir, "profile"),
	}, nil
}

func (m *Manager) SessionID() string {
	return m.sessionID
}

// SetStore configures thread-local browser event and artifact storage.
func (m *Manager) SetStore(store *Store) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
}

// SetCurrentTurnLoader configures active turn lookup for browser event attribution.
func (m *Manager) SetCurrentTurnLoader(loader func(threadID string) (*thread.TurnState, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentTurn = loader
}

func (m *Manager) browserStore() *Store {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store
}

func (m *Manager) currentTurnLoader() func(threadID string) (*thread.TurnState, error) {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentTurn
}

func (m *Manager) WebSocketPath() string {
	return fmt.Sprintf("/sessions/%s/browser/cdp", url.PathEscape(m.sessionID))
}

func (m *Manager) LoopbackWebSocketURL() string {
	return fmt.Sprintf("ws://127.0.0.1:%d%s?token=%s", m.port, m.WebSocketPath(), url.QueryEscape(m.token))
}

func (m *Manager) Env() map[string]string {
	return map[string]string{
		EnvCDPURL:          m.LoopbackWebSocketURL(),
		EnvHarnessCDPURL:   m.LoopbackWebSocketURL(),
		EnvBrowserUseCDPWS: m.LoopbackWebSocketURL(),
	}
}

func (m *Manager) EnvForThread(threadID string) map[string]string {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return m.Env()
	}
	threadURL := appendThreadQuery(m.LoopbackWebSocketURL(), threadID)
	return map[string]string{
		EnvCDPURL:          threadURL,
		EnvHarnessCDPURL:   threadURL,
		EnvBrowserUseCDPWS: threadURL,
		EnvBrowserThreadID: threadID,
	}
}

func (m *Manager) Info() Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Info{
		SessionID:     m.sessionID,
		Running:       m.cmd != nil && m.cmd.Process != nil && m.cmd.ProcessState == nil,
		WebSocketPath: m.WebSocketPath(),
		WebSocketURL:  m.LoopbackWebSocketURL(),
		Token:         m.token,
		UserDataDir:   m.userDataDir,
		LastError:     m.lastError,
	}
}

func (m *Manager) TokenMatches(token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return token != "" && token == m.token
}

func (m *Manager) UpstreamWebSocketURL() (string, error) {
	m.mu.Lock()
	if m.cmd != nil && m.cmd.Process != nil && m.cmd.ProcessState == nil && m.upstreamWSURL != "" {
		wsURL := m.upstreamWSURL
		m.mu.Unlock()
		return wsURL, nil
	}
	m.mu.Unlock()

	log.Printf("browser[%s]: resolving upstream websocket URL", m.sessionID)
	if err := m.ensureRunning(); err != nil {
		log.Printf("browser[%s]: resolve upstream websocket URL failed: %v", m.sessionID, err)
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.upstreamWSURL == "" {
		return "", fmt.Errorf("browser websocket URL is unavailable")
	}
	return m.upstreamWSURL, nil
}

// CaptureScreenshot captures a PNG screenshot from the active page target.
func (m *Manager) CaptureScreenshot(ctx context.Context, _ string) ([]byte, error) {
	wsURL, err := m.UpstreamWebSocketURL()
	if err != nil {
		return nil, err
	}
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: &httpClientNoProxy,
	})
	if err != nil {
		return nil, fmt.Errorf("dial browser websocket: %w", err)
	}
	ConfigureCDPConn(conn)
	defer conn.Close(websocket.StatusNormalClosure, "done")

	client := &cdpClient{conn: conn}
	targetID, err := client.pageTargetID(ctx, "")
	if err != nil {
		return nil, err
	}
	var attach struct {
		SessionID string `json:"sessionId"`
	}
	if err := client.call(ctx, "Target.attachToTarget", map[string]any{
		"targetId": targetID,
		"flatten":  true,
	}, "", &attach); err != nil {
		return nil, fmt.Errorf("attach to browser target: %w", err)
	}
	if attach.SessionID == "" {
		return nil, fmt.Errorf("attach to browser target: missing session ID")
	}
	defer func() {
		_ = client.call(context.Background(), "Target.detachFromTarget", map[string]any{
			"sessionId": attach.SessionID,
		}, "", nil)
	}()

	// Enable Page domain first so screenshot capture works consistently even when
	// the page target was newly created.
	if err := client.call(ctx, "Page.enable", nil, attach.SessionID, nil); err != nil {
		return nil, fmt.Errorf("enable page domain: %w", err)
	}
	var screenshot struct {
		Data string `json:"data"`
	}
	if err := client.call(ctx, "Page.captureScreenshot", map[string]any{
		"format": "png",
	}, attach.SessionID, &screenshot); err != nil {
		return nil, fmt.Errorf("capture screenshot: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(screenshot.Data)
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return nil, fmt.Errorf("capture screenshot: invalid PNG data")
	}
	uniform, err := isUniformColorPNG(data)
	if err != nil {
		return nil, fmt.Errorf("inspect screenshot pixels: %w", err)
	}
	if uniform {
		return nil, nil
	}
	return data, nil
}

// ReadThreadArtifact reads a browser artifact from a thread-local artifact URI
// path.
func (m *Manager) ReadThreadArtifact(threadID, artifactPath string) (*files.ReadResult, *files.Error) {
	store := m.browserStore()
	if store == nil {
		return nil, &files.Error{Message: ErrStoreUnavailable.Error(), Status: http.StatusServiceUnavailable}
	}
	return store.readThreadArtifact(threadID, artifactPath)
}

// CurrentTurn returns the active turn state used to attribute browser events.
func (m *Manager) CurrentTurn(threadID string) (*thread.TurnState, error) {
	loader := m.currentTurnLoader()
	if loader == nil {
		return nil, ErrStoreUnavailable
	}
	return loader(threadID)
}

// AppendEvent persists a browser event under a thread turn step.
func (m *Manager) AppendEvent(threadID, turnID string, stepIndex int, event thread.BrowserEvent) error {
	store := m.browserStore()
	if store == nil {
		return ErrStoreUnavailable
	}
	return store.appendBrowserEvent(threadID, turnID, stepIndex, event)
}

// SaveScreenshot saves a browser screenshot artifact and returns its event file
// reference.
func (m *Manager) SaveScreenshot(threadID, turnID string, stepIndex int, eventID string, png []byte) (thread.BrowserEventFile, error) {
	store := m.browserStore()
	if store == nil {
		return thread.BrowserEventFile{}, ErrStoreUnavailable
	}
	return store.saveBrowserScreenshot(threadID, turnID, stepIndex, eventID, png)
}

// EventEntries loads all persisted browser event entries for a thread.
func (m *Manager) EventEntries(threadID string) ([]thread.BrowserEventEntry, error) {
	store := m.browserStore()
	if store == nil {
		return nil, ErrStoreUnavailable
	}
	return store.loadAllBrowserEventEntries(threadID)
}

func isUniformColorPNG(data []byte) (bool, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	bounds := img.Bounds()
	if bounds.Empty() {
		return true, nil
	}
	firstR, firstG, firstB, firstA := img.At(bounds.Min.X, bounds.Min.Y).RGBA()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if r != firstR || g != firstG || b != firstB || a != firstA {
				return false, nil
			}
		}
	}
	return true, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	cmd := m.cmd
	done := m.processDone
	m.cmd = nil
	m.processDone = nil
	m.upstreamWSURL = ""
	m.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	killErr := killBrowserCommand(cmd)
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return fmt.Errorf("wait for chromium shutdown: timed out")
		}
	}
	if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return killErr
	}
	return nil
}

func (m *Manager) ensureRunning() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil && m.cmd.ProcessState == nil && m.upstreamWSURL != "" {
		return nil
	}

	chromiumPath, err := resolveChromiumPath()
	if err != nil {
		m.lastError = err.Error()
		return err
	}
	m.chromiumPath = chromiumPath

	if err := os.MkdirAll(m.userDataDir, 0o755); err != nil {
		m.lastError = err.Error()
		return fmt.Errorf("create browser profile dir: %w", err)
	}
	if err := prepareUserDataDirForLaunch(m.userDataDir); err != nil {
		m.lastError = err.Error()
		log.Printf("browser[%s]: prepare browser profile dir failed: %v", m.sessionID, err)
		return err
	}

	logPath := filepath.Join(m.stateDir, "chromium.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		m.lastError = err.Error()
		return fmt.Errorf("open browser log: %w", err)
	}

	args := chromiumArgs(m.userDataDir)
	launchMode := "display"
	if slices.Contains(args, "--headless=new") {
		launchMode = "headless"
	}
	log.Printf("browser[%s]: launching chromium mode=%s display=%q userDataDir=%s", m.sessionID, launchMode, os.Getenv("DISPLAY"), m.userDataDir)
	cmd := exec.Command(chromiumPath, args...) //nolint:gosec
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	configureBrowserCommand(cmd)
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		m.lastError = err.Error()
		log.Printf("browser[%s]: chromium start failed: %v", m.sessionID, err)
		return fmt.Errorf("start chromium: %w", err)
	}

	upstreamWSURL, err := waitForDevToolsURL(filepath.Join(m.userDataDir, "DevToolsActivePort"))
	if err != nil {
		_ = killBrowserCommand(cmd)
		_ = logFile.Close()
		m.lastError = err.Error()
		log.Printf("browser[%s]: waiting for DevToolsActivePort failed: %v", m.sessionID, err)
		return err
	}

	m.cmd = cmd
	m.processDone = make(chan struct{})
	processDone := m.processDone
	m.upstreamWSURL = upstreamWSURL
	m.lastError = ""
	log.Printf("browser[%s]: chromium ready upstream=%s pid=%d", m.sessionID, upstreamWSURL, cmd.Process.Pid)

	go func() {
		defer close(processDone)
		err := cmd.Wait()
		_ = logFile.Close()
		m.mu.Lock()
		if m.cmd == cmd {
			m.cmd = nil
			m.processDone = nil
			m.upstreamWSURL = ""
		}
		m.mu.Unlock()
		if err != nil {
			log.Printf("browser[%s]: chromium exited with error: %v", m.sessionID, err)
			return
		}
		log.Printf("browser[%s]: chromium exited cleanly", m.sessionID)
	}()

	return nil
}

func chromiumArgs(userDataDir string) []string {
	args := []string{
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--no-first-run",
		"--no-default-browser-check",
		"--remote-debugging-address=127.0.0.1",
		"--remote-debugging-port=0",
		"--user-data-dir=" + userDataDir,
	}
	if strings.TrimSpace(os.Getenv("DISPLAY")) == "" {
		args = append(args, "--headless=new")
	} else {
		args = append(args, "--start-maximized")
	}
	return append(args, "about:blank")
}

func prepareUserDataDirForLaunch(userDataDir string) error {
	devToolsPath := filepath.Join(userDataDir, "DevToolsActivePort")
	if err := os.Remove(devToolsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale DevToolsActivePort: %w", err)
	}
	return nil
}

func resolveChromiumPath() (string, error) {
	for _, candidate := range chromiumExecutableCandidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: install one of %s or add it to PATH (PATH=%q)",
		ErrChromiumNotFound, strings.Join(chromiumExecutableCandidates, ", "), os.Getenv("PATH"))
}

// IsChromiumNotFound reports whether err was caused by a missing Chromium binary.
func IsChromiumNotFound(err error) bool {
	return errors.Is(err, ErrChromiumNotFound)
}

var chromiumExecutableCandidates = []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"}

func waitForDevToolsURL(path string) (string, error) {
	deadline := time.Now().Add(devToolsStartupTimeout)
	for {
		wsURL, err := readDevToolsURL(path)
		if err == nil {
			return wsURL, nil
		}
		if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "incomplete") {
			return "", err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("wait for chromium DevToolsActivePort: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func readDevToolsURL(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := make([]string, 0, 2)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
		if len(lines) == 2 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if len(lines) < 2 || lines[0] == "" || lines[1] == "" {
		return "", fmt.Errorf("incomplete DevToolsActivePort file")
	}
	return fmt.Sprintf("ws://127.0.0.1:%s%s", lines[0], lines[1]), nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func appendThreadQuery(rawURL, threadID string) string {
	rawURL = strings.TrimSpace(rawURL)
	threadID = strings.TrimSpace(threadID)
	if rawURL == "" || threadID == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set("threadId", threadID)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

var httpClientNoProxy = http.Client{
	Transport: &http.Transport{Proxy: nil},
}

func ConfigureCDPConn(conn *websocket.Conn) {
	if conn != nil {
		conn.SetReadLimit(cdpReadLimit)
	}
}

type cdpClient struct {
	conn   *websocket.Conn
	nextID int64
}

type cdpCommandError struct {
	Message string `json:"message"`
}

type cdpResponseEnvelope struct {
	ID      int64            `json:"id,omitempty"`
	Session string           `json:"sessionId,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *cdpCommandError `json:"error,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type cdpTargetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

func (c *cdpClient) call(ctx context.Context, method string, params any, sessionID string, result any) error {
	c.nextID++
	req := map[string]any{
		"id":     c.nextID,
		"method": method,
	}
	if params != nil {
		req["params"] = params
	}
	if sessionID != "" {
		req["sessionId"] = sessionID
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return err
	}
	for {
		_, payload, err := c.conn.Read(ctx)
		if err != nil {
			return err
		}
		var resp cdpResponseEnvelope
		if err := json.Unmarshal(payload, &resp); err != nil {
			continue
		}
		if resp.ID != c.nextID {
			continue
		}
		if resp.Error != nil {
			return fmt.Errorf("%s", strings.TrimSpace(resp.Error.Message))
		}
		if result == nil || len(resp.Result) == 0 {
			return nil
		}
		return json.Unmarshal(resp.Result, result)
	}
}

func (c *cdpClient) pageTargetID(ctx context.Context, preferredTargetID string) (string, error) {
	var targets struct {
		TargetInfos []cdpTargetInfo `json:"targetInfos"`
	}
	if err := c.call(ctx, "Target.getTargets", nil, "", &targets); err != nil {
		return "", fmt.Errorf("list browser targets: %w", err)
	}
	preferredTargetID = strings.TrimSpace(preferredTargetID)
	if preferredTargetID != "" {
		for _, target := range targets.TargetInfos {
			if target.TargetID == preferredTargetID && target.Type == "page" {
				return target.TargetID, nil
			}
		}
	}
	for _, target := range targets.TargetInfos {
		if target.Type == "page" && strings.TrimSpace(target.URL) != "" {
			return target.TargetID, nil
		}
	}
	for _, target := range targets.TargetInfos {
		if target.Type == "page" {
			return target.TargetID, nil
		}
	}
	var created struct {
		TargetID string `json:"targetId"`
	}
	if err := c.call(ctx, "Target.createTarget", map[string]any{"url": "about:blank"}, "", &created); err != nil {
		return "", fmt.Errorf("create browser target: %w", err)
	}
	if created.TargetID == "" {
		return "", fmt.Errorf("create browser target: missing target ID")
	}
	return created.TargetID, nil
}
