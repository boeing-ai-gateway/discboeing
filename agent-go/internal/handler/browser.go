package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/browser"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func (h *Handler) GetBrowserSession(w http.ResponseWriter, r *http.Request) {
	if !h.validBrowserSession(chi.URLParam(r, "sessionId")) {
		h.Error(w, http.StatusNotFound, "browser session not found")
		return
	}
	info := h.browserManager.Info()
	h.JSON(w, http.StatusOK, api.BrowserSessionResponse{
		SessionID:     info.SessionID,
		Running:       info.Running,
		WebSocketPath: info.WebSocketPath,
		WebSocketURL:  info.WebSocketURL,
		Token:         info.Token,
		UserDataDir:   info.UserDataDir,
		LastError:     info.LastError,
	})
}

func (h *Handler) ProxyBrowserCDP(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	threadID := strings.TrimSpace(r.URL.Query().Get("threadId"))
	if !h.validBrowserSession(sessionID) {
		http.Error(w, "browser session not found", http.StatusNotFound)
		return
	}
	if !h.browserManager.TokenMatches(r.URL.Query().Get("token")) {
		log.Printf("browser[%s]: rejected cdp connect invalid token thread=%q", sessionID, threadID)
		http.Error(w, "invalid browser token", http.StatusUnauthorized)
		return
	}

	log.Printf("browser[%s]: accepted cdp connect thread=%q", sessionID, threadID)
	upstreamURL, err := h.browserManager.UpstreamWebSocketURL(r.Context())
	if err != nil {
		log.Printf("browser[%s]: upstream websocket unavailable thread=%q: %v", sessionID, threadID, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	upstreamConn, _, err := websocket.Dial(r.Context(), upstreamURL, &websocket.DialOptions{
		HTTPClient: browserCDPHTTPClient,
	})
	if err != nil {
		log.Printf("browser[%s]: upstream websocket dial failed thread=%q upstream=%s: %v", sessionID, threadID, upstreamURL, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	browser.ConfigureCDPConn(upstreamConn)
	defer upstreamConn.Close(websocket.StatusInternalError, "proxy closing")

	clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"127.0.0.1", "localhost"},
	})
	if err != nil {
		log.Printf("browser[%s]: client websocket accept failed thread=%q: %v", sessionID, threadID, err)
		return
	}
	browser.ConfigureCDPConn(clientConn)
	defer clientConn.Close(websocket.StatusInternalError, "proxy closing")

	tracker := h.newBrowserCDPTracker(strings.TrimSpace(r.URL.Query().Get("threadId")))

	errCh := make(chan error, 2)
	go proxyWebSocket(r.Context(), clientConn, upstreamConn, errCh, tracker.onClientMessage)
	go proxyWebSocket(r.Context(), upstreamConn, clientConn, errCh, tracker.onServerMessage)

	if err := <-errCh; err != nil && err != io.EOF {
		log.Printf("browser[%s]: cdp proxy error thread=%q: %v", sessionID, threadID, err)
		_ = clientConn.Close(websocket.StatusInternalError, err.Error())
		_ = upstreamConn.Close(websocket.StatusInternalError, err.Error())
		return
	}
	log.Printf("browser[%s]: cdp proxy closed thread=%q", sessionID, threadID)
	_ = clientConn.Close(websocket.StatusNormalClosure, "done")
	_ = upstreamConn.Close(websocket.StatusNormalClosure, "done")
}

func proxyWebSocket(ctx context.Context, src *websocket.Conn, dst *websocket.Conn, errCh chan<- error, intercept func([]byte)) {
	for {
		msgType, payload, err := src.Read(ctx)
		if err != nil {
			errCh <- err
			return
		}
		if intercept != nil && msgType == websocket.MessageText {
			intercept(payload)
		}
		if err := dst.Write(ctx, msgType, payload); err != nil {
			errCh <- err
			return
		}
	}
}

var browserCDPHTTPClient = &http.Client{
	Transport: &http.Transport{Proxy: nil},
}

func (h *Handler) validBrowserSession(sessionID string) bool {
	return h.browserManager != nil && sessionID == h.browserManager.SessionID()
}

type browserCDPTracker struct {
	threadID string
	store    *thread.Store

	captureScreenshot func(context.Context, string) ([]byte, error)
	emitChunk         func(message.MessageChunk)

	mu                       sync.Mutex
	pendingByID              map[string]browserPendingApproval
	callsSinceLastScreenshot int
}

type browserPendingApproval struct {
	turnID             string
	assistantMessageID string
	stepIndex          int
	method             string
	requestPayload     json.RawMessage
	eventID            string
}

type browserCDPMessage struct {
	ID     json.RawMessage  `json:"id,omitempty"`
	Method string           `json:"method,omitempty"`
	Error  *json.RawMessage `json:"error,omitempty"`
	Result json.RawMessage  `json:"result,omitempty"`
}

func (h *Handler) newBrowserCDPTracker(threadID string) *browserCDPTracker {
	if threadID == "" || h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return &browserCDPTracker{}
	}
	return &browserCDPTracker{
		threadID: threadID,
		store:    h.defaultAgent.Store(),
		captureScreenshot: func(ctx context.Context, threadID string) ([]byte, error) {
			if h.browserManager == nil {
				return nil, fmt.Errorf("browser manager unavailable")
			}
			return h.browserManager.CaptureScreenshot(ctx, threadID)
		},
		emitChunk: func(chunk message.MessageChunk) {
			if h.completions == nil {
				return
			}
			h.completions.EmitChunkIfActive(threadID, chunk)
		},
		pendingByID: map[string]browserPendingApproval{},
	}
}

