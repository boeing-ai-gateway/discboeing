package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	serverapi "github.com/boeing-ai-gateway/discboeing/server/api"
	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/mock"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
	"github.com/boeing-ai-gateway/discboeing/server/internal/store"
	"github.com/boeing-ai-gateway/discboeing/server/internal/terminal"
)

// mockPTY implements sandbox.PTY for testing terminal behavior
type mockPTY struct {
	readBuffer  *bytes.Buffer
	writeBuffer *bytes.Buffer
	readErr     error
	writeErr    error
	resizeErr   error
	exitCode    int
	waitDelay   time.Duration
	closed      bool
	mu          sync.Mutex
	readDelay   time.Duration // Simulate slow reads
	onRead      func()        // Callback for read operations
	onWrite     func()        // Callback for write operations
}

func TestTerminalWorkDir(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/terminal/ws", nil)
	if got := terminalWorkDir(req); got != "" {
		t.Fatalf("terminalWorkDir() = %q, want empty", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/terminal/ws?workdir=workspace", nil)
	if got := terminalWorkDir(req); got != terminalWorkspaceDir {
		t.Fatalf("terminalWorkDir() = %q, want %q", got, terminalWorkspaceDir)
	}
}

func TestTerminalReuseKeyIncludesWorkDir(t *testing.T) {
	homeKey := terminalReuseKey("session-1", "1000:1000", "")
	if homeKey != "session-1:1000:1000" {
		t.Fatalf("terminalReuseKey() = %q, want home key", homeKey)
	}

	workspaceKey := terminalReuseKey("session-1", "1000:1000", terminalWorkspaceDir)
	if workspaceKey == homeKey {
		t.Fatal("workspace terminal reuse key should differ from home terminal reuse key")
	}
	if !strings.Contains(workspaceKey, terminalWorkspaceDir) {
		t.Fatalf("workspace terminal reuse key %q does not include workdir", workspaceKey)
	}
}

func closeWebSocket(conn *websocket.Conn) {
	_ = conn.Close(websocket.StatusNormalClosure, "done")
}

func readTestWebSocketJSON(conn *websocket.Conn, v any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return wsjson.Read(ctx, conn, v)
}

func writeTestWebSocketJSON(conn *websocket.Conn, v any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return wsjson.Write(ctx, conn, v)
}

func newMockPTY() *mockPTY {
	return &mockPTY{
		readBuffer:  bytes.NewBuffer(nil),
		writeBuffer: bytes.NewBuffer(nil),
		exitCode:    0,
	}
}

func (m *mockPTY) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.onRead != nil {
		m.onRead()
	}

	if m.readDelay > 0 {
		time.Sleep(m.readDelay)
	}

	if m.readErr != nil {
		return 0, m.readErr
	}

	return m.readBuffer.Read(p)
}

func (m *mockPTY) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.onWrite != nil {
		m.onWrite()
	}

	if m.writeErr != nil {
		return 0, m.writeErr
	}

	return m.writeBuffer.Write(p)
}

func (m *mockPTY) Resize(_ context.Context, _, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resizeErr
}

func (m *mockPTY) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockPTY) Wait(_ context.Context) (int, error) {
	if m.waitDelay > 0 {
		time.Sleep(m.waitDelay)
	}
	return m.exitCode, nil
}

// feedOutput simulates PTY producing output
func (m *mockPTY) feedOutput(data string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuffer.WriteString(data)
}

// setReadError makes the next Read call return an error
func (m *mockPTY) setReadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readErr = err
}

// setWriteError makes Write calls return an error
func (m *mockPTY) setWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

// getWrittenData returns what was written to the PTY
func (m *mockPTY) getWrittenData() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeBuffer.String()
}

type blockingPTY struct {
	closeCh chan struct{}
}

func newBlockingPTY() *blockingPTY {
	return &blockingPTY{closeCh: make(chan struct{})}
}

func (p *blockingPTY) Read(_ []byte) (int, error) {
	<-p.closeCh
	return 0, io.EOF
}

func (p *blockingPTY) Write(data []byte) (int, error) {
	return len(data), nil
}

func (p *blockingPTY) Resize(_ context.Context, _, _ int) error {
	return nil
}

func (p *blockingPTY) Close() error {
	select {
	case <-p.closeCh:
	default:
		close(p.closeCh)
	}
	return nil
}

