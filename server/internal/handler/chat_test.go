package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	api "github.com/boeing-ai-gateway/discboeing/server/api"
	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/jobs"
	"github.com/boeing-ai-gateway/discboeing/server/internal/middleware"
	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	mocksandbox "github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/mock"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/sandboxapi"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
	"github.com/boeing-ai-gateway/discboeing/server/internal/store"
)

const testProjectID = "test-project"

// setupChatTestStore creates an in-memory SQLite database for testing.
func setupChatTestStore(t *testing.T) *store.Store {
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

// seedWorkspace creates a workspace in the store for testing.
func seedWorkspace(t *testing.T, s *store.Store) {
	t.Helper()
	ctx := context.Background()

	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  testProjectID,
		Path:       "/workspace",
		SourceType: "local",
		Status:     "ready",
	}
	if err := s.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
}

// parseMessages unmarshals a JSON array string for use in api.ChatRequest.Messages.
func parseMessages(t *testing.T, s string) []api.Message {
	t.Helper()
	var msgs []api.Message
	if err := json.Unmarshal([]byte(s), &msgs); err != nil {
		t.Fatalf("parseMessages: %v", err)
	}
	return msgs
}

// seedSession creates a workspace and session in the store for testing.
func seedSession(t *testing.T, s *store.Store, sessionID string) {
	t.Helper()
	ctx := context.Background()

	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  testProjectID,
		Path:       "/workspace",
		SourceType: "local",
		Status:     "ready",
	}
	if err := s.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	workspacePath := "/workspace"
	session := &model.Session{
		ID:            sessionID,
		ProjectID:     testProjectID,
		WorkspaceID:   "test-workspace",
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusReady,
		WorkspacePath: &workspacePath,
	}
	if err := s.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
}

// newChatTestHandler creates a handler wired up with real services
// backed by the given store and mock sandbox provider.
func newChatTestHandler(t *testing.T, s *store.Store, provider *mocksandbox.Provider) *Handler {
	t.Helper()

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	cfg := &config.Config{
		SandboxIdleTimeout: 30 * time.Minute,
		WorkspaceDir:       t.TempDir(),
		EncryptionKey:      []byte("01234567890123456789012345678901"),
	}

	jobQueue := jobs.NewQueue(s, cfg)
	sandboxSvc := service.NewSandboxService(s, provider, cfg, nil, nil, jobQueue, nil)
	workspaceSvc := service.NewWorkspaceService(s, nil, sandboxSvc, nil, jobQueue)
	sessionSvc := service.NewSessionService(s, nil, sandboxSvc, nil, jobQueue)
	sandboxSvc.SetSessionInitializer(sessionSvc)
	chatSvc := service.NewChatService(s, cfg, sessionSvc, jobQueue, nil, sandboxSvc, nil)

	return &Handler{
		store:            s,
		cfg:              cfg,
		chatService:      chatSvc,
		sessionService:   sessionSvc,
		sandboxService:   sandboxSvc,
		workspaceService: workspaceSvc,
		jobQueue:         jobQueue,
		shutdownCtx:      shutdownCtx,
		shutdownCancel:   shutdownCancel,
	}
}

