package service

import (
	"strings"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
)

// SessionActivityStatus is the API shape for the session-level thread activity
// summary. The agent remains authoritative for per-thread details; the server
// stores only the last known aggregate status on the session row.
type SessionActivityStatus struct {
	Status                 string `json:"status"`
	Reason                 string `json:"reason,omitempty"`
	NeedsAttentionCount    int    `json:"needsAttentionCount,omitempty"`
	RunningCount           int    `json:"runningCount,omitempty"`
	QueuedCount            int    `json:"queuedCount,omitempty"`
	UnknownCount           int    `json:"unknownCount,omitempty"`
	RepresentativeThreadID string `json:"threadId,omitempty"`
	UpdatedAt              string `json:"updatedAt,omitempty"`
}

func normalizeSessionActivityStatus(status string) string {
	switch strings.TrimSpace(status) {
	case model.SessionActivityStatusQueued:
		return model.SessionActivityStatusQueued
	case model.SessionActivityStatusRunning:
		return model.SessionActivityStatusRunning
	case model.SessionActivityStatusNeedsAttention:
		return model.SessionActivityStatusNeedsAttention
	case model.SessionActivityStatusUnknown:
		return model.SessionActivityStatusUnknown
	default:
		return model.SessionActivityStatusIdle
	}
}

func sessionActivityPriority(status string) int {
	switch normalizeSessionActivityStatus(status) {
	case model.SessionActivityStatusNeedsAttention:
		return 4
	case model.SessionActivityStatusRunning:
		return 3
	case model.SessionActivityStatusQueued:
		return 2
	case model.SessionActivityStatusUnknown:
		return 1
	default:
		return 0
	}
}

func sessionActivityNonTerminalStatuses() []string {
	return []string{
		model.SessionActivityStatusQueued,
		model.SessionActivityStatusRunning,
		model.SessionActivityStatusUnknown,
	}
}

func isSessionActivityNonTerminal(status string) bool {
	switch normalizeSessionActivityStatus(status) {
	case model.SessionActivityStatusQueued, model.SessionActivityStatusRunning, model.SessionActivityStatusUnknown:
		return true
	default:
		return false
	}
}

func sessionActivityStatusFromStoredStatus(status string) *SessionActivityStatus {
	return &SessionActivityStatus{Status: normalizeSessionActivityStatus(status)}
}

func sessionActivityStatusFromSnapshot(snapshot *sandboxapi.SessionActivityResponse) *SessionActivityStatus {
	if snapshot == nil {
		return nil
	}
	return &SessionActivityStatus{
		Status:                 normalizeSessionActivityStatus(snapshot.Status),
		Reason:                 strings.TrimSpace(snapshot.Reason),
		NeedsAttentionCount:    snapshot.NeedsAttentionCount,
		RunningCount:           snapshot.RunningCount,
		QueuedCount:            snapshot.QueuedCount,
		UnknownCount:           snapshot.UnknownCount,
		RepresentativeThreadID: strings.TrimSpace(snapshot.RepresentativeThreadID),
	}
}

func sessionActivityStatusFromThreads(threads []sandboxapi.Thread) *SessionActivityStatus {
	status := &SessionActivityStatus{Status: model.SessionActivityStatusIdle}
	for _, thread := range threads {
		threadStatus, reason := threadActivitySummary(thread)
		if threadStatus == model.SessionActivityStatusIdle {
			continue
		}
		switch threadStatus {
		case model.SessionActivityStatusNeedsAttention:
			status.NeedsAttentionCount++
		case model.SessionActivityStatusRunning:
			status.RunningCount++
		case model.SessionActivityStatusQueued:
			status.QueuedCount++
		case model.SessionActivityStatusUnknown:
			status.UnknownCount++
		}
		if sessionActivityPriority(threadStatus) > sessionActivityPriority(status.Status) {
			status.Status = threadStatus
			status.Reason = reason
			status.RepresentativeThreadID = strings.TrimSpace(thread.ID)
		}
	}
	return status
}

func threadActivitySummary(thread sandboxapi.Thread) (string, string) {
	if thread.ActivityStatus != nil {
		status := normalizeSessionActivityStatus(thread.ActivityStatus.Status)
		if status != model.SessionActivityStatusIdle {
			return status, strings.TrimSpace(thread.ActivityStatus.Reason)
		}
	}
	if thread.PendingQuestion {
		return model.SessionActivityStatusNeedsAttention, model.SessionActivityReasonPendingQuestion
	}
	if strings.TrimSpace(thread.ErrorMessage) != "" {
		return model.SessionActivityStatusNeedsAttention, model.SessionActivityReasonThreadError
	}
	switch strings.TrimSpace(thread.State) {
	case model.SessionActivityReasonInterrupted:
		return model.SessionActivityStatusNeedsAttention, model.SessionActivityReasonInterrupted
	case model.SessionActivityReasonCancelled:
		return model.SessionActivityStatusNeedsAttention, model.SessionActivityReasonCancelled
	}
	if strings.TrimSpace(thread.ActiveCommand) != "" {
		return model.SessionActivityStatusRunning, model.SessionActivityReasonCompletion
	}
	if len(thread.PromptQueue) > 0 {
		return model.SessionActivityStatusQueued, model.SessionActivityReasonQueuedPrompt
	}
	return model.SessionActivityStatusIdle, ""
}
