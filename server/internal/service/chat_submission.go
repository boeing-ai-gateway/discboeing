package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
	"github.com/obot-platform/discobot/server/internal/store"
)

func (c *ChatService) SubmitPrompt(ctx context.Context, projectID, sessionID, threadID string, messages []json.RawMessage, requestModel, reasoning, runAfter string) (*model.PromptSubmission, *sandboxapi.ChatStartedResponse, error) {
	messageID := lastUserMessageID(messages)
	if messageID == "" {
		messageID = promptSubmissionFallbackKey(messages, requestModel, reasoning, runAfter)
	}

	rawMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal messages: %w", err)
	}
	encryptedMessages, err := c.encryptPromptPayload(rawMessages)
	if err != nil {
		return nil, nil, err
	}

	submission, err := c.getOrCreatePromptSubmission(ctx, &model.PromptSubmission{
		ProjectID:             projectID,
		SessionID:             sessionID,
		ThreadID:              threadID,
		ClientMessageID:       messageID,
		MessageID:             messageID,
		MessagesEncryptedData: encryptedMessages,
		Model:                 requestModel,
		Reasoning:             reasoning,
		RunAfter:              runAfter,
		Status:                model.PromptSubmissionStatusPending,
	})
	if err != nil {
		return nil, nil, err
	}

	if submission.Status == model.PromptSubmissionStatusAccepted {
		started := promptSubmissionStartedResponse(submission)
		c.updateThreadStatusAfterChatStart(ctx, projectID, sessionID, started)
		return submission, started, nil
	}
	if submission.Status == model.PromptSubmissionStatusFailed {
		if err := c.store.ReleasePromptSubmissionToPending(ctx, submission.ID, nil); err != nil {
			return nil, nil, err
		}
		submission, err = c.store.GetPromptSubmissionByID(ctx, submission.ID)
		if err != nil {
			return nil, nil, err
		}
	}

	c.promoteSessionThreadStatus(ctx, projectID, sessionID, model.SessionActivityStatusQueued)
	c.enqueuePromptDispatch(ctx, submission)
	if err := c.DispatchPromptSubmission(ctx, submission.ID); err != nil {
		latest, latestErr := c.store.GetPromptSubmissionByID(ctx, submission.ID)
		if latestErr == nil && latest.Status == model.PromptSubmissionStatusAccepted {
			started := promptSubmissionStartedResponse(latest)
			c.updateThreadStatusAfterChatStart(ctx, projectID, sessionID, started)
			return latest, started, nil
		}
		return latest, nil, err
	}

	latest, err := c.store.GetPromptSubmissionByID(ctx, submission.ID)
	if err != nil {
		return nil, nil, err
	}
	started := promptSubmissionStartedResponse(latest)
	if started == nil {
		switch latest.Status {
		case model.PromptSubmissionStatusPending, model.PromptSubmissionStatusDispatching:
			c.promoteSessionThreadStatus(ctx, projectID, sessionID, model.SessionActivityStatusQueued)
			return latest, &sandboxapi.ChatStartedResponse{Status: "queued"}, nil
		case model.PromptSubmissionStatusFailed:
			if latest.ErrorMessage != nil && *latest.ErrorMessage != "" {
				return latest, nil, errors.New(*latest.ErrorMessage)
			}
		}
		return latest, nil, fmt.Errorf("prompt dispatch did not reach sandbox")
	}
	c.updateThreadStatusAfterChatStart(ctx, projectID, sessionID, started)
	return latest, started, nil
}