// makeChatRequest builds an http.Request for the Chat endpoint with the project ID set in context.
func makeChatRequest(ctx context.Context, t *testing.T, sessionID, threadID string, req api.ChatRequest) *http.Request {
	t.Helper()
	if threadID == "" {
		threadID = sessionID
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	httpReq := httptest.NewRequest("POST", "/api/projects/"+testProjectID+"/sessions/"+sessionID+"/threads/"+threadID+"/chat", bytes.NewReader(body))
	httpReq.SetPathValue("sessionId", sessionID)
	httpReq.SetPathValue("threadId", threadID)
	ctx = context.WithValue(ctx, middleware.ProjectIDKey, testProjectID)
	return httpReq.WithContext(ctx)
}

// TestChat_GetSessionByID_UnexpectedError verifies that the Chat handler returns
// a 500 Internal Server Error when GetSessionByID fails with a non-ErrNotFound error
// (e.g., a database failure), rather than falling through to create a new session.
func TestChat_GetSessionByID_UnexpectedError(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	h := newChatTestHandler(t, s, provider)

	// Close the underlying DB to cause all queries to fail with a non-ErrNotFound error
	sqlDB, err := s.DB().DB()
	if err != nil {
		t.Fatalf("failed to get underlying DB: %v", err)
	}
	sqlDB.Close()

	req := makeChatRequest(context.Background(), t, "session-123", "", api.ChatRequest{
		Messages:    parseMessages(t, `[{"role":"user","parts":[{"type":"text","text":"hello"}]}]`),
		WorkspaceID: "test-workspace",
	})
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d; body: %s",
			http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestChat_EmptyMessages_CreatesSessionWithoutSendingToSandbox(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	seedWorkspace(t, s)

	getCalled := false
	provider.GetFunc = func(_ context.Context, _ string) (*sandbox.Sandbox, error) {
		getCalled = true
		t.Fatalf("sandbox should not be contacted for empty chat submissions")
		return nil, nil
	}

	h := newChatTestHandler(t, s, provider)

	req := makeChatRequest(context.Background(), t, "session-empty-create", "", api.ChatRequest{
		Messages:    []api.Message{},
		WorkspaceID: "test-workspace",
	})
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusOK, w.Code, w.Body.String())
	}
	var response api.ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response, got error: %v; body: %s", err, w.Body.String())
	}
	if response.SessionID != "session-empty-create" {
		t.Fatalf("expected session ID in response, got %q", response.SessionID)
	}
	if response.WorkspaceID != "test-workspace" {
		t.Fatalf("expected workspace ID in response, got %q", response.WorkspaceID)
	}
	if response.ThreadID != "session-empty-create" {
		t.Fatalf("expected default thread ID to match session ID, got %q", response.ThreadID)
	}
	if stringValue(response.MessageID) != "" {
		t.Fatalf("expected empty message ID for empty submission, got %q", stringValue(response.MessageID))
	}
	if getCalled {
		t.Fatal("sandbox should not be contacted for empty chat submissions")
	}

	sess, err := s.GetSessionByID(context.Background(), "session-empty-create")
	if err != nil {
		t.Fatalf("expected session to be created: %v", err)
	}
	if sess.Name != "" {
		t.Fatalf("expected empty session name, got %q", sess.Name)
	}
}

