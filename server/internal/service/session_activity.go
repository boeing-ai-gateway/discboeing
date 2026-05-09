package service

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/obot-platform/discobot/server/internal/events"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
	"github.com/obot-platform/discobot/server/internal/store"
)

// SessionActivityStatus is the API shape for aggregated non-idle thread state.
type SessionActivityStatus struct {
	Status                 string `json:"status"`
	Reason                 string `json:"reason,omitempty"`
	NeedsAttentionCount    int    `json:"needsAttentionCount"`
	RunningCount           int    `json:"runningCount"`
	QueuedCount            int    `json:"queuedCount"`
	UnknownCount           int    `json:"unknownCount"`
	RepresentativeThreadID string `json:"threadId,omitempty"`
	UpdatedAt              string `json:"updatedAt,omitempty"`
}

// SessionActivityService maintains sparse non-idle thread state and the derived
// per-session aggregate. It intentionally deletes idle thread rows so large
// sessions with many historical idle threads remain cheap to monitor.
type SessionActivityService struct {
	store       *store.Store
	eventBroker *events.Broker
}

func NewSessionActivityService(s *store.Store, eventBroker *events.Broker) *SessionActivityService {
	return &SessionActivityService{store: s, eventBroker: eventBroker}
}

func (s *SessionActivityService) MarkRunning(ctx context.Context, projectID, sessionID, threadID, completionID string) {
	if s == nil || strings.TrimSpace(threadID) == "" {
		return
	}
	state := &model.SessionThreadState{
		ProjectID:    projectID,
		Status:       model.SessionActivityStatusRunning,
		Reason:       model.SessionActivityReasonCompletion,
		CompletionID: stringPtrOrNil(completionID),
	}
	s.update(ctx, sessionID, threadID, state)
}

func (s *SessionActivityService) MarkQueued(ctx context.Context, projectID, sessionID, threadID string) {
	if s == nil || strings.TrimSpace(threadID) == "" {
		return
	}
	state := &model.SessionThreadState{
		ProjectID:  projectID,
		Status:     model.SessionActivityStatusQueued,
		Reason:     model.SessionActivityReasonQueuedPrompt,
		QueueCount: 1,
	}
	s.update(ctx, sessionID, threadID, state)
}

func (s *SessionActivityService) MarkNeedsAttention(ctx context.Context, projectID, sessionID, threadID, reason, message string) {
	if s == nil || strings.TrimSpace(threadID) == "" {
		return
	}
	if strings.TrimSpace(reason) == "" {
		reason = model.SessionActivityReasonPendingQuestion
	}
	state := &model.SessionThreadState{
		ProjectID: projectID,
		Status:    model.SessionActivityStatusNeedsAttention,
		Reason:    reason,
		Message:   message,
	}
	s.update(ctx, sessionID, threadID, state)
}

func (s *SessionActivityService) MarkIdle(ctx context.Context, sessionID, threadID string) {
	if s == nil || strings.TrimSpace(threadID) == "" {
		return
	}
	s.update(ctx, sessionID, threadID, nil)
}

func (s *SessionActivityService) MarkThreadDeleted(ctx context.Context, sessionID, threadID string) {
	s.MarkIdle(ctx, sessionID, threadID)
}

func (s *SessionActivityService) MarkSandboxStopped(ctx context.Context, sessionID string) {
	if s == nil {
		return
	}
	states, err := s.store.ListSessionThreadStates(ctx, sessionID)
	if err != nil {
		log.Printf("SessionActivityService: failed to load thread states for stopped session %s: %v", sessionID, err)
		return
	}
	for _, state := range states {
		if state.Status != model.SessionActivityStatusRunning {
			continue
		}
		state.Status = model.SessionActivityStatusNeedsAttention
		state.Reason = model.SessionActivityReasonSandboxStoppedDuringRun
		state.CompletionID = nil
		s.update(ctx, sessionID, state.ThreadID, state)
	}
}

func (s *SessionActivityService) ApplyThreadSnapshot(ctx context.Context, projectID, sessionID string, thread sandboxapi.Thread) {
	if s == nil || strings.TrimSpace(thread.ID) == "" {
		return
	}
	if thread.PendingQuestion {
		s.MarkNeedsAttention(ctx, projectID, sessionID, thread.ID, model.SessionActivityReasonPendingQuestion, "")
		return
	}
	if strings.TrimSpace(thread.ErrorMessage) != "" {
		s.MarkNeedsAttention(ctx, projectID, sessionID, thread.ID, model.SessionActivityReasonThreadError, thread.ErrorMessage)
		return
	}
	switch strings.TrimSpace(thread.State) {
	case "interrupted":
		s.MarkNeedsAttention(ctx, projectID, sessionID, thread.ID, model.SessionActivityReasonInterrupted, "")
		return
	case "cancelled":
		s.MarkNeedsAttention(ctx, projectID, sessionID, thread.ID, model.SessionActivityReasonCancelled, "")
		return
	}
	if len(thread.PromptQueue) > 0 {
		state := &model.SessionThreadState{
			ProjectID:  projectID,
			Status:     model.SessionActivityStatusQueued,
			Reason:     model.SessionActivityReasonQueuedPrompt,
			QueueCount: len(thread.PromptQueue),
		}
		for _, queued := range thread.PromptQueue {
			if queued.RunAfter.IsZero() {
				continue
			}
			if state.NextRunAfter == nil || queued.RunAfter.Before(*state.NextRunAfter) {
				runAfter := queued.RunAfter
				state.NextRunAfter = &runAfter
			}
		}
		s.update(ctx, sessionID, thread.ID, state)
		return
	}
	s.MarkIdle(ctx, sessionID, thread.ID)
}