func (t *browserCDPTracker) onClientMessage(payload []byte) {
	if t.store == nil || t.threadID == "" {
		return
	}
	msgID, method := parseBrowserCDPRequest(payload)
	if msgID == "" {
		return
	}
	turnState, err := t.store.LoadTurnState(t.threadID)
	if err != nil || turnState == nil {
		return
	}
	approvalID := browserApprovalID()
	event := thread.BrowserEvent{
		EventID:   approvalID,
		RequestID: msgID,
		Method:    method,
		Direction: "request",
		Payload:   json.RawMessage(payload),
	}
	if err := t.store.AppendBrowserEvent(t.threadID, turnState.ID, turnState.CurrentStep, event); err != nil {
		return
	}
	t.emitBrowserEvent(turnState.ID, strings.TrimSpace(turnState.AssistantMsgID), turnState.CurrentStep, event)
	t.mu.Lock()
	t.pendingByID[msgID] = browserPendingApproval{
		turnID:             turnState.ID,
		assistantMessageID: strings.TrimSpace(turnState.AssistantMsgID),
		stepIndex:          turnState.CurrentStep,
		method:             method,
		requestPayload:     json.RawMessage(append([]byte(nil), payload...)),
		eventID:            approvalID,
	}
	t.mu.Unlock()
}

func (t *browserCDPTracker) onServerMessage(payload []byte) {
	if t.store == nil || t.threadID == "" {
		return
	}
	msgID, hasError := parseBrowserCDPResponse(payload)
	if msgID == "" {
		return
	}
	t.mu.Lock()
	pending, ok := t.pendingByID[msgID]
	if ok {
		delete(t.pendingByID, msgID)
	}
	shouldCaptureForInterval := false
	if ok && !hasError {
		t.callsSinceLastScreenshot++
		shouldCaptureForInterval = t.callsSinceLastScreenshot >= browserScreenshotCallInterval
	}
	t.mu.Unlock()
	if !ok {
		return
	}
	event := thread.BrowserEvent{
		EventID:   fmt.Sprintf("%s-response", pending.eventID),
		RequestID: msgID,
		Method:    pending.method,
		Direction: "response",
		Payload:   json.RawMessage(payload),
	}
	shouldCaptureForAction := shouldCaptureBrowserScreenshot(
		pending.method,
		pending.requestPayload,
		event.Payload,
	)
	if (shouldCaptureForAction || shouldCaptureForInterval) && !hasError {
		if screenshot, err := t.captureScreenshotForEvent(pending); err != nil {
			log.Printf("browser thread[%s]: capture screenshot failed turn=%s step=%d request=%s method=%s: %v", t.threadID, pending.turnID, pending.stepIndex, msgID, pending.method, err)
		} else if screenshot.Path != "" {
			t.markScreenshotCaptured()
			event.Files = append(event.Files, screenshot)
		}
	}
	if err := t.store.AppendBrowserEvent(t.threadID, pending.turnID, pending.stepIndex, event); err != nil {
		return
	}
	t.emitBrowserEvent(pending.turnID, pending.assistantMessageID, pending.stepIndex, event)
}

