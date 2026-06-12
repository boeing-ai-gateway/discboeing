package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/events"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
	"github.com/obot-platform/discobot/server/internal/store"
)

const defaultThreadStatusSyncInterval = 10 * time.Second

// SessionThreadStatusSyncer refreshes persisted session-level thread summaries
// while sandboxes are running. Sandbox watch replay starts an agent activity
// stream for each running sandbox; the stream sends an initial snapshot and then
// change-driven snapshots. The poll loop remains as a recovery fallback for
// non-terminal summaries.
type SessionThreadStatusSyncer struct {
	store         *store.Store
	sandboxSvc    *SandboxService
	eventBroker   *events.Broker
	logger        *slog.Logger
	checkInterval time.Duration

	mu           sync.Mutex
	running      bool
	stopping     bool
	streams      map[string]*sessionActivityStream
	threadStates map[string]map[string]sessionThreadActivitySnapshot
	stopChan     chan struct{}
	wg           sync.WaitGroup
	shutdownOnce sync.Once
}

type sessionActivityStream struct {
	cancel context.CancelFunc
}

type sessionThreadActivitySnapshot struct {
	Status       string
	Reason       string
	CompletionID string
	QueueCount   int
	NextRunAfter string
	Message      string
}

// NewSessionThreadStatusSyncer creates a background syncer for session thread
// summaries observed from running sandboxes.
func NewSessionThreadStatusSyncer(
	store *store.Store,
	sandboxSvc *SandboxService,
	eventBroker *events.Broker,
	logger *slog.Logger,
	checkInterval time.Duration,
) *SessionThreadStatusSyncer {
	if logger == nil {
		logger = slog.Default()
	}
	if checkInterval <= 0 {
		checkInterval = defaultThreadStatusSyncInterval
	}
	return &SessionThreadStatusSyncer{
		store:         store,
		sandboxSvc:    sandboxSvc,
		eventBroker:   eventBroker,
		logger:        logger.With("component", "session_thread_status_syncer"),
		checkInterval: checkInterval,
		streams:       make(map[string]*sessionActivityStream),
		threadStates:  make(map[string]map[string]sessionThreadActivitySnapshot),
		stopChan:      make(chan struct{}),
	}
}

// Start begins the background sync loop.
func (s *SessionThreadStatusSyncer) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.syncLoop(ctx)
	if s.sandboxSvc != nil {
		s.wg.Add(1)
		go s.watchSandboxEvents(ctx)
	}

	s.logger.Info("session thread status syncer started", "check_interval", s.checkInterval)
}

// Shutdown gracefully stops the background sync loop.
func (s *SessionThreadStatusSyncer) Shutdown(ctx context.Context) error {
	var err error
	s.shutdownOnce.Do(func() {
		s.logger.Info("shutting down session thread status syncer")
		s.mu.Lock()
		s.stopping = true
		s.mu.Unlock()
		close(s.stopChan)
		s.stopAllSessionActivityStreams()

		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			s.logger.Info("session thread status syncer shutdown complete")
		case <-ctx.Done():
			err = fmt.Errorf("shutdown timeout exceeded")
			s.logger.Error("session thread status syncer shutdown timeout")
		}
	})
	return err
}

func (s *SessionThreadStatusSyncer) syncLoop(ctx context.Context) {
	defer s.wg.Done()

	if err := s.syncNonTerminalSessions(ctx); err != nil && ctx.Err() == nil {
		s.logger.Error("error syncing session thread statuses", "error", err)
	}

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("sync loop stopped: context cancelled")
			return
		case <-s.stopChan:
			s.logger.Info("sync loop stopped: shutdown signal")
			return
		case <-ticker.C:
			if err := s.syncNonTerminalSessions(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				s.logger.Error("error syncing session thread statuses", "error", err)
			}
		}
	}
}

func (s *SessionThreadStatusSyncer) watchSandboxEvents(ctx context.Context) {
	defer s.wg.Done()

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if s.sandboxSvc == nil {
		return
	}

	eventCh, err := s.sandboxSvc.WatchSandboxEvents(watchCtx)
	if err != nil {
		s.logger.Warn("failed to watch sandbox events for thread status sync", "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("sandbox event sync stopped: context cancelled")
			return
		case <-s.stopChan:
			s.logger.Info("sandbox event sync stopped: shutdown signal")
			return
		case event, ok := <-eventCh:
			if !ok {
				s.logger.Info("sandbox event sync stopped: event channel closed")
				return
			}
			if event.Status == sandbox.StatusRunning {
				s.startSessionActivityStream(ctx, event.SessionID)
			} else {
				s.stopSessionActivityStream(event.SessionID)
			}
		}
	}
}