func (c *ChatService) DispatchPromptSubmission(ctx context.Context, submissionID string) error {
	submission, err := c.store.GetPromptSubmissionByID(ctx, submissionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}

	switch submission.Status {
	case model.PromptSubmissionStatusAccepted, model.PromptSubmissionStatusFailed, model.PromptSubmissionStatusDispatching:
		return nil
	}

	if done, err := c.finishPromptDispatchForClosedSession(ctx, submission); done || err != nil {
		return err
	}

	claimed, err := c.store.ClaimPromptSubmissionForDispatch(ctx, submission.ID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	c.promoteSessionThreadStatus(ctx, submission.ProjectID, submission.SessionID, model.SessionActivityStatusQueued)

	prepared, err := c.prepareChatRequest(ctx, submission.ProjectID, submission.SessionID, submission.Model, submission.Reasoning)
	if err != nil {
		_ = c.store.ReleasePromptSubmissionToPending(ctx, submission.ID, stringPtr(err.Error()))
		return err
	}
	rawMessages, err := c.decryptPromptPayload(submission)
	if err != nil {
		_ = c.store.MarkPromptSubmissionFailed(ctx, submission.ID, err.Error())
		return err
	}

	opts := new(RequestOptions)
	if prepared.opts != nil {
		*opts = *prepared.opts
	}
	opts.RunAfter = submission.RunAfter
	started, err := prepared.client.StartChat(ctx, submission.ThreadID, rawMessages, prepared.modelID, opts)
	if err != nil {
		return c.handlePromptDispatchError(ctx, submission, err)
	}

	var completionID *string
	if started.CompletionID != "" {
		completionID = &started.CompletionID
	}
	var queuedPromptID *string
	if started.QueuedPromptID != "" {
		queuedPromptID = &started.QueuedPromptID
	}
	if err := c.store.MarkPromptSubmissionAccepted(ctx, submission.ID, completionID, queuedPromptID); err != nil {
		return err
	}
	c.updateThreadStatusAfterChatStart(ctx, submission.ProjectID, submission.SessionID, started)
	if err := c.sessionService.ClearTerminalCommitState(ctx, submission.ProjectID, submission.SessionID); err != nil {
		log.Printf("Warning: failed to clear terminal commit state for %s: %v", submission.SessionID, err)
	}
	return nil
}

func (c *ChatService) finishPromptDispatchForClosedSession(ctx context.Context, submission *model.PromptSubmission) (bool, error) {
	sess, err := c.store.GetSessionByID(ctx, submission.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return true, c.store.MarkPromptSubmissionFailed(ctx, submission.ID, "session no longer exists")
		}
		return false, err
	}
	switch sess.Status {
	case model.SessionStatusRemoving, model.SessionStatusRemoved:
		return true, c.store.MarkPromptSubmissionFailed(ctx, submission.ID, fmt.Sprintf("session is %s", sess.Status))
	default:
		return false, nil
	}
}

func (c *ChatService) ReconcilePromptSubmissions(ctx context.Context) error {
	submissions, err := c.store.ListPromptSubmissionsByStatuses(ctx,
		model.PromptSubmissionStatusPending,
		model.PromptSubmissionStatusDispatching,
	)
	if err != nil {
		return err
	}
	for _, submission := range submissions {
		if submission.Status == model.PromptSubmissionStatusDispatching {
			if err := c.store.ReleasePromptSubmissionToPending(ctx, submission.ID, submission.ErrorMessage); err != nil {
				return err
			}
		}
		c.enqueuePromptDispatch(ctx, submission)
	}
	return nil
}

func (c *ChatService) enqueuePromptDispatch(ctx context.Context, submission *model.PromptSubmission) {
	if c.jobEnqueuer == nil || submission == nil {
		return
	}
	err := c.jobEnqueuer.Enqueue(ctx, jobs.PromptDispatchPayload{
		ProjectID:    submission.ProjectID,
		SubmissionID: submission.ID,
	})
	if err != nil && !errors.Is(err, jobs.ErrJobAlreadyExists) {
		log.Printf("Warning: failed to enqueue prompt dispatch for submission %s: %v", submission.ID, err)
	}
}