func (s *SessionActivityService) ApplySessionSnapshot(ctx context.Context, projectID, sessionID string, snapshot *sandboxapi.SessionActivityResponse) {
	if s == nil || snapshot == nil {
		return
	}
	seen := make(map[string]struct{}, len(snapshot.Threads))
	for _, thread := range snapshot.Threads {
		threadID := strings.TrimSpace(thread.ThreadID)
		if threadID == "" {
			continue
		}
		seen[threadID] = struct{}{}
		state := &model.SessionThreadState{
			ProjectID:    projectID,
			Status:       strings.TrimSpace(thread.Status),
			Reason:       strings.TrimSpace(thread.Reason),
			CompletionID: thread.CompletionID,
			QueueCount:   thread.QueueCount,
			NextRunAfter: thread.NextRunAfter,
			Message:      thread.Message,
		}
		if state.Status == "" || state.Status == model.SessionActivityStatusIdle {
			s.MarkIdle(ctx, sessionID, threadID)
			continue
		}
		s.update(ctx, sessionID, threadID, state)
	}

	existing, err := s.store.ListSessionThreadStates(ctx, sessionID)
	if err != nil {
		log.Printf("SessionActivityService: failed to load existing activity states for %s: %v", sessionID, err)
		return
	}
	for _, state := range existing {
		if _, ok := seen[state.ThreadID]; ok {
			continue
		}
		s.MarkIdle(ctx, sessionID, state.ThreadID)
	}
}

func (s *SessionActivityService) IsSessionExecuting(ctx context.Context, sessionID string) bool {
	if s == nil {
		return false
	}
	status, err := s.store.GetSessionActivityStatus(ctx, sessionID)
	if err != nil {
		return false
	}
	return status.Status == model.SessionActivityStatusRunning || status.Status == model.SessionActivityStatusQueued || status.Status == model.SessionActivityStatusUnknown
}

func (s *SessionActivityService) ToAPI(status *model.SessionActivityStatus) *SessionActivityStatus {
	if status == nil {
		return &SessionActivityStatus{Status: model.SessionActivityStatusIdle}
	}
	api := &SessionActivityStatus{
		Status:              status.Status,
		Reason:              status.Reason,
		NeedsAttentionCount: status.NeedsAttentionCount,
		RunningCount:        status.RunningCount,
		QueuedCount:         status.QueuedCount,
		UnknownCount:        status.UnknownCount,
	}
	if status.RepresentativeThreadID != nil {
		api.RepresentativeThreadID = *status.RepresentativeThreadID
	}
	if !status.UpdatedAt.IsZero() {
		api.UpdatedAt = status.UpdatedAt.Format(time.RFC3339)
	}
	return api
}

func (s *SessionActivityService) update(ctx context.Context, sessionID, threadID string, state *model.SessionThreadState) {
	aggregate, changed, err := s.store.UpdateSessionThreadActivity(ctx, sessionID, threadID, state)
	if err != nil {
		log.Printf("SessionActivityService: failed to update %s/%s: %v", sessionID, threadID, err)
		return
	}
	if !changed || s.eventBroker == nil || aggregate == nil {
		return
	}
	threadStatus := s.ToAPI(aggregate)
	eventStatus := &events.SessionActivityStatusData{
		Status:                 threadStatus.Status,
		Reason:                 threadStatus.Reason,
		NeedsAttentionCount:    threadStatus.NeedsAttentionCount,
		RunningCount:           threadStatus.RunningCount,
		QueuedCount:            threadStatus.QueuedCount,
		UnknownCount:           threadStatus.UnknownCount,
		RepresentativeThreadID: threadStatus.RepresentativeThreadID,
		UpdatedAt:              threadStatus.UpdatedAt,
	}
	if err := s.eventBroker.PublishSessionActivityUpdated(ctx, aggregate.ProjectID, sessionID, eventStatus); err != nil {
		log.Printf("SessionActivityService: failed to publish session activity update: %v", err)
	}
}

func stringPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
