package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
	"github.com/obot-platform/discobot/server/internal/service"
	"github.com/obot-platform/discobot/server/internal/store"
)

const (
	requestCommitPullApprovalContext = "request_commit_pull"
	requestCommitPullApprovedKey     = "__request_commit_pull_approved__"
	requestCommitPullRejectedKey     = "__request_commit_pull_rejected__"
)

type requestCommitPullMetadata struct {
	Directory   string `json:"directory"`
	BaseCommit  string `json:"baseCommit,omitempty"`
	CommitHash  string `json:"commitHash"`
	CommitTitle string `json:"commitTitle,omitempty"`
	CommitBody  string `json:"commitBody,omitempty"`
}

// ChatRequest represents the request body for the chat endpoint.
// This matches the AI SDK's DefaultChatTransport format.
// Each element is a single UIMessage encoded as JSON.
type ChatRequest struct {
	// Messages is optional for create-only requests. When omitted or [], the
	// handler creates/validates the session and returns an immediate empty SSE completion.
	Messages []json.RawMessage `json:"messages"`
	// Trigger indicates the type of request: "submit-message" or "regenerate-message"
	Trigger string `json:"trigger,omitempty"`
	// WorkspaceID is optional for new sessions.
	// If omitted, the server creates a local workspace under Discobot's data directory.
	WorkspaceID string `json:"workspaceId,omitempty"`
	// ProviderID is optional for new sessions and selects a sandbox provider instance.
	ProviderID string `json:"providerId,omitempty"`
	// Model is optional for new sessions.
	Model string `json:"model,omitempty"`
	// Reasoning controls extended thinking. This is passed through as a string
	// reasoning level such as "auto", "low", "medium", "high", "xhigh",
	// "none", "default", or "" for model/provider default behavior.
	Reasoning string `json:"reasoning,omitempty"`
	// Mode is the permission mode: "plan" for planning mode, "" for default (build mode)
	Mode string `json:"mode,omitempty"`
	// RunAfter queues the prompt until the given RFC3339 timestamp, even if the thread is idle.
	RunAfter string `json:"runAfter,omitempty"`
}

type ChatResponse struct {
	WorkspaceID    string `json:"workspaceId"`
	SessionID      string `json:"sessionId"`
	ThreadID       string `json:"threadId"`
	SubmissionID   string `json:"submissionId,omitempty"`
	MessageID      string `json:"messageId,omitempty"`
	CompletionID   string `json:"completionId,omitempty"`
	Status         string `json:"status,omitempty"`
	QueuedPromptID string `json:"queuedPromptId,omitempty"`
}

