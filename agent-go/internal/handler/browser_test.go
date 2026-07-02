package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/agentimpl"
	"github.com/boeing-ai-gateway/discboeing/agent-go/browser"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/middleware"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func TestGetBrowserSession(t *testing.T) {
	t.Parallel()

	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	h := New("", agent.NewConversationManager(&streamTestAgent{}), nil, nil, nil, browserMgr)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/sessions/session-1/browser")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.BrowserSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SessionID != "session-1" {
		t.Fatalf("expected session-1, got %q", body.SessionID)
	}
	if body.WebSocketURL == "" || body.Token == "" {
		t.Fatalf("expected websocket URL and token, got %#v", body)
	}
}

func TestGetBrowserSession_WrongSession(t *testing.T) {
	t.Parallel()

	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	h := New("", agent.NewConversationManager(&streamTestAgent{}), nil, nil, nil, browserMgr)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/sessions/other/browser")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestBrowserCDPRouteSkipsBearerAuthButBrowserInfoDoesNot(t *testing.T) {
	t.Parallel()

	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	h := New("", agent.NewConversationManager(&streamTestAgent{}), nil, nil, nil, browserMgr)

	r := chi.NewRouter()
	r.Get("/sessions/{sessionId}/browser/cdp", h.ProxyBrowserCDP)
	authed := chi.NewRouter()
	authed.Use(middleware.Auth(testSecretHash("secret-token"), ""))
	h.RegisterRoutes(authed)
	r.Mount("/", authed)
	server := httptest.NewServer(r)
	defer server.Close()

	cdpResp, err := http.Get(server.URL + "/sessions/session-1/browser/cdp?token=wrong-token")
	if err != nil {
		t.Fatal(err)
	}
	defer cdpResp.Body.Close()
	if cdpResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected /browser/cdp to reject the browser token itself, got %d", cdpResp.StatusCode)
	}
	var cdpBody []byte
	cdpBody, err = io.ReadAll(cdpResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(cdpBody) != "invalid browser token\n" {
		t.Fatalf("expected browser-token auth failure, got %q", string(cdpBody))
	}

	browserResp, err := http.Get(server.URL + "/sessions/session-1/browser")
	if err != nil {
		t.Fatal(err)
	}
	defer browserResp.Body.Close()
	if browserResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected /browser to require bearer auth, got %d", browserResp.StatusCode)
	}
}

func TestReadThreadArtifact(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := thread.NewStore(baseDir)
	agentImpl := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	if err := os.MkdirAll(filepath.Join(store.ThreadDir("thread-1"), "artifacts", "browser", "sha256"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.ThreadDir("thread-1"), "artifacts", "browser", "sha256", "shot.png"), []byte("\x89PNG\r\n\x1a\ntest"), 0o644); err != nil {
		t.Fatal(err)
	}

	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	browserMgr.SetStore(browser.NewStore(baseDir))
	h := New("", agent.NewConversationManager(&streamTestAgent{}), nil, nil, agentImpl, browserMgr)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/threads/thread-1/artifacts/read?uri=artifacts://artifacts/browser/sha256/shot.png")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var body api.ReadFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Path != "artifacts/browser/sha256/shot.png" {
		t.Fatalf("unexpected artifact path %q", body.Path)
	}
	if body.Encoding != "base64" {
		t.Fatalf("expected base64 encoding, got %q", body.Encoding)
	}
}

func TestReadThreadArtifact_RejectsTraversal(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := thread.NewStore(baseDir)
	agentImpl := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	browserMgr.SetStore(browser.NewStore(baseDir))
	h := New("", agent.NewConversationManager(&streamTestAgent{}), nil, nil, agentImpl, browserMgr)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/threads/thread-1/artifacts/read?uri=artifacts://../secrets.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(body))
	}
}

func testSecretHash(token string) string {
	salt := []byte("testsalt12345678")
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(token))
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(h.Sum(nil))
}

