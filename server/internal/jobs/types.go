// Package jobs defines job types and payloads for background job processing.
package jobs

import "time"

// JobType represents the type of job.
type JobType string

const (
	JobTypeSessionInit          JobType = "session_init"
	JobTypeSessionDelete        JobType = "session_delete"
	JobTypeSessionSandboxDelete JobType = "session_sandbox_delete"
	JobTypeSessionCommit        JobType = "session_commit"
	JobTypePromptDispatch       JobType = "prompt_dispatch"
	JobTypeWorkspaceInit        JobType = "workspace_init"
	JobTypeWorkspaceDelete      JobType = "workspace_delete"
)

// JobPayload is implemented by all job payloads. The payload struct itself
// is JSON-marshaled as the job's Payload field.
type JobPayload interface {
	JobType() JobType
	ResourceKey() (resourceType string, resourceID string)
}

// Prioritized is an optional interface payloads can implement to override the default priority (10).
type Prioritized interface {
	Priority() int
}

// MaxAttempter is an optional interface payloads can implement to override the default max attempts.
type MaxAttempter interface {
	MaxAttempts() int
}

// DuplicateAllower is an optional interface payloads can implement to allow
// multiple pending/running jobs for the same resource. Jobs are still serialized
// at execution time (only one runs at a time per resource), but multiple can be queued.
type DuplicateAllower interface {
	AllowDuplicates() bool
}

// SessionInitPayload is the payload for session_init jobs.
type SessionInitPayload struct {
	ProjectID   string `json:"projectId"`
	SessionID   string `json:"sessionId"`
	WorkspaceID string `json:"workspaceId"`
}

func (p SessionInitPayload) JobType() JobType              { return JobTypeSessionInit }
func (p SessionInitPayload) ResourceKey() (string, string) { return ResourceTypeSession, p.SessionID }
func (p SessionInitPayload) MaxAttempts() int              { return 1 }

// WorkspaceInitPayload is the payload for workspace_init jobs.
type WorkspaceInitPayload struct {
	ProjectID   string `json:"projectId"`
	WorkspaceID string `json:"workspaceId"`
}

func (p WorkspaceInitPayload) JobType() JobType { return JobTypeWorkspaceInit }
func (p WorkspaceInitPayload) ResourceKey() (string, string) {
	return ResourceTypeWorkspace, p.WorkspaceID
}

// WorkspaceDeletePayload is the payload for workspace_delete jobs.
type WorkspaceDeletePayload struct {
	ProjectID   string `json:"projectId"`
	WorkspaceID string `json:"workspaceId"`
	DeleteFiles bool   `json:"deleteFiles"`
}

func (p WorkspaceDeletePayload) JobType() JobType { return JobTypeWorkspaceDelete }
func (p WorkspaceDeletePayload) ResourceKey() (string, string) {
	return ResourceTypeWorkspace, p.WorkspaceID
}
func (p WorkspaceDeletePayload) Priority() int         { return 5 }
func (p WorkspaceDeletePayload) AllowDuplicates() bool { return true }

// SessionDeletePayload is the payload for session_delete jobs.
type SessionDeletePayload struct {
	ProjectID    string `json:"projectId"`
	SessionID    string `json:"sessionId"`
	CreateFailed bool   `json:"createFailed,omitempty"`
}

func (p SessionDeletePayload) JobType() JobType              { return JobTypeSessionDelete }
func (p SessionDeletePayload) ResourceKey() (string, string) { return ResourceTypeSession, p.SessionID }
func (p SessionDeletePayload) Priority() int                 { return 5 }
func (p SessionDeletePayload) AllowDuplicates() bool         { return true }

// SessionSandboxDeletePayload is the payload for session_sandbox_delete jobs.
type SessionSandboxDeletePayload struct {
	SessionID string    `json:"sessionId"`
	DeleteAt  time.Time `json:"deleteAt"`
}

func (p SessionSandboxDeletePayload) JobType() JobType { return JobTypeSessionSandboxDelete }
func (p SessionSandboxDeletePayload) ResourceKey() (string, string) {
	return ResourceTypeRetainedSandbox, p.SessionID
}
func (p SessionSandboxDeletePayload) ScheduledAt() time.Time { return p.DeleteAt }
func (p SessionSandboxDeletePayload) Priority() int          { return 1 }

// SessionCommitPayload is the payload for session_commit jobs.
type SessionCommitPayload struct {
	ProjectID           string `json:"projectId"`
	SessionID           string `json:"sessionId"`
	WorkspaceID         string `json:"workspaceId"`
	RequestedDirectory  string `json:"requestedDirectory,omitempty"`
	RequestedBaseCommit string `json:"requestedBaseCommit,omitempty"`
	RequestedCommitHash string `json:"requestedCommitHash,omitempty"`
	ApprovalThreadID    string `json:"approvalThreadId,omitempty"`
	ApprovalQuestionID  string `json:"approvalQuestionId,omitempty"`
}

func (p SessionCommitPayload) JobType() JobType { return JobTypeSessionCommit }
func (p SessionCommitPayload) ResourceKey() (string, string) {
	return ResourceTypeWorkspace, p.WorkspaceID
}
func (p SessionCommitPayload) MaxAttempts() int      { return 1 }
func (p SessionCommitPayload) AllowDuplicates() bool { return true }

// PromptDispatchPayload is the payload for prompt_dispatch jobs.
type PromptDispatchPayload struct {
	ProjectID    string `json:"projectId"`
	SubmissionID string `json:"submissionId"`
}

func (p PromptDispatchPayload) JobType() JobType { return JobTypePromptDispatch }
func (p PromptDispatchPayload) ResourceKey() (string, string) {
	return ResourceTypePromptSubmission, p.SubmissionID
}