func parseBrowserCDPRequest(payload []byte) (string, string) {
	var msg browserCDPMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return "", ""
	}
	return normalizeBrowserCDPID(msg.ID), strings.TrimSpace(msg.Method)
}

func parseBrowserCDPResponse(payload []byte) (string, bool) {
	var msg browserCDPMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return "", false
	}
	return normalizeBrowserCDPID(msg.ID), msg.Error != nil
}

func normalizeBrowserCDPID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return fmt.Sprint(value)
}

func browserApprovalID() string {
	return fmt.Sprintf("browser-%d", time.Now().UTC().UnixNano())
}

const browserScreenshotCallInterval = 5

func shouldCaptureBrowserScreenshot(method string, requestPayload, responsePayload json.RawMessage) bool {
	method = strings.TrimSpace(method)
	if method == "" {
		return false
	}
	if strings.HasPrefix(method, "Input.") {
		return true
	}
	if strings.HasPrefix(method, "Emulation.") {
		return true
	}
	switch method {
	case "DOM.setFileInputFiles",
		"Page.navigate",
		"Page.reload",
		"Page.goBack",
		"Page.goForward",
		"Page.handleJavaScriptDialog",
		"Target.activateTarget",
		"Target.closeTarget":
		return true
	case "Runtime.evaluate":
		return shouldCaptureForRuntimeEvaluate(requestPayload, responsePayload)
	case "Runtime.callFunctionOn":
		return true
	default:
		return false
	}
}

func shouldCaptureForRuntimeEvaluate(requestPayload, responsePayload json.RawMessage) bool {
	var response struct {
		Result struct {
			Result struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return true
	}
	value := strings.TrimSpace(response.Result.Result.Value)
	if strings.HasPrefix(value, "🟢") {
		return false
	}
	if isDocumentReadyStateRequest(requestPayload) && value == "complete" {
		return true
	}
	switch value {
	case "", "interactive", "loading", "complete", "undefined":
		return false
	default:
		return true
	}
}

func isDocumentReadyStateRequest(payload json.RawMessage) bool {
	var request struct {
		Method string `json:"method"`
		Params struct {
			Expression string `json:"expression"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &request); err != nil {
		return false
	}
	if strings.TrimSpace(request.Method) != "Runtime.evaluate" {
		return false
	}
	return strings.TrimSpace(request.Params.Expression) == "document.readyState"
}

func (t *browserCDPTracker) captureScreenshotForEvent(pending browserPendingApproval) (thread.BrowserEventFile, error) {
	if t.captureScreenshot == nil || t.store == nil {
		return thread.BrowserEventFile{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	png, err := t.captureScreenshot(ctx, t.threadID)
	if err != nil {
		return thread.BrowserEventFile{}, err
	}
	if len(png) == 0 {
		return thread.BrowserEventFile{}, nil
	}
	return t.store.SaveBrowserScreenshot(t.threadID, pending.turnID, pending.stepIndex, pending.eventID, png)
}

func (t *browserCDPTracker) markScreenshotCaptured() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callsSinceLastScreenshot = 0
}

func (t *browserCDPTracker) emitBrowserEvent(turnID, assistantMessageID string, stepIndex int, event thread.BrowserEvent) {
	if t.emitChunk == nil {
		return
	}
	data, err := json.Marshal(browserEventChunkPayload{
		ThreadID:           t.threadID,
		TurnID:             turnID,
		AssistantMessageID: assistantMessageID,
		StepIndex:          stepIndex,
		Event:              event,
	})
	if err != nil {
		return
	}
	t.emitChunk(message.DataChunk{
		DataType: "browser-event",
		Data:     data,
	})
}

type browserEventChunkPayload struct {
	ThreadID           string              `json:"threadId"`
	TurnID             string              `json:"turnId"`
	AssistantMessageID string              `json:"assistantMessageId,omitempty"`
	StepIndex          int                 `json:"stepIndex"`
	Event              thread.BrowserEvent `json:"event"`
}