// Chat handles AI chat initiation.
// POST /api/projects/{projectId}/sessions/{sessionId}/threads/{threadId}/chat
// Request body: { messages, workspaceId?, trigger?, messageId?, model?, reasoning?, mode? }
// Response: JSON metadata for the initiated chat request
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)

	// Parse request
	var req ChatRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	emptySubmission := len(req.Messages) == 0

	sessionID := r.PathValue("sessionId")
	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	threadID := r.PathValue("threadId")
	if threadID == "" {
		h.Error(w, http.StatusBadRequest, "threadId is required")
		return
	}
	// Check if session exists
	existingSession, err := h.chatService.GetSessionByID(ctx, sessionID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		// Unexpected error (DB failure, context cancelled, etc.) — don't fall through
		h.Error(w, http.StatusInternalServerError, "failed to look up session")
		return
	}

	sessionWorkspaceID := ""

	if existingSession != nil {
		// Session exists - validate it belongs to this project
		if existingSession.ProjectID != projectID {
			h.Error(w, http.StatusForbidden, "session does not belong to this project")
			return
		}
		// For existing sessions, validate session resources still belong to project
		if err := h.chatService.ValidateSessionResources(ctx, projectID, existingSession); err != nil {
			h.Error(w, http.StatusForbidden, err.Error())
			return
		}
		// Block chat during commit states for real prompt submissions.
		if !emptySubmission && (existingSession.CommitStatus == "pending" || existingSession.CommitStatus == "committing") {
			h.Error(w, http.StatusConflict, "Cannot send messages while session is committing")
			return
		}
		sessionWorkspaceID = existingSession.WorkspaceID
	} else {
		// Session doesn't exist - create it
		req.ProviderID = strings.TrimSpace(req.ProviderID)
		if err := h.sandboxService.ValidateSandboxProviderID(ctx, projectID, req.ProviderID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				h.Error(w, http.StatusBadRequest, "Sandbox provider not found")
				return
			}
			h.Error(w, http.StatusInternalServerError, "Failed to validate sandbox provider")
			return
		}

		workspaceID, err := h.resolveWorkspaceIDForNewSession(ctx, projectID, req.WorkspaceID)
		if err != nil {
			h.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessionWorkspaceID = workspaceID

		_, err = h.chatService.NewSession(ctx, service.NewSessionRequest{
			SessionID:   sessionID,
			ProjectID:   projectID,
			WorkspaceID: workspaceID,
			ProviderID:  req.ProviderID,
			Messages:    req.Messages,
		})
		if err != nil {
			h.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	response := ChatResponse{
		WorkspaceID: sessionWorkspaceID,
		SessionID:   sessionID,
		ThreadID:    threadID,
		MessageID:   lastUserMessageID(req.Messages),
	}

	if emptySubmission {
		h.JSON(w, http.StatusOK, response)
		return
	}

	// Use a context that won't be cancelled when the client disconnects.
	// This ensures prompt persistence and dispatch continue even if the
	// client aborts the request. The explicit cancel endpoint
	// (/sessions/{sessionId}/threads/{threadId}/cancel) remains the way to stop a running chat completion.
	sendCtx := context.WithoutCancel(ctx)
	submission, started, err := h.chatService.SubmitPrompt(sendCtx, projectID, sessionID, threadID, req.Messages, req.Model, req.Reasoning, req.Mode, req.RunAfter)
	if err != nil {
		log.Printf("[Chat] Failed to start chat for session %s: %v", sessionID, err)
		var startErr *service.SandboxChatStartError
		if errors.As(err, &startErr) {
			switch startErr.ErrorCode {
			case "completion_in_progress":
				h.JSON(w, http.StatusConflict, sandboxapi.ChatConflictResponse{
					Error:        startErr.ErrorCode,
					CompletionID: startErr.CompletionID,
				})
				return
			case "pending_question_requires_answer":
				h.JSON(w, http.StatusConflict, sandboxapi.ChatTurnStateConflictResponse{
					Error:      startErr.ErrorCode,
					Message:    startErr.Message,
					QuestionID: startErr.QuestionID,
				})
				return
			}
		}
		errMsg := err.Error()
		if _, updateErr := h.sessionService.UpdateErrorMessage(ctx, projectID, sessionID, &errMsg); updateErr != nil {
			log.Printf("[Chat] Failed to persist chat start error for session %s: %v", sessionID, updateErr)
		}
		h.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	if _, err := h.sessionService.UpdateErrorMessage(ctx, projectID, sessionID, nil); err != nil {
		log.Printf("[Chat] Failed to clear chat start error for session %s: %v", sessionID, err)
	}
	if submission != nil {
		response.SubmissionID = submission.ID
	}
	response.CompletionID = started.CompletionID
	response.Status = started.Status
	response.QueuedPromptID = started.QueuedPromptID

	h.JSON(w, http.StatusOK, response)
}

// ChatStream proxies the reusable thread chat SSE stream.
// GET /api/projects/{projectId}/sessions/{sessionId}/threads/{threadId}/stream
func (h *Handler) ChatStream(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := h.withShutdownContext(r.Context())
	defer cancel()

	projectID := middleware.GetProjectID(ctx)
	sessionID, threadID, _, ok := h.resolveSessionAndThread(w, r, projectID, false)
	if !ok {
		return
	}

	lastEventID := r.Header.Get("Last-Event-ID")

	// Get the stream from sandbox. Fresh requests replay persisted history by
	// default; valid Last-Event-ID reconnects continue from the requested offset.
	sseCh, err := h.chatService.GetStream(ctx, projectID, sessionID, threadID, lastEventID)
	if err != nil {
		log.Printf("[ChatStream] Error getting stream: %v", err)
		h.Error(w, http.StatusBadGateway, err.Error())
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("x-vercel-ai-ui-message-stream", "v1")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.Error(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}
	flusher.Flush()

	// Pass through SSE events from sandbox
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			log.Printf("[ChatStream] Client disconnected, stopping SSE stream")
			return
		case line, ok := <-sseCh:
			if !ok {
				return
			}
			if line.Done {
				continue
			}
			h.publishThreadUpdatedEvent(ctx, projectID, sessionID, line)
			h.observeThreadActivityEvent(ctx, projectID, sessionID, threadID, line)
			writeStreamEvent(w, line)
			flusher.Flush()
		}
	}
}

type streamThreadUpdateChunk struct {
	Type string `json:"type"`
	Data struct {
		Thread struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"thread"`
	} `json:"data"`
}

type streamCompletionStatusChunk struct {
	Type string `json:"type"`
	Data struct {
		ThreadID  string `json:"threadId"`
		IsRunning bool   `json:"isRunning"`
	} `json:"data"`
}

func (h *Handler) publishThreadUpdatedEvent(ctx context.Context, projectID, sessionID string, line service.SSELine) {
	if h.eventBroker == nil || line.Event != "chunk" || line.Data == "" {
		return
	}

	var chunk streamThreadUpdateChunk
	if err := json.Unmarshal([]byte(line.Data), &chunk); err != nil {
		return
	}
	if chunk.Type != "data-thread-update" {
		return
	}

	threadID := strings.TrimSpace(chunk.Data.Thread.ID)
	threadName := strings.TrimSpace(chunk.Data.Thread.Name)
	if threadID == "" || threadName == "" {
		return
	}

	if err := h.eventBroker.PublishThreadUpdated(
		ctx,
		projectID,
		sessionID,
		threadID,
		threadName,
	); err != nil {
		log.Printf("[ChatStream] Failed to publish thread update event: %v", err)
	}
}

func (h *Handler) observeThreadActivityEvent(ctx context.Context, projectID, sessionID, threadID string, line service.SSELine) {
	if h.sessionService == nil || line.Event != "chunk" || line.Data == "" {
		return
	}

	var chunk streamCompletionStatusChunk
	if err := json.Unmarshal([]byte(line.Data), &chunk); err != nil {
		return
	}
	if chunk.Type != "data-completion-status" {
		return
	}
	if observedThreadID := strings.TrimSpace(chunk.Data.ThreadID); observedThreadID != "" && observedThreadID != threadID {
		return
	}

	updateCtx := context.WithoutCancel(ctx)
	var err error
	if chunk.Data.IsRunning {
		err = h.sessionService.PromoteThreadStatus(updateCtx, projectID, sessionID, model.SessionActivityStatusRunning)
	} else {
		err = h.sessionService.RefreshThreadStatus(updateCtx, projectID, sessionID)
	}
	if err != nil {
		log.Printf("[ChatStream] Failed to persist thread activity for session %s: %v", sessionID, err)
	}
}

func writeStreamEvent(w http.ResponseWriter, line service.SSELine) {
	if line.ID != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", line.ID)
	}
	if line.Event != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", line.Event)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", line.Data)
}

// ChatQuestion returns the current pending AskUserQuestion for a session thread.
// GET /api/projects/{projectId}/sessions/{sessionId}/threads/{threadId}/question/{questionId}
// Returns { status: "pending", question: {...} } if that question is still waiting
// Returns { status: "answered", question: null } if already answered or unknown
func (h *Handler) ChatQuestion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID, threadID, _, ok := h.resolveSessionAndThread(w, r, projectID, false)
	if !ok {
		return
	}

	toolUseID := r.PathValue("questionId")

	result, err := h.chatService.GetQuestion(ctx, projectID, sessionID, threadID, toolUseID)
	if err != nil {
		log.Printf("[ChatQuestion] Error: %v", err)
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// ChatQuestionCommitPreview returns the parsed replay bundle preview for a pending
// request_commit_pull approval on a session thread.
// GET /api/projects/{projectId}/sessions/{sessionId}/threads/{threadId}/question/{questionId}/commit-preview
func (h *Handler) ChatQuestionCommitPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID, threadID, _, ok := h.resolveSessionAndThread(w, r, projectID, false)
	if !ok {
		return
	}

	questionID := r.PathValue("questionId")
	if questionID == "" {
		h.Error(w, http.StatusBadRequest, "questionId is required")
		return
	}

	result, err := h.chatService.GetRequestCommitPullPreview(ctx, projectID, sessionID, threadID, questionID)
	if err != nil {
		log.Printf("[ChatQuestionCommitPreview] Error: %v", err)
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// ChatAnswer submits answers to a pending AskUserQuestion for a session thread.
// POST /api/projects/{projectId}/sessions/{sessionId}/threads/{threadId}/answer/{questionId}
func (h *Handler) ChatAnswer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID, threadID, _, ok := h.resolveSessionAndThread(w, r, projectID, false)
	if !ok {
		return
	}

	questionID := r.PathValue("questionId")
	if questionID == "" {
		h.Error(w, http.StatusBadRequest, "questionId is required")
		return
	}

	var req sandboxapi.AnswerQuestionRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.ToolUseID = questionID

	if req.Answers == nil {
		h.Error(w, http.StatusBadRequest, "answers is required")
		return
	}

	pendingQuestion, err := h.chatService.GetQuestion(ctx, projectID, sessionID, threadID, questionID)
	if err != nil {
		log.Printf("[ChatAnswer] Error loading pending question: %v", err)
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if metadata, ok := approvedCommitPullMetadata(pendingQuestion, req.Answers); ok {
		if err := h.sessionService.CommitSession(ctx, projectID, sessionID, h.jobQueue, service.CommitSessionOptions{
			RequestedDirectory:  metadata.Directory,
			RequestedBaseCommit: metadata.BaseCommit,
			RequestedCommitHash: metadata.CommitHash,
			ApprovalThreadID:    threadID,
			ApprovalQuestionID:  questionID,
		}); err != nil {
			if errors.Is(err, service.ErrSessionOperationInProgress) {
				h.Error(w, http.StatusConflict, "Session operation already in progress")
				return
			}
			log.Printf("[ChatAnswer] Error starting commit pull: %v", err)
			h.Error(w, http.StatusInternalServerError, "Failed to initiate session commit")
			return
		}
		h.JSON(w, http.StatusOK, &sandboxapi.AnswerQuestionResponse{Success: true})
		return
	}

	result, err := h.chatService.AnswerQuestion(ctx, projectID, sessionID, threadID, &req)
	if err != nil {
		if errors.Is(err, service.ErrNoActiveCompletion) {
			h.Error(w, http.StatusNotFound, "no pending question for this toolUseID")
			return
		}
		log.Printf("[ChatAnswer] Error: %v", err)
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

func approvedCommitPullMetadata(question *sandboxapi.PendingQuestionResponse, answers map[string]string) (requestCommitPullMetadata, bool) {
	if question == nil || question.Status != "pending" || question.Question == nil {
		return requestCommitPullMetadata{}, false
	}
	if !strings.EqualFold(strings.TrimSpace(question.Question.Context), requestCommitPullApprovalContext) {
		return requestCommitPullMetadata{}, false
	}
	if strings.TrimSpace(answers[requestCommitPullRejectedKey]) != "" {
		return requestCommitPullMetadata{}, false
	}
	if strings.TrimSpace(answers[requestCommitPullApprovedKey]) == "" {
		return requestCommitPullMetadata{}, false
	}
	var metadata requestCommitPullMetadata
	if len(question.Question.Metadata) > 0 {
		_ = json.Unmarshal(question.Question.Metadata, &metadata)
	}
	return metadata, true
}

// ChatCancel handles cancelling an in-progress chat completion.
// POST /api/projects/{projectId}/sessions/{sessionId}/threads/{threadId}/cancel
func (h *Handler) ChatCancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID, threadID, _, ok := h.resolveSessionAndThread(w, r, projectID, false)
	if !ok {
		return
	}

	// Cancel the completion
	result, err := h.chatService.CancelCompletion(ctx, projectID, sessionID, threadID)
	if err != nil {
		if errors.Is(err, service.ErrNoActiveCompletion) {
			h.Error(w, http.StatusConflict, "no active completion to cancel")
			return
		}
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

func (h *Handler) resolveSessionAndThread(w http.ResponseWriter, r *http.Request, projectID string, noContentOnMissing bool) (sessionID, threadID string, session *model.Session, ok bool) {
	ctx := r.Context()
	sessionID = r.PathValue("sessionId")
	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return "", "", nil, false
	}
	threadID = r.PathValue("threadId")
	if threadID == "" {
		h.Error(w, http.StatusBadRequest, "threadId is required")
		return "", "", nil, false
	}

	session, err := h.chatService.GetSessionByID(ctx, sessionID)
	if err != nil {
		if noContentOnMissing {
			w.WriteHeader(http.StatusNoContent)
		} else {
			h.Error(w, http.StatusNotFound, "session not found")
		}
		return "", "", nil, false
	}
	if session.ProjectID != projectID {
		h.Error(w, http.StatusForbidden, "session does not belong to this project")
		return "", "", nil, false
	}

	return sessionID, threadID, session, true
}

// lastUserMessageID returns the ID of the last user message in the slice, or "".
func lastUserMessageID(messages []json.RawMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		var m struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		}
		if json.Unmarshal(messages[i], &m) == nil && m.Role == "user" && m.ID != "" {
			return m.ID
		}
	}
	return ""
}
