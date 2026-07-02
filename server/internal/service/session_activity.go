package service

import (
	"strings"

	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/sandboxapi"
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