func TestBrowserCDPTrackerPersistsRequestAndResponse(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := thread.NewStore(baseDir)
	agentImpl := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	if err := store.SaveTurnState("thread-1", thread.TurnState{
		ID:          "turn-1",
		ThreadID:    "thread-1",
		CurrentStep: 2,
		Phase:       thread.PhaseTools,
	}); err != nil {
		t.Fatal(err)
	}
	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	browserStore := browser.NewStore(baseDir)
	browserMgr.SetStore(browserStore)
	browserMgr.SetCurrentTurnLoader(store.LoadTurnState)
	h := New("", agent.NewConversationManager(&streamTestAgent{}), nil, nil, agentImpl, browserMgr)
	tracker := h.newBrowserCDPTracker("thread-1")

	tracker.onClientMessage([]byte(`{"id":7,"method":"Browser.getVersion","params":{}}`))
	tracker.onServerMessage([]byte(`{"id":7,"result":{"product":"Chrome/123"}}`))

	entries, err := browserMgr.EventEntries("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 browser events, got %d", len(entries))
	}
	events := []thread.BrowserEvent{entries[0].Event, entries[1].Event}
	if events[0].Direction != "request" || events[0].Method != "Browser.getVersion" {
		t.Fatalf("unexpected request event %#v", events[0])
	}
	if string(events[0].Payload) != `{"id":7,"method":"Browser.getVersion","params":{}}` {
		t.Fatalf("unexpected request payload %q", string(events[0].Payload))
	}
	if events[1].Direction != "response" {
		t.Fatalf("unexpected response event %#v", events[1])
	}
	if string(events[1].Payload) != `{"id":7,"result":{"product":"Chrome/123"}}` {
		t.Fatalf("unexpected response payload %q", string(events[1].Payload))
	}
}

