package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
)

type recordingJobEnqueuer struct {
	count int
}

func (e *recordingJobEnqueuer) Enqueue(context.Context, jobs.JobPayload) error {
	e.count++
	return nil
}

func TestSubmitPromptReturnsQueuedWhileDispatchContinues(t *testing.T) {
	st := setupTestStore(t)
	chatService := NewChatService(st, &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}, nil, nil, nil, nil, nil)

	messages := []json.RawMessage{
		json.RawMessage(`{"id":"msg-user-1","role":"user","parts":[{"type":"text","text":"hello"}]}`),
	}
	rawMessages, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("failed to marshal messages: %v", err)
	}
	encryptedMessages, err := chatService.encryptPromptPayload(rawMessages)
	if err != nil {
		t.Fatalf("failed to encrypt prompt payload: %v", err)
	}

	submission := &model.PromptSubmission{
		ProjectID:             "project-1",
		SessionID:             "session-1",
		ThreadID:              "thread-1",
		ClientMessageID:       "msg-user-1",
		MessageID:             "msg-user-1",
		MessagesEncryptedData: encryptedMessages,
		Status:                model.PromptSubmissionStatusDispatching,
	}
	if err := st.CreatePromptSubmission(context.Background(), submission); err != nil {
		t.Fatalf("failed to create prompt submission: %v", err)
	}

	latest, started, err := chatService.SubmitPrompt(
		context.Background(),
		"project-1",
		"session-1",
		"thread-1",
		messages,
		"",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("SubmitPrompt returned error: %v", err)
	}
	if latest == nil {
		t.Fatal("SubmitPrompt returned nil submission")
	}
	if latest.Status != model.PromptSubmissionStatusDispatching {
		t.Fatalf("expected dispatching submission status, got %q", latest.Status)
	}
	if started == nil {
		t.Fatal("SubmitPrompt returned nil started response")
	}
	if started.Status != "queued" {
		t.Fatalf("expected queued status, got %q", started.Status)
	}
}

func TestSubmitPromptQueuesWhileSessionInitializes(t *testing.T) {
	ctx := context.Background()
	st := setupTestStore(t)
	enqueuer := &recordingJobEnqueuer{}
	chatService := NewChatService(st, &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}, nil, enqueuer, nil, nil, nil)

	workspace := &model.Workspace{
		ID:         "workspace-initializing",
		ProjectID:  "project-1",
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:            "session-initializing",
		ProjectID:     "project-1",
		WorkspaceID:   workspace.ID,
		Name:          "Initializing Session",
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	messages := []json.RawMessage{
		json.RawMessage(`{"id":"msg-user-1","role":"user","parts":[{"type":"text","text":"hello"}]}`),
	}
	latest, started, err := chatService.SubmitPrompt(
		ctx,
		session.ProjectID,
		session.ID,
		session.ID,
		messages,
		"",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("SubmitPrompt returned error: %v", err)
	}
	if latest == nil {
		t.Fatal("SubmitPrompt returned nil submission")
	}
	if latest.Status != model.PromptSubmissionStatusPending {
		t.Fatalf("expected pending submission status, got %q", latest.Status)
	}
	if started == nil {
		t.Fatal("SubmitPrompt returned nil started response")
	}
	if started.Status != "queued" {
		t.Fatalf("expected queued status, got %q", started.Status)
	}
	if enqueuer.count != 1 {
		t.Fatalf("expected one queued dispatch job, got %d", enqueuer.count)
	}
}

func TestDispatchPromptSubmissionSkipsRemovingSession(t *testing.T) {
	ctx := context.Background()
	st := setupTestStore(t)
	chatService := NewChatService(st, &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}, nil, nil, nil, nil, nil)

	workspace := &model.Workspace{
		ID:         "workspace-removing",
		ProjectID:  "project-1",
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:            "session-removing",
		ProjectID:     "project-1",
		WorkspaceID:   workspace.ID,
		Name:          "Removing Session",
		SandboxStatus: model.SessionStatusRemoving,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	submission := &model.PromptSubmission{
		ProjectID:       "project-1",
		SessionID:       session.ID,
		ThreadID:        "thread-1",
		ClientMessageID: "msg-user-1",
		MessageID:       "msg-user-1",
		Status:          model.PromptSubmissionStatusPending,
	}
	if err := st.CreatePromptSubmission(ctx, submission); err != nil {
		t.Fatalf("failed to create prompt submission: %v", err)
	}

	if err := chatService.DispatchPromptSubmission(ctx, submission.ID); err != nil {
		t.Fatalf("DispatchPromptSubmission returned error: %v", err)
	}

	latest, err := st.GetPromptSubmissionByID(ctx, submission.ID)
	if err != nil {
		t.Fatalf("failed to load prompt submission: %v", err)
	}
	if latest.Status != model.PromptSubmissionStatusFailed {
		t.Fatalf("expected failed submission status, got %q", latest.Status)
	}
	if latest.ErrorMessage == nil || *latest.ErrorMessage != "session is removing" {
		t.Fatalf("expected removing session error, got %v", latest.ErrorMessage)
	}
}

func TestDispatchPromptSubmissionIgnoresDeletedSubmission(t *testing.T) {
	st := setupTestStore(t)
	chatService := NewChatService(st, &config.Config{
		EncryptionKey: []byte("test-key-32-bytes-long-123456789"),
	}, nil, nil, nil, nil, nil)

	if err := chatService.DispatchPromptSubmission(context.Background(), "missing-submission"); err != nil {
		t.Fatalf("DispatchPromptSubmission returned error for missing submission: %v", err)
	}
}