func (p *blockingPTY) Wait(_ context.Context) (int, error) {
	<-p.closeCh
	return 0, nil
}

type outputBlockingPTY struct {
	output  *bytes.Buffer
	writes  *bytes.Buffer
	closeCh chan struct{}
	mu      sync.Mutex
}

func newOutputBlockingPTY(output string) *outputBlockingPTY {
	return &outputBlockingPTY{
		output:  bytes.NewBufferString(output),
		writes:  bytes.NewBuffer(nil),
		closeCh: make(chan struct{}),
	}
}

func (p *outputBlockingPTY) Read(data []byte) (int, error) {
	p.mu.Lock()
	if p.output.Len() > 0 {
		n, err := p.output.Read(data)
		p.mu.Unlock()
		return n, err
	}
	p.mu.Unlock()

	<-p.closeCh
	return 0, io.EOF
}

func (p *outputBlockingPTY) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writes.Write(data)
}

func (p *outputBlockingPTY) Resize(_ context.Context, _, _ int) error {
	return nil
}

func (p *outputBlockingPTY) Close() error {
	select {
	case <-p.closeCh:
	default:
		close(p.closeCh)
	}
	return nil
}

func (p *outputBlockingPTY) Wait(_ context.Context) (int, error) {
	<-p.closeCh
	return 0, nil
}

func (p *outputBlockingPTY) writtenData() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writes.String()
}

// TestHandleTerminalSession_NormalFlow tests that PTY output reaches the WebSocket
// client and WebSocket input is forwarded to the PTY.
//
// With persistent sessions the PTY is owned by the terminal.Manager, not the
// WebSocket handler.  The handler only subscribes/unsubscribes; PTY lifecycle
// is separate.
func TestHandleTerminalSession_NormalFlow(t *testing.T) {
	pty := newOutputBlockingPTY("hello from shell\n$ ")

	// Create a Manager and wrap the mock PTY in a persistent Session.
	mgr := terminal.NewManager()
	sess, err := mgr.GetOrCreate(context.Background(), "test-session:test-user",
		func(_ context.Context) (sandbox.PTY, error) { return pty, nil })
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	// Subscribe before the read loop may have finished so we always get output.
	sub := sess.Subscribe()

	// Create mock WebSocket pair.
	server, client := createMockWebSocketPair(t)
	defer closeWebSocket(server)
	defer closeWebSocket(client)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handlePersistentTerminalSession(ctx, sess, sub, server)
	}()

	// Read the output message.
	var msg serverapi.TerminalMessage
	if err := readTestWebSocketJSON(client, &msg); err != nil {
		t.Fatalf("read websocket JSON: %v", err)
	}
	if msg.Type != "output" {
		t.Errorf("want type=output, got %s", msg.Type)
	}
	var output string
	if err := unmarshalTerminalMessageData(msg.Data, &output); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !strings.Contains(output, "hello from shell") {
		t.Errorf("want 'hello from shell' in output, got %q", output)
	}

	// Send input.
	inputMsg := serverapi.TerminalMessage{Type: "input", Data: json.RawMessage(`"ls\n"`)}
	if err := writeTestWebSocketJSON(client, inputMsg); err != nil {
		t.Fatalf("write websocket JSON: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for pty.writtenData() != "ls\n" {
		select {
		case <-deadline:
			t.Fatalf("PTY input: want %q, got %q", "ls\n", pty.writtenData())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err := pty.Close(); err != nil {
		t.Fatalf("Close PTY: %v", err)
	}

	// Drain the client side so that coder/websocket's close handling runs when
	// the server sends the close frame.  This sends the close ack back to the
	// server, which unblocks the websocket read in the input goroutine and lets the
	// handler return cleanly.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for {
			if _, _, err := client.Read(ctx); err != nil {
				return
			}
		}
	}()

	// Wait for the handler to return (PTY exits → sub closes → close frame sent
	// → client acks → server websocket read fails or deadline fires → handler returns).
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler didn't finish in time")
	}

	// Input was verified before closing the PTY.
}