func TestBrowserCDPTrackerCapturesScreenshotAndEmitsChunk(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := thread.NewStore(baseDir)
	if err := store.SaveTurnState("thread-1", thread.TurnState{
		ID:          "turn-1",
		ThreadID:    "thread-1",
		CurrentStep: 2,
		Phase:       thread.PhaseTools,
	}); err != nil {
		t.Fatal(err)
	}

	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	browserMgr.SetStore(browser.NewStore(baseDir))

	var emitted []message.MessageChunk
	var capturedThreadID string
	tracker := &browserCDPTracker{
		threadID: "thread-1",
		captureScreenshot: func(_ context.Context, threadID string) ([]byte, error) {
			capturedThreadID = threadID
			return []byte("\x89PNG\r\n\x1a\nbrowser-test"), nil
		},
		emitChunk: func(chunk message.MessageChunk) {
			emitted = append(emitted, chunk)
		},
		currentTurn:    store.LoadTurnState,
		appendEvent:    browserMgr.AppendEvent,
		saveScreenshot: browserMgr.SaveScreenshot,
		pendingByID:    map[string]browserPendingApproval{},
	}

	tracker.onClientMessage([]byte(`{"id":7,"method":"Input.dispatchMouseEvent","params":{"type":"mousePressed"}}`))
	tracker.onServerMessage([]byte(`{"id":7,"result":{}}`))

	entries, err := browserMgr.EventEntries("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 browser events, got %d", len(entries))
	}
	events := []thread.BrowserEvent{entries[0].Event, entries[1].Event}
	if len(events[1].Files) != 1 {
		t.Fatalf("expected response event screenshot file, got %#v", events[1].Files)
	}
	if events[1].Files[0].MediaType != "image/png" {
		t.Fatalf("expected image/png screenshot, got %#v", events[1].Files[0])
	}
	saved, err := os.ReadFile(filepath.Join(store.ThreadDir("thread-1"), filepath.FromSlash(events[1].Files[0].Path)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, []byte("\x89PNG\r\n\x1a\nbrowser-test")) {
		t.Fatalf("unexpected saved screenshot %q", string(saved))
	}
	if capturedThreadID != "thread-1" {
		t.Fatalf("expected screenshot capture for thread-1, got %q", capturedThreadID)
	}
	if len(emitted) != 2 {
		t.Fatalf("expected 2 emitted chunks, got %d", len(emitted))
	}
	lastChunk, ok := emitted[1].(message.DataChunk)
	if !ok {
		t.Fatalf("expected DataChunk, got %T", emitted[1])
	}
	if lastChunk.DataType != "browser-event" {
		t.Fatalf("expected browser-event chunk, got %q", lastChunk.DataType)
	}
	var payload struct {
		ThreadID  string              `json:"threadId"`
		TurnID    string              `json:"turnId"`
		StepIndex int                 `json:"stepIndex"`
		Event     thread.BrowserEvent `json:"event"`
	}
	if err := json.Unmarshal(lastChunk.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ThreadID != "thread-1" || payload.TurnID != "turn-1" || payload.StepIndex != 2 {
		t.Fatalf("unexpected browser event chunk payload %#v", payload)
	}
	if len(payload.Event.Files) != 1 {
		t.Fatalf("expected chunk payload to include screenshot, got %#v", payload.Event)
	}
}

func TestBrowserCDPTrackerCapturesScreenshotEveryFiveCalls(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := thread.NewStore(baseDir)
	if err := store.SaveTurnState("thread-1", thread.TurnState{
		ID:          "turn-1",
		ThreadID:    "thread-1",
		CurrentStep: 2,
		Phase:       thread.PhaseTools,
	}); err != nil {
		t.Fatal(err)
	}

	browserMgr, err := browser.NewManager("session-1", t.TempDir(), 3002)
	if err != nil {
		t.Fatal(err)
	}
	defer browserMgr.Close()
	browserMgr.SetStore(browser.NewStore(baseDir))

	captureCount := 0
	tracker := &browserCDPTracker{
		threadID: "thread-1",
		captureScreenshot: func(_ context.Context, _ string) ([]byte, error) {
			captureCount++
			return []byte("\x89PNG\r\n\x1a\nbrowser-test"), nil
		},
		currentTurn:    store.LoadTurnState,
		appendEvent:    browserMgr.AppendEvent,
		saveScreenshot: browserMgr.SaveScreenshot,
		pendingByID:    map[string]browserPendingApproval{},
	}

	for i := range 5 {
		id := i + 1
		tracker.onClientMessage(fmt.Appendf(nil, `{"id":%d,"method":"Target.getTargets","params":{}}`, id))
		tracker.onServerMessage(fmt.Appendf(nil, `{"id":%d,"result":{"targetInfos":[]}}`, id))
	}

	if captureCount != 1 {
		t.Fatalf("expected one periodic screenshot after five calls, got %d", captureCount)
	}

	entries, err := browserMgr.EventEntries("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Fatalf("expected 10 browser events, got %d", len(entries))
	}
	if len(entries[9].Event.Files) != 1 {
		t.Fatalf("expected fifth response event to include screenshot, got %#v", entries[9].Event.Files)
	}
}

func TestShouldCaptureBrowserScreenshot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		request json.RawMessage
		payload json.RawMessage
		want    bool
	}{
		{
			name:   "navigate response captures",
			method: "Page.navigate",
			want:   true,
		},
		{
			name:   "create target skips",
			method: "Target.createTarget",
			want:   false,
		},
		{
			name:    "ready state probe skips before complete",
			method:  "Runtime.evaluate",
			request: json.RawMessage(`{"id":1,"method":"Runtime.evaluate","params":{"expression":"document.readyState"}}`),
			payload: json.RawMessage(`{"result":{"result":{"type":"string","value":"interactive"}}}`),
			want:    false,
		},
		{
			name:    "ready state complete captures",
			method:  "Runtime.evaluate",
			request: json.RawMessage(`{"id":1,"method":"Runtime.evaluate","params":{"expression":"document.readyState"}}`),
			payload: json.RawMessage(`{"result":{"result":{"type":"string","value":"complete"}}}`),
			want:    true,
		},
		{
			name:    "title marker probe skips",
			method:  "Runtime.evaluate",
			payload: json.RawMessage(`{"result":{"result":{"type":"string","value":"🟢 "}}}`),
			want:    false,
		},
		{
			name:    "url diagnostic capture stays enabled",
			method:  "Runtime.evaluate",
			payload: json.RawMessage(`{"result":{"result":{"type":"string","value":"{\"url\":\"https://example.com\"}"}}}`),
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldCaptureBrowserScreenshot(tt.method, tt.request, tt.payload); got != tt.want {
				t.Fatalf("shouldCaptureBrowserScreenshot(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}
