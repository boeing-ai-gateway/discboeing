package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/model"
)

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