func TestHandleTerminalSession_ShutdownClosesWebSocket(t *testing.T) {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	mgr := terminal.NewManager()
	h := &Handler{
		terminalManager: mgr,
		shutdownCtx:     shutdownCtx,
		shutdownCancel:  shutdownCancel,
	}

	sess, err := mgr.GetOrCreate(context.Background(), "test-session:test-user",
		func(_ context.Context) (sandbox.PTY, error) { return newBlockingPTY(), nil })
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	sub := sess.Subscribe()
	server, client := createMockWebSocketPair(t)
	defer closeWebSocket(server)
	defer closeWebSocket(client)
	defer sess.Unsubscribe(sub)

	done := make(chan struct{})
	go func() {
		defer close(done)
		handlePersistentTerminalSession(context.Background(), sess, sub, server)
	}()

	h.BeginShutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := client.Read(ctx); err == nil {
		t.Fatal("expected websocket to close on shutdown")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("websocket handler did not exit on shutdown")
	}
}

// TestHandleTerminalSession_HalfClose_ClientStopsWriting tests that output
// continues when the client stops writing but the PTY is still producing output.
func TestHandleTerminalSession_HalfClose_ClientStopsWriting(t *testing.T) {
	t.Skip("TODO: Fix WebSocket lifecycle handling in test")

	pty := newMockPTY()
	pty.waitDelay = 200 * time.Millisecond

	readCount := 0
	pty.onRead = func() {
		readCount++
		switch readCount {
		case 1:
			pty.readBuffer.WriteString("output line 1\n")
		case 2:
			pty.readBuffer.WriteString("output line 2\n")
		case 3:
			pty.readBuffer.WriteString("output line 3\n")
		default:
			pty.setReadError(io.EOF)
		}
	}

	mgr := terminal.NewManager()
	sess, err := mgr.GetOrCreate(context.Background(), "test-session:test-user",
		func(_ context.Context) (sandbox.PTY, error) { return pty, nil })
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	sub := sess.Subscribe()

	server, client := createMockWebSocketPair(t)
	defer closeWebSocket(server)
	defer closeWebSocket(client)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handlePersistentTerminalSession(ctx, sess, sub, server)
	}()

	outputReceived := make(chan string, 10)
	go func() {
		for {
			var msg serverapi.TerminalMessage
			if err := readTestWebSocketJSON(client, &msg); err != nil {
				return
			}
			if msg.Type == "output" {
				var output string
				_ = unmarshalTerminalMessageData(msg.Data, &output)
				outputReceived <- output
			}
		}
	}()

	timeout := time.After(2 * time.Second)
	allOutput := []string{}
collectLoop:
	for {
		select {
		case output := <-outputReceived:
			allOutput = append(allOutput, output)
			if len(allOutput) >= 3 {
				break collectLoop
			}
		case <-timeout:
			t.Fatal("Timeout waiting for output")
		}
	}

	_ = client.Close(websocket.StatusNormalClosure, "test done")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Handler didn't finish after client closed")
	}

	if len(allOutput) < 3 {
		t.Errorf("Expected at least 3 output messages, got %d", len(allOutput))
	}
	fullOutput := strings.Join(allOutput, "")
	for _, want := range []string{"output line 1", "output line 2", "output line 3"} {
		if !strings.Contains(fullOutput, want) {
			t.Errorf("Missing %q in output", want)
		}
	}
}