// TestChat_ClientDisconnect_DoesNotCancelSandbox verifies that when a client
// disconnects after chat initiation, the background sandbox request still uses
// a non-cancelled context.
func TestChat_ClientDisconnect_DoesNotCancelSandbox(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	sessionID := "session-ctx-test"

	seedSession(t, s, sessionID)

	ctx := context.Background()
	_, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{
		SharedSecret:  "test-secret",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	var (
		mu                  sync.Mutex
		postCalled          = make(chan struct{}, 1)
		proceedAfterPost    = make(chan struct{})
		contextWasCancelled bool
		contextChecked      bool
	)

	provider.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/health") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"healthy":true,"connected":true}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/chat") && r.Method == "POST" {
			select {
			case postCalled <- struct{}{}:
			default:
			}

			<-proceedAfterPost

			mu.Lock()
			contextWasCancelled = r.Context().Err() != nil
			contextChecked = true
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"started"}`))
			return
		}
		http.NotFound(w, r)
	})

	h := newChatTestHandler(t, s, provider)

	reqCtx, cancelReq := context.WithCancel(context.Background())
	req := makeChatRequest(reqCtx, t, sessionID, "", api.ChatRequest{
		Messages: parseMessages(t, `[{"role":"user","parts":[{"type":"text","text":"hello"}]}]`),
	})
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.Chat(w, req)
		close(done)
	}()

	select {
	case <-postCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sandbox POST /chat request")
	}

	cancelReq()
	<-reqCtx.Done()
	close(proceedAfterPost)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for chat handler to return")
	}

	mu.Lock()
	wasChecked := contextChecked
	wasCancelled := contextWasCancelled
	mu.Unlock()

	if !wasChecked {
		t.Fatal("sandbox request context was never checked")
	}
	if wasCancelled {
		t.Error("context passed to sandbox request was cancelled; client disconnect should not cancel sandbox operations")
	}
}

// TestChat_StartsCompletion_StatusBecomesRunning verifies that a normal chat
// request starts the sandbox completion and marks the session as running.
func TestChat_StartsCompletion_StatusBecomesRunning(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	sessionID := "session-status-test"

	seedSession(t, s, sessionID)

	// Create and start the sandbox
	ctx := context.Background()
	_, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{
		SharedSecret:  "test-secret",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	h := newChatTestHandler(t, s, provider)

	req := makeChatRequest(context.Background(), t, sessionID, "", api.ChatRequest{
		Messages: parseMessages(t, `[{"id":"msg-1","role":"user","parts":[{"type":"text","text":"hello"}]}]`),
	})
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var response api.ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response, got error: %v; body: %s", err, w.Body.String())
	}
	if response.SessionID != sessionID {
		t.Fatalf("expected session ID %q, got %q", sessionID, response.SessionID)
	}
	if response.ThreadID != sessionID {
		t.Fatalf("expected default thread ID %q, got %q", sessionID, response.ThreadID)
	}
	if response.WorkspaceID != "test-workspace" {
		t.Fatalf("expected workspace ID %q, got %q", "test-workspace", response.WorkspaceID)
	}
	if stringValue(response.MessageID) != "msg-1" {
		t.Fatalf("expected message ID %q, got %q", "msg-1", stringValue(response.MessageID))
	}

	session, err := s.GetSessionByID(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if session.SandboxStatus != model.SessionStatusReady {
		t.Errorf("expected session status to remain %q after chat start, got %q",
			model.SessionStatusReady, session.SandboxStatus)
	}
}

func TestChat_PersistsChatStartErrorsOnTheSession(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	sessionID := "session-chat-error-test"

	seedSession(t, s, sessionID)

	ctx := context.Background()
	_, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{
		SharedSecret:  "test-secret",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	provider.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/health") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"healthy":true,"connected":true}`))
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/chat") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_api_key"}`))
			return
		}
		http.NotFound(w, r)
	})

	h := newChatTestHandler(t, s, provider)
	req := makeChatRequest(context.Background(), t, sessionID, "", api.ChatRequest{
		Messages: parseMessages(t, `[{"id":"msg-error","role":"user","parts":[{"type":"text","text":"hello"}]}]`),
	})
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusBadGateway, w.Code, w.Body.String())
	}

	session, err := s.GetSessionByID(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if session.ErrorMessage == nil {
		t.Fatal("expected chat start error to be persisted on the session")
	}
	if got := *session.ErrorMessage; got != "sandbox returned status 400: invalid_api_key" {
		t.Fatalf("expected persisted session error %q, got %q", "sandbox returned status 400: invalid_api_key", got)
	}

	submission, err := s.GetPromptSubmissionByMessageID(context.Background(), sessionID, sessionID, "msg-error")
	if err != nil {
		t.Fatalf("expected prompt submission to be persisted: %v", err)
	}
	if len(submission.MessagesEncryptedData) == 0 {
		t.Fatal("expected failed prompt submission payload to remain encrypted for retry")
	}
	if bytes.Contains(submission.MessagesEncryptedData, []byte("hello")) {
		t.Fatal("expected prompt submission payload to be encrypted at rest")
	}

	serviceSession, err := h.sessionService.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("failed to load session through the service: %v", err)
	}
	response := mapSessionResponse(serviceSession)
	if response.SandboxStatus != model.SessionStatusReady {
		t.Fatalf("expected session status %q, got %q", model.SessionStatusReady, response.SandboxStatus)
	}
	if response.ErrorMessage != "sandbox returned status 400: invalid_api_key" {
		t.Fatalf("expected response error message %q, got %q", "sandbox returned status 400: invalid_api_key", response.ErrorMessage)
	}
}

func TestChat_ClearsPersistedChatStartErrorsAfterSuccessfulRetry(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	sessionID := "session-chat-error-clear-test"

	seedSession(t, s, sessionID)

	ctx := context.Background()
	_, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{
		SharedSecret:  "test-secret",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	session, err := s.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("failed to get seeded session: %v", err)
	}
	staleError := "sandbox returned status 400: invalid_api_key"
	session.ErrorMessage = &staleError
	if err := s.UpdateSession(ctx, session); err != nil {
		t.Fatalf("failed to seed session error: %v", err)
	}

	provider.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/health") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"healthy":true,"connected":true}`))
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/chat") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"started"}`))
			return
		}
		http.NotFound(w, r)
	})

	h := newChatTestHandler(t, s, provider)
	req := makeChatRequest(context.Background(), t, sessionID, "", api.ChatRequest{
		Messages: parseMessages(t, `[{"id":"msg-retry","role":"user","parts":[{"type":"text","text":"retry"}]}]`),
	})
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	updatedSession, err := s.GetSessionByID(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if updatedSession.ErrorMessage != nil {
		t.Fatalf("expected persisted session error to be cleared, got %q", *updatedSession.ErrorMessage)
	}
}

