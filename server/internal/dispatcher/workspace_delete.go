package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/boeing-ai-gateway/discboeing/server/internal/jobs"
	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
)

// WorkspaceDeleteExecutor handles workspace_delete jobs.
type WorkspaceDeleteExecutor struct {
	workspaceService *service.WorkspaceService
}

// NewWorkspaceDeleteExecutor creates a new workspace delete executor.
func NewWorkspaceDeleteExecutor(workspaceSvc *service.WorkspaceService) *WorkspaceDeleteExecutor {
	return &WorkspaceDeleteExecutor{workspaceService: workspaceSvc}
}

// Type returns the job type this executor handles.
func (e *WorkspaceDeleteExecutor) Type() jobs.JobType {
	return jobs.JobTypeWorkspaceDelete
}

// Execute processes the job.
func (e *WorkspaceDeleteExecutor) Execute(ctx context.Context, job *model.Job) error {
	if e.workspaceService == nil {
		return fmt.Errorf("workspace service not available")
	}

	var payload jobs.WorkspaceDeletePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if payload.WorkspaceID == "" {
		return fmt.Errorf("workspaceId is required")
	}

	return e.workspaceService.PerformDeletion(ctx, payload.WorkspaceID, payload.DeleteFiles)
}