// TestTerminalWebSocket_PTYExitsCleanly tests Ctrl-D scenario
func TestTerminalWebSocket_PTYExitsCleanly(t *testing.T) {
	t.Skip("TODO: Update to use handlePersistentTerminalSession")
	pty := newMockPTY()
	pty.feedOutput("$ exit\n")
	pty.exitCode = 0

	// After output is read, return EOF
	readOnce := false
	pty.onRead = func() {
		if readOnce {
			pty.setReadError(io.EOF)
		}
		readOnce = true
	}

	mockProvider := mock.NewProvider()

	testStore := setupTestStore(t)
	sandboxService := service.NewSandboxService(testStore, mockProvider, nil, nil, nil, nil, nil)

	handler := &Handler{
		sandboxService:  sandboxService,
		store:           testStore,
		terminalManager: terminal.NewManager(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.TerminalWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer closeWebSocket(ws)

	// Read output
	var msg serverapi.TerminalMessage
	if err := readTestWebSocketJSON(ws, &msg); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	// Wait for close message
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err = ws.Read(ctx)
	if err == nil {
		t.Error("Expected connection to close")
	}

	if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Errorf("Expected StatusNormalClosure, got: %v", err)
	}
}

// TestTerminalWebSocket_PTYWriteError tests when writing to PTY fails
func TestTerminalWebSocket_PTYWriteError(t *testing.T) {
	t.Skip("TODO: Update to use handlePersistentTerminalSession")
	pty := newMockPTY()
	pty.setWriteError(errors.New("pty write failed"))
	pty.feedOutput("initial output\n")

	readOnce := false
	pty.onRead = func() {
		if !readOnce {
			readOnce = true
			return
		}
		pty.setReadError(io.EOF)
	}

	mockProvider := mock.NewProvider()

	testStore := setupTestStore(t)
	sandboxService := service.NewSandboxService(testStore, mockProvider, nil, nil, nil, nil, nil)

	handler := &Handler{
		sandboxService:  sandboxService,
		store:           testStore,
		terminalManager: terminal.NewManager(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.TerminalWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer closeWebSocket(ws)

	// Read initial output
	var msg serverapi.TerminalMessage
	readTestWebSocketJSON(ws, &msg)

	// Try to send input (should fail to write to PTY)
	inputMsg := serverapi.TerminalMessage{
		Type: "input",
		Data: json.RawMessage(`"test input\n"`),
	}
	writeTestWebSocketJSON(ws, inputMsg)

	// Output should still continue
	for {
		if err := readTestWebSocketJSON(ws, &msg); err != nil {
			break
		}
	}
}

// TestTerminalWebSocket_PTYReadError tests non-EOF read errors
func TestTerminalWebSocket_PTYReadError(t *testing.T) {
	t.Skip("TODO: Update to use handlePersistentTerminalSession")
	pty := newMockPTY()
	pty.feedOutput("some output\n")

	readOnce := false
	pty.onRead = func() {
		if !readOnce {
			readOnce = true
			return
		}
		// Simulate a non-EOF error
		pty.setReadError(errors.New("pty read failed"))
	}

	mockProvider := mock.NewProvider()

	testStore := setupTestStore(t)
	sandboxService := service.NewSandboxService(testStore, mockProvider, nil, nil, nil, nil, nil)

	handler := &Handler{
		sandboxService:  sandboxService,
		store:           testStore,
		terminalManager: terminal.NewManager(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.TerminalWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer closeWebSocket(ws)

	// Read initial output
	var msg serverapi.TerminalMessage
	readTestWebSocketJSON(ws, &msg)

	// Connection should close due to error
	for {
		if err := readTestWebSocketJSON(ws, &msg); err != nil {
			break
		}
	}
}

// TestTerminalWebSocket_ResizeOperations tests terminal resize handling
func TestTerminalWebSocket_ResizeOperations(t *testing.T) {
	t.Skip("TODO: Update to use handlePersistentTerminalSession")
	pty := newMockPTY()
	pty.feedOutput("$ ")

	resizeReceived := make(chan bool, 1)
	pty.onRead = func() {
		// After first read, return EOF
		pty.setReadError(io.EOF)
	}

	// Track resize calls - we'll override the resize method
	pty.resizeErr = nil
	oldOnRead := pty.onRead
	pty.onRead = func() {
		// Track that resize was called (implicitly through the handler)
		if oldOnRead != nil {
			oldOnRead()
		}
	}

	mockProvider := mock.NewProvider()

	testStore := setupTestStore(t)
	sandboxService := service.NewSandboxService(testStore, mockProvider, nil, nil, nil, nil, nil)

	handler := &Handler{
		sandboxService:  sandboxService,
		store:           testStore,
		terminalManager: terminal.NewManager(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.TerminalWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer closeWebSocket(ws)

	// Read initial output
	var msg serverapi.TerminalMessage
	readTestWebSocketJSON(ws, &msg)

	// Send resize message
	resizeData, _ := json.Marshal(serverapi.ResizeData{Rows: 40, Cols: 120})
	resizeMsg := serverapi.TerminalMessage{
		Type: "resize",
		Data: json.RawMessage(resizeData),
	}
	if err := writeTestWebSocketJSON(ws, resizeMsg); err != nil {
		t.Fatalf("Failed to send resize: %v", err)
	}

	// Wait for resize to be processed
	select {
	case <-resizeReceived:
		// Success!
	case <-time.After(2 * time.Second):
		t.Error("Resize was not processed")
	}

	closeWebSocket(ws)
}

// TestTerminalWebSocket_OutputDraining tests that all output is sent before closing
func TestTerminalWebSocket_OutputDraining(t *testing.T) {
	t.Skip("TODO: Update to use handlePersistentTerminalSession")
	pty := newMockPTY()

	outputChunks := []string{
		"chunk 1\n",
		"chunk 2\n",
		"chunk 3\n",
		"chunk 4\n",
		"chunk 5\n",
	}

	chunkIndex := 0
	pty.onRead = func() {
		if chunkIndex < len(outputChunks) {
			pty.readBuffer.WriteString(outputChunks[chunkIndex])
			chunkIndex++
		} else {
			pty.setReadError(io.EOF)
		}
	}

	mockProvider := mock.NewProvider()

	testStore := setupTestStore(t)
	sandboxService := service.NewSandboxService(testStore, mockProvider, nil, nil, nil, nil, nil)

	handler := &Handler{
		sandboxService:  sandboxService,
		store:           testStore,
		terminalManager: terminal.NewManager(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.TerminalWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer closeWebSocket(ws)

	receivedChunks := []string{}

	for {
		var msg serverapi.TerminalMessage
		if err := readTestWebSocketJSON(ws, &msg); err != nil {
			break
		}

		if msg.Type == "output" {
			var output string
			_ = unmarshalTerminalMessageData(msg.Data, &output)
			receivedChunks = append(receivedChunks, output)
		}
	}

	if len(receivedChunks) != len(outputChunks) {
		t.Errorf("Expected %d chunks, got %d", len(outputChunks), len(receivedChunks))
	}

	fullOutput := strings.Join(receivedChunks, "")
	for i, expected := range outputChunks {
		if !strings.Contains(fullOutput, expected) {
			t.Errorf("Missing chunk %d: %q", i, expected)
		}
	}
}

// TestTerminalWebSocket_ConcurrentInputOutput tests concurrent operations
func TestTerminalWebSocket_ConcurrentInputOutput(t *testing.T) {
	t.Skip("TODO: Update to use handlePersistentTerminalSession")
	pty := newMockPTY()

	go func() {
		for i := range 20 {
			pty.feedOutput(fmt.Sprintf("output %d\n", i))
			time.Sleep(10 * time.Millisecond)
		}
		pty.setReadError(io.EOF)
	}()

	mockProvider := mock.NewProvider()

	testStore := setupTestStore(t)
	sandboxService := service.NewSandboxService(testStore, mockProvider, nil, nil, nil, nil, nil)

	handler := &Handler{
		sandboxService:  sandboxService,
		store:           testStore,
		terminalManager: terminal.NewManager(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.TerminalWebSocket(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer closeWebSocket(ws)

	var wg sync.WaitGroup

	wg.Go(func() {
		for i := range 10 {
			inputMsg := serverapi.TerminalMessage{
				Type: "input",
				Data: json.RawMessage(fmt.Sprintf(`"input %d\n"`, i)),
			}
			writeTestWebSocketJSON(ws, inputMsg)
			time.Sleep(15 * time.Millisecond)
		}
	})

	outputCount := 0
	wg.Go(func() {
		for {
			var msg serverapi.TerminalMessage
			if err := readTestWebSocketJSON(ws, &msg); err != nil {
				break
			}
			if msg.Type == "output" {
				outputCount++
			}
		}
	})

	wg.Wait()

	if outputCount == 0 {
		t.Error("Expected to receive output messages")
	}

	written := pty.getWrittenData()
	if !strings.Contains(written, "input 0") {
		t.Error("Expected to receive input")
	}
}

// createMockWebSocketPair creates a pair of connected WebSocket connections for testing.
// Returns (server-side conn, client-side conn).
func createMockWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()

	serverConn := make(chan *websocket.Conn, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			t.Fatalf("Failed to upgrade: %v", err)
		}
		serverConn <- conn
	}))
	t.Cleanup(func() { server.Close() })

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	serverSide := <-serverConn
	return serverSide, client
}

// setupTestStore creates an in-memory SQLite database for testing
func setupTestStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return store.New(db, nil)
}
