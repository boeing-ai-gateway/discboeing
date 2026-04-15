package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/service"
)

// SessionCommitExecutor handles session_commit jobs.
type SessionCommitExecutor struct {
	sessionService *service.SessionService
}

// NewSessionCommitExecutor creates a new session commit executor.
func NewSessionCommitExecutor(sessionSvc *service.SessionService) *SessionCommitExecutor {
	return &SessionCommitExecutor{sessionService: sessionSvc}
}

// Type returns the job type this executor handles.
func (e *SessionCommitExecutor) Type() jobs.JobType {
	return jobs.JobTypeSessionCommit
}

// Execute processes the job.
func (e *SessionCommitExecutor) Execute(ctx context.Context, job *model.Job) error {
	if e.sessionService == nil {
		return fmt.Errorf("session service not available")
	}

	var payload jobs.SessionCommitPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if payload.SessionID == "" {
		return fmt.Errorf("sessionId is required")
	}
	if payload.ProjectID == "" {
		return fmt.Errorf("projectId is required")
	}

	err := e.sessionService.PerformCommit(ctx, payload.ProjectID, payload.SessionID, service.CommitSessionOptions{
		RequestedDirectory:  payload.RequestedDirectory,
		RequestedCommitHash: payload.RequestedCommitHash,
		ApprovalThreadID:    payload.ApprovalThreadID,
		ApprovalQuestionID:  payload.ApprovalQuestionID,
	})
	if payload.ApprovalThreadID != "" && payload.ApprovalQuestionID != "" {
		if answerErr := e.sessionService.FinalizeRequestCommitPullApproval(ctx, payload.SessionID, payload.ApprovalThreadID, payload.ApprovalQuestionID, err); answerErr != nil {
			if err != nil {
				return fmt.Errorf("perform commit: %w; finalize request commit pull approval: %v", err, answerErr)
			}
			return fmt.Errorf("finalize request commit pull approval: %w", answerErr)
		}
	}
	return err
}
