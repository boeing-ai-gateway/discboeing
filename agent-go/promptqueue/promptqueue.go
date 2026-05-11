package promptqueue

import (
	"time"

	"github.com/obot-platform/discobot/agent-go/message"
)

// Prompt stores one queued user submission for a thread.
type Prompt struct {
	ID        string            `json:"id"`
	CreatedAt time.Time         `json:"createdAt,omitzero"`
	RunAfter  time.Time         `json:"runAfter,omitzero"`
	Message   message.UIMessage `json:"message"`
	Model     string            `json:"model,omitempty"`
	Reasoning string            `json:"reasoning,omitempty"`
}

// Update describes editable fields for a queued prompt.
type Update struct {
	RunAfter      *time.Time
	ClearRunAfter bool
	Message       *message.UIMessage
	Position      *int
}

// FromMessage builds a queued prompt from a user message and chat options.
func FromMessage(userMessage message.UIMessage, model, reasoning string, runAfter time.Time) Prompt {
	queued := Prompt{
		Message: message.UIMessage{
			ID:       userMessage.ID,
			Role:     userMessage.Role,
			Parts:    append([]message.UIPart{}, userMessage.Parts...),
			Metadata: userMessage.Metadata,
		},
		Model:     model,
		Reasoning: reasoning,
	}
	if !runAfter.IsZero() {
		queued.RunAfter = runAfter.UTC()
	}
	return queued
}

// ToThreadUpdateInfo converts queued prompts into thread update summaries.
func ToThreadUpdateInfo(queue []Prompt) []message.ThreadQueuedPromptInfo {
	if len(queue) == 0 {
		return nil
	}
	items := make([]message.ThreadQueuedPromptInfo, 0, len(queue))
	for _, prompt := range queue {
		items = append(items, message.ThreadQueuedPromptInfo{
			ID:        prompt.ID,
			CreatedAt: prompt.CreatedAt,
			RunAfter:  prompt.RunAfter,
			Message:   prompt.Message,
			Model:     prompt.Model,
			Reasoning: prompt.Reasoning,
		})
	}
	return items
}
