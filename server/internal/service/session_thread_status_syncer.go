package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/store"
)

const defaultThreadStatusSyncInterval = 10 * time.Second

// SessionThreadStatusSyncer refreshes persisted session-level thread summaries
// for sessions that are known to have non-terminal activity. Stream
// observation handles live UI traffic; this monitor covers the case where work
// finishes while nobody is watching the stream.
type SessionThreadStatusSyncer struct {
	store         *store.Store
	sessionSvc    *SessionService
	logger        *slog.Logger
	checkInterval time.Duration

	mu           sync.Mutex
	running      bool
	stopChan     chan struct{}
	wg           sync.WaitGroup
	shutdownOnce sync.Once
}

// NewSessionThreadStatusSyncer creates a background syncer for non-terminal
// session thread summaries.
func NewSessionThreadStatusSyncer(
	store *store.Store,
	sessionSvc *SessionService,
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
		sessionSvc:    sessionSvc,
		logger:        logger.With("component", "session_thread_status_syncer"),
		checkInterval: checkInterval,
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

	s.logger.Info("session thread status syncer started", "check_interval", s.checkInterval)
}

// Shutdown gracefully stops the background sync loop.
func (s *SessionThreadStatusSyncer) Shutdown(ctx context.Context) error {
	var err error
	s.shutdownOnce.Do(func() {
		s.logger.Info("shutting down session thread status syncer")
		close(s.stopChan)

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

func (s *SessionThreadStatusSyncer) syncNonTerminalSessions(ctx context.Context) error {
	if s == nil || s.store == nil || s.sessionSvc == nil {
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
		if err := s.sessionSvc.RefreshThreadStatus(ctx, session.ProjectID, session.ID); err != nil {
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