func TestChat_RetryUsesPersistedPromptSubmission(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	sessionID := "session-retry-prompt"

	seedSession(t, s, sessionID)

	ctx := context.Background()
	_, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{
		SharedSecret:  "test-secret",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	postCount := 0
	provider.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/health") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"healthy":true,"connected":true}`))
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/chat") {
			postCount++
			if postCount > 1 {
				t.Fatalf("expected one POST /chat, got %d", postCount)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"started","completionId":"completion-1"}`))
			return
		}
		http.NotFound(w, r)
	})

	h := newChatTestHandler(t, s, provider)
	chatReq := api.ChatRequest{
		Messages: parseMessages(t, `[{"id":"msg-retry-persisted","role":"user","parts":[{"type":"text","text":"hello"}]}]`),
	}

	w := httptest.NewRecorder()
	h.Chat(w, makeChatRequest(context.Background(), t, sessionID, "", chatReq))
	if w.Code != http.StatusOK {
		t.Fatalf("expected first status %d, got %d; body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.Chat(w, makeChatRequest(context.Background(), t, sessionID, "", chatReq))
	if w.Code != http.StatusOK {
		t.Fatalf("expected retry status %d, got %d; body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response api.ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response, got error: %v; body: %s", err, w.Body.String())
	}
	if stringValue(response.SubmissionID) == "" {
		t.Fatal("expected submission ID in response")
	}
	if stringValue(response.CompletionID) != "completion-1" {
		t.Fatalf("expected completion ID %q, got %q", "completion-1", stringValue(response.CompletionID))
	}
	submission, err := s.GetPromptSubmissionByID(context.Background(), stringValue(response.SubmissionID))
	if err != nil {
		t.Fatalf("expected prompt submission to exist: %v", err)
	}
	if len(submission.MessagesEncryptedData) != 0 {
		t.Fatal("expected accepted prompt submission payload to be deleted promptly")
	}
	if postCount != 1 {
		t.Fatalf("expected one POST /chat, got %d", postCount)
	}
}

func TestChat_UsesExplicitThreadID(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	sessionID := "session-explicit-thread"
	threadID := "thread-custom-1"

	seedSession(t, s, sessionID)

	ctx := context.Background()
	_, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{
		SharedSecret:  "test-secret",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	pathCh := make(chan string, 1)
	provider.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/health") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"healthy":true,"connected":true}`))
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/chat") {
			select {
			case pathCh <- r.URL.Path:
			default:
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"started"}`))
			return
		}
		http.NotFound(w, r)
	})

	h := newChatTestHandler(t, s, provider)
	req := makeChatRequest(context.Background(), t, sessionID, threadID, api.ChatRequest{
		Messages: parseMessages(t, `[{"id":"msg-thread","role":"user","parts":[{"type":"text","text":"hello"}]}]`),
	})
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var response api.ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response, got error: %v; body: %s", err, w.Body.String())
	}
	if response.SessionID != sessionID {
		t.Fatalf("expected session ID %q, got %q", sessionID, response.SessionID)
	}
	if response.ThreadID != threadID {
		t.Fatalf("expected thread ID %q, got %q", threadID, response.ThreadID)
	}

	select {
	case path := <-pathCh:
		expected := "/threads/" + threadID + "/chat"
		if path != expected {
			t.Fatalf("expected sandbox chat path %q, got %q", expected, path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sandbox POST /chat request")
	}
}

// TestChat_ReturnsJSONResponse verifies that chat initiation returns JSON
// metadata instead of proxying the SSE stream body.
func TestChat_ReturnsJSONResponse(t *testing.T) {
	s := setupChatTestStore(t)
	provider := mocksandbox.NewProvider()
	sessionID := "session-json-response-test"

	seedSession(t, s, sessionID)

	ctx := context.Background()
	_, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{
		SharedSecret:  "test-secret",
		WorkspacePath: "/workspace",
	})
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	h := newChatTestHandler(t, s, provider)

	req := makeChatRequest(context.Background(), t, sessionID, "", api.ChatRequest{
		Messages: parseMessages(t, `[{"id":"msg-json","role":"user","parts":[{"type":"text","text":"hello"}]}]`),
	})
	w := httptest.NewRecorder()

	h.Chat(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var response api.ChatResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response, got error: %v; body: %s", err, w.Body.String())
	}
	if response.SessionID != sessionID {
		t.Fatalf("expected session ID %q, got %q", sessionID, response.SessionID)
	}
	if response.ThreadID != sessionID {
		t.Fatalf("expected default thread ID %q, got %q", sessionID, response.ThreadID)
	}
	if stringValue(response.MessageID) != "msg-json" {
		t.Fatalf("expected message ID %q, got %q", "msg-json", stringValue(response.MessageID))
	}
	if bytes.Contains(w.Body.Bytes(), []byte("data: [DONE]")) {
		t.Fatalf("expected JSON response instead of SSE body, got: %s", w.Body.String())
	}
}

func TestApprovedCommitPullMetadata(t *testing.T) {
	metadataJSON := []byte(`{"directory":"subdir","commitHash":"abc123def456"}`)
	tests := []struct {
		name     string
		question *sandboxapi.PendingQuestionResponse
		answers  map[string]string
		want     bool
	}{
		{
			name: "approved commit pull",
			question: &sandboxapi.PendingQuestionResponse{
				Status:   "pending",
				Question: &sandboxapi.PendingQuestion{Context: requestCommitPullApprovalContext, Metadata: metadataJSON},
			},
			answers: map[string]string{requestCommitPullApprovedKey: "true"},
			want:    true,
		},
		{
			name: "rejected commit pull",
			question: &sandboxapi.PendingQuestionResponse{
				Status:   "pending",
				Question: &sandboxapi.PendingQuestion{Context: requestCommitPullApprovalContext, Metadata: metadataJSON},
			},
			answers: map[string]string{requestCommitPullRejectedKey: "true"},
			want:    false,
		},
		{
			name: "different approval context",
			question: &sandboxapi.PendingQuestionResponse{
				Status:   "pending",
				Question: &sandboxapi.PendingQuestion{Context: "request_user_credential", Metadata: metadataJSON},
			},
			answers: map[string]string{requestCommitPullApprovedKey: "true"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := approvedCommitPullMetadata(tt.question, tt.answers)
			if ok != tt.want {
				t.Fatalf("approvedCommitPullMetadata() ok = %v, want %v", ok, tt.want)
			}
			if tt.want && (got.Directory != "subdir" || got.CommitHash != "abc123def456") {
				t.Fatalf("unexpected metadata: %+v", got)
			}
		})
	}
}