func (c *ChatService) getOrCreatePromptSubmission(ctx context.Context, submission *model.PromptSubmission) (*model.PromptSubmission, error) {
	existing, err := c.store.GetPromptSubmissionByMessageID(ctx, submission.SessionID, submission.ThreadID, submission.ClientMessageID)
	if err == nil {
		if existing.Status == model.PromptSubmissionStatusAccepted {
			return existing, nil
		}
		existingPayload, decryptErr := c.decryptPromptPayload(existing)
		if decryptErr != nil {
			return nil, decryptErr
		}
		submissionPayload, decryptErr := c.decryptPromptPayload(submission)
		if decryptErr != nil {
			return nil, decryptErr
		}
		if existing.Model != submission.Model || existing.Reasoning != submission.Reasoning || existing.RunAfter != submission.RunAfter || !bytes.Equal(existingPayload, submissionPayload) {
			return nil, fmt.Errorf("prompt submission key reused with different payload")
		}
		return existing, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if err := c.store.CreatePromptSubmission(ctx, submission); err != nil {
		existing, getErr := c.store.GetPromptSubmissionByMessageID(ctx, submission.SessionID, submission.ThreadID, submission.ClientMessageID)
		if getErr == nil {
			return existing, nil
		}
		return nil, err
	}
	return submission, nil
}

func (c *ChatService) encryptPromptPayload(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if c.encryptor == nil {
		return nil, fmt.Errorf("prompt submission encryptor not configured")
	}
	encrypted, err := c.encryptor.Encrypt(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt prompt submission payload: %w", err)
	}
	return encrypted, nil
}

func (c *ChatService) decryptPromptPayload(submission *model.PromptSubmission) (json.RawMessage, error) {
	if submission == nil || len(submission.MessagesEncryptedData) == 0 {
		return nil, fmt.Errorf("prompt submission payload unavailable")
	}
	if c.encryptor == nil {
		return nil, fmt.Errorf("prompt submission encryptor not configured")
	}
	raw, err := c.encryptor.Decrypt(submission.MessagesEncryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt prompt submission payload: %w", err)
	}
	return json.RawMessage(raw), nil
}

func (c *ChatService) handlePromptDispatchError(ctx context.Context, submission *model.PromptSubmission, err error) error {
	var startErr *SandboxChatStartError
	if errors.As(err, &startErr) {
		switch startErr.ErrorCode {
		case "completion_in_progress":
			var completionID *string
			if startErr.CompletionID != "" {
				completionID = &startErr.CompletionID
			}
			if markErr := c.store.MarkPromptSubmissionAccepted(ctx, submission.ID, completionID, nil); markErr != nil {
				return markErr
			}
			c.promoteSessionThreadStatus(ctx, submission.ProjectID, submission.SessionID, model.SessionActivityStatusRunning)
			return nil
		case "pending_question_requires_answer":
			if markErr := c.store.MarkPromptSubmissionFailed(ctx, submission.ID, err.Error()); markErr != nil {
				return markErr
			}
			c.promoteSessionThreadStatus(ctx, submission.ProjectID, submission.SessionID, model.SessionActivityStatusNeedsAttention)
			return err
		default:
			if startErr.StatusCode >= 400 && startErr.StatusCode < 500 {
				if markErr := c.store.MarkPromptSubmissionFailed(ctx, submission.ID, err.Error()); markErr != nil {
					return markErr
				}
				return err
			}
		}
	}

	if releaseErr := c.store.ReleasePromptSubmissionToPending(ctx, submission.ID, stringPtr(err.Error())); releaseErr != nil {
		return releaseErr
	}
	c.promoteSessionThreadStatus(ctx, submission.ProjectID, submission.SessionID, model.SessionActivityStatusQueued)
	return err
}

func promptSubmissionStartedResponse(submission *model.PromptSubmission) *sandboxapi.ChatStartedResponse {
	if submission == nil || submission.Status != model.PromptSubmissionStatusAccepted {
		return nil
	}
	response := &sandboxapi.ChatStartedResponse{Status: "started"}
	if submission.CompletionID != nil {
		response.CompletionID = *submission.CompletionID
	}
	if submission.QueuedPromptID != nil {
		response.QueuedPromptID = *submission.QueuedPromptID
		response.Status = "queued"
	}
	return response
}

func promptSubmissionFallbackKey(messages []json.RawMessage, requestModel, reasoning, runAfter string) string {
	hash := sha256.New()
	for _, msg := range messages {
		_, _ = hash.Write(msg)
	}
	_, _ = hash.Write([]byte("\x00" + requestModel + "\x00" + reasoning + "\x00" + runAfter))
	return "generated-" + hex.EncodeToString(hash.Sum(nil))
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