func (s *SessionThreadStatusSyncer) syncNonTerminalSessions(ctx context.Context) error {
	if s == nil || s.store == nil || s.sandboxSvc == nil {
		return nil
	}

	sessions, err := s.store.ListSessionsByStatusesAndThreadStatuses(ctx,
		[]string{model.SessionStatusReady, legacySessionStatusRunning},
		sessionActivityNonTerminalStatuses(),
	)
	if err != nil {
		return fmt.Errorf("failed to list non-terminal thread-status sessions: %w", err)
	}
	if len(sessions) == 0 {
		return nil
	}

	s.logger.Debug("syncing session thread statuses", "count", len(sessions))
	for _, session := range sessions {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if session == nil || !isSessionActivityNonTerminal(session.ThreadStatus) {
			continue
		}

		logger := s.logger.With(
			"session_id", session.ID,
			"project_id", session.ProjectID,
			"thread_status", session.ThreadStatus,
		)
		if err := s.syncRunningSession(ctx, session.ID); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, sandbox.ErrNotRunning) || errors.Is(err, store.ErrNotFound) {
				logger.Debug("skipping thread status sync", "error", err)
				continue
			}
			logger.Warn("failed to refresh session thread status", "error", err)
			continue
		}
	}

	return nil
}

func (s *SessionThreadStatusSyncer) syncRunningSession(ctx context.Context, sessionID string) error {
	if s == nil || s.store == nil || s.sandboxSvc == nil || sessionID == "" {
		return nil
	}
	session, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	observedAt := time.Now().UTC()
	refreshCtx, cancel := context.WithTimeout(ctx, threadStatusRefreshTimeout)
	defer cancel()
	snapshot, err := s.sandboxSvc.GetSessionActivityIfRunning(refreshCtx, sessionID)
	if err != nil {
		return err
	}
	return s.applyActivitySnapshot(ctx, session.ProjectID, session.ID, snapshot, observedAt)
}

func (s *SessionThreadStatusSyncer) startSessionActivityStream(ctx context.Context, sessionID string) {
	if s == nil || s.sandboxSvc == nil || sessionID == "" {
		return
	}

	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return
	}
	if _, ok := s.streams[sessionID]; ok {
		s.mu.Unlock()
		return
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream := &sessionActivityStream{cancel: cancel}
	s.streams[sessionID] = stream
	s.wg.Add(1)
	s.mu.Unlock()

	go s.streamSessionActivity(streamCtx, sessionID, stream)
}

func (s *SessionThreadStatusSyncer) stopSessionActivityStream(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}

	s.mu.Lock()
	stream := s.streams[sessionID]
	delete(s.streams, sessionID)
	delete(s.threadStates, sessionID)
	s.mu.Unlock()

	if stream != nil {
		stream.cancel()
	}
}

func (s *SessionThreadStatusSyncer) stopAllSessionActivityStreams() {
	if s == nil {
		return
	}

	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.streams))
	for sessionID, cancel := range s.streams {
		cancels = append(cancels, cancel.cancel)
		delete(s.streams, sessionID)
		delete(s.threadStates, sessionID)
	}
	s.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (s *SessionThreadStatusSyncer) clearSessionActivityStream(sessionID string, stream *sessionActivityStream) {
	s.mu.Lock()
	if current := s.streams[sessionID]; current == stream {
		delete(s.streams, sessionID)
	}
	s.mu.Unlock()
}

func (s *SessionThreadStatusSyncer) streamSessionActivity(ctx context.Context, sessionID string, stream *sessionActivityStream) {
	defer s.wg.Done()
	defer s.clearSessionActivityStream(sessionID, stream)

	session, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.logger.Warn("failed to load session for activity stream",
				"session_id", sessionID,
				"error", err,
			)
		}
		return
	}
	projectID := session.ProjectID
	retryDelay := min(s.checkInterval, 5*time.Second)
	if retryDelay <= 0 {
		retryDelay = 5 * time.Second
	}
	logger := s.logger.With("session_id", sessionID, "project_id", projectID)

	for ctx.Err() == nil {
		activityCh, err := s.sandboxSvc.StreamSessionActivityIfRunning(ctx, sessionID)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, sandbox.ErrNotRunning) || errors.Is(err, store.ErrNotFound) {
				logger.Debug("activity stream stopped: sandbox not running", "error", err)
				return
			}
			logger.Warn("failed to open activity stream; falling back to snapshot",
				"error", err,
			)
			if syncErr := s.syncRunningSession(ctx, sessionID); syncErr != nil && ctx.Err() == nil &&
				!errors.Is(syncErr, sandbox.ErrNotRunning) && !errors.Is(syncErr, store.ErrNotFound) {
				logger.Warn("fallback activity snapshot failed", "error", syncErr)
			}
			if !s.waitForStreamRetry(ctx, retryDelay) {
				return
			}
			continue
		}

		logger.Debug("activity stream connected")
		for snapshot := range activityCh {
			if snapshot == nil {
				continue
			}
			if err := s.applyActivitySnapshot(ctx, projectID, sessionID, snapshot, time.Now().UTC()); err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Warn("failed to apply activity stream snapshot", "error", err)
			}
		}
		if ctx.Err() != nil {
			return
		}
		logger.Debug("activity stream closed; reconnecting")
		if !s.waitForStreamRetry(ctx, retryDelay) {
			return
		}
	}
}

