package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	PromptSubmissionStatusPending     = "pending"
	PromptSubmissionStatusDispatching = "dispatching"
	PromptSubmissionStatusAccepted    = "accepted"
	PromptSubmissionStatusFailed      = "failed"
)

// PromptSubmission persists a prompt intent until the sandbox accepts it.
type PromptSubmission struct {
	ID                    string     `gorm:"primaryKey;type:text" json:"id"`
	ProjectID             string     `gorm:"column:project_id;not null;type:text;index" json:"projectId"`
	SessionID             string     `gorm:"column:session_id;not null;type:text;index;uniqueIndex:idx_prompt_submission_key" json:"sessionId"`
	ThreadID              string     `gorm:"column:thread_id;not null;type:text;uniqueIndex:idx_prompt_submission_key" json:"threadId"`
	ClientMessageID       string     `gorm:"column:client_message_id;not null;type:text;uniqueIndex:idx_prompt_submission_key" json:"clientMessageId"`
	MessageID             string     `gorm:"column:message_id;not null;type:text" json:"messageId"`
	MessagesEncryptedData []byte     `gorm:"column:messages_encrypted_data" json:"-"`
	Model                 string     `gorm:"type:text" json:"model,omitempty"`
	Reasoning             string     `gorm:"type:text" json:"reasoning,omitempty"`
	Mode                  string     `gorm:"type:text" json:"mode,omitempty"`
	RunAfter              string     `gorm:"column:run_after;type:text" json:"runAfter,omitempty"`
	Status                string     `gorm:"not null;type:text;default:pending;index" json:"status"`
	CompletionID          *string    `gorm:"column:completion_id;type:text" json:"completionId,omitempty"`
	QueuedPromptID        *string    `gorm:"column:queued_prompt_id;type:text" json:"queuedPromptId,omitempty"`
	ErrorMessage          *string    `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`
	AcceptedAt            *time.Time `gorm:"column:accepted_at" json:"acceptedAt,omitempty"`
	CreatedAt             time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt             time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (PromptSubmission) TableName() string { return "prompt_submissions" }

func (p *PromptSubmission) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}
