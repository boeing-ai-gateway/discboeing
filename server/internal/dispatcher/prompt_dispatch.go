package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/boeing-ai-gateway/discboeing/server/internal/jobs"
	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
)

// PromptDispatchExecutor handles prompt_dispatch jobs.
type PromptDispatchExecutor struct {
	chatService *service.ChatService
}

// NewPromptDispatchExecutor creates a new prompt dispatch executor.
func NewPromptDispatchExecutor(chatSvc *service.ChatService) *PromptDispatchExecutor {
	return &PromptDispatchExecutor{chatService: chatSvc}
}

// Type returns the job type this executor handles.
func (e *PromptDispatchExecutor) Type() jobs.JobType {
	return jobs.JobTypePromptDispatch
}

// Execute processes the job.
func (e *PromptDispatchExecutor) Execute(ctx context.Context, job *model.Job) error {
	if e.chatService == nil {
		return fmt.Errorf("chat service not available")
	}

	var payload jobs.PromptDispatchPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	if payload.SubmissionID == "" {
		return fmt.Errorf("submissionId is required")
	}

	return e.chatService.DispatchPromptSubmission(ctx, payload.SubmissionID)
}