func (s *SessionThreadStatusSyncer) applyActivitySnapshot(ctx context.Context, projectID, sessionID string, snapshot *sandboxapi.SessionActivityResponse, observedAt time.Time) error {
	if s == nil || s.store == nil {
		return nil
	}
	activity := sessionActivityStatusFromSnapshot(snapshot)
	if activity == nil {
		return nil
	}
	status := normalizeSessionActivityStatus(activity.Status)
	var (
		changed bool
		err     error
	)
	if observedAt.IsZero() {
		changed, err = s.store.UpdateSessionThreadStatus(ctx, sessionID, status)
	} else {
		changed, err = s.store.UpdateSessionThreadStatusIfUnchangedSince(ctx, sessionID, status, observedAt)
	}
	if err != nil {
		return fmt.Errorf("failed to update session thread status: %w", err)
	}
	accepted, err := s.activitySnapshotAccepted(ctx, sessionID, observedAt, changed)
	if err != nil {
		return err
	}
	threadActivityChanged := false
	if accepted {
		threadActivityChanged = s.noteSessionThreadActivitySnapshot(sessionID, snapshot)
	}
	s.publishSessionThreadStatusChanged(ctx, projectID, sessionID, changed, threadActivityChanged)
	return nil
}

func (s *SessionThreadStatusSyncer) activitySnapshotAccepted(ctx context.Context, sessionID string, observedAt time.Time, changed bool) (bool, error) {
	if changed || observedAt.IsZero() {
		return true, nil
	}
	session, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check session thread snapshot freshness: %w", err)
	}
	return !session.UpdatedAt.After(observedAt), nil
}

func (s *SessionThreadStatusSyncer) noteSessionThreadActivitySnapshot(sessionID string, snapshot *sandboxapi.SessionActivityResponse) bool {
	if s == nil || sessionID == "" || snapshot == nil {
		return false
	}
	next := sessionThreadActivitySnapshotFromResponse(snapshot)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.threadStates == nil {
		s.threadStates = make(map[string]map[string]sessionThreadActivitySnapshot)
	}
	previous, ok := s.threadStates[sessionID]
	s.threadStates[sessionID] = next
	if !ok {
		return len(next) > 0
	}
	return !maps.Equal(previous, next)
}

func sessionThreadActivitySnapshotFromResponse(snapshot *sandboxapi.SessionActivityResponse) map[string]sessionThreadActivitySnapshot {
	if snapshot == nil || len(snapshot.Threads) == 0 {
		return map[string]sessionThreadActivitySnapshot{}
	}
	threads := make(map[string]sessionThreadActivitySnapshot, len(snapshot.Threads))
	for _, thread := range snapshot.Threads {
		if thread.ThreadID == "" {
			continue
		}
		state := sessionThreadActivitySnapshot{
			Status:     normalizeSessionActivityStatus(thread.Status),
			Reason:     thread.Reason,
			QueueCount: thread.QueueCount,
			Message:    thread.Message,
		}
		if thread.CompletionID != nil {
			state.CompletionID = *thread.CompletionID
		}
		if thread.NextRunAfter != nil {
			state.NextRunAfter = thread.NextRunAfter.UTC().Format(time.RFC3339Nano)
		}
		threads[thread.ThreadID] = state
	}
	return threads
}

func (s *SessionThreadStatusSyncer) publishSessionThreadStatusChanged(ctx context.Context, projectID, sessionID string, sessionStatusChanged, threadActivityChanged bool) {
	if s == nil || (!sessionStatusChanged && !threadActivityChanged) || s.eventBroker == nil {
		return
	}
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	if sessionStatusChanged {
		if err := s.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, "", ""); err != nil {
			logger.Warn("failed to publish session thread status event", "error", err)
		}
	}
	if err := s.eventBroker.PublishSessionThreadsUpdated(ctx, projectID, sessionID); err != nil {
		logger.Warn("failed to publish session threads update event", "error", err)
	}
}

func (s *SessionThreadStatusSyncer) waitForStreamRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-s.stopChan:
		return false
	case <-timer.C:
		return true
	}
}
