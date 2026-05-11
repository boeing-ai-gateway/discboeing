package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/store"
)

func createThreadStatusSyncerFixtures(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	project := &model.Project{ID: "test-project", Name: "Test"}
	workspace := &model.Workspace{
		ID:          "test-workspace",
		ProjectID:   project.ID,
		Path:        "/test",
		SourceType:  "local",
		DisplayName: new("Test Workspace"),
	}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}
}

func createThreadStatusSyncerSession(ctx context.Context, t *testing.T, st *store.Store, id, status, threadStatus string) {
	t.Helper()

	if err := st.CreateSession(ctx, &model.Session{
		ID:           id,
		ProjectID:    "test-project",
		WorkspaceID:  "test-workspace",
		Name:         id,
		Status:       status,
		ThreadStatus: threadStatus,
	}); err != nil {
		t.Fatal(err)
	}
}

func newThreadStatusSyncerForTest(st *store.Store, provider *mockSandboxProvider) *SessionThreadStatusSyncer {
	sandboxSvc := NewSandboxService(st, provider, &config.Config{}, nil, nil, nil, nil)
	sessionSvc := NewSessionService(st, nil, sandboxSvc, nil, nil)
	return NewSessionThreadStatusSyncer(st, sessionSvc, slog.Default(), time.Hour)
}

func TestSessionThreadStatusSyncerPollsOnlyNonTerminalSessions(t *testing.T) {
	ctx := context.Background()
	st := setupTestStoreForIdleMonitor(t)
	createThreadStatusSyncerFixtures(ctx, t, st)

	var requests atomic.Int32
	provider := &mockSandboxProvider{
		secret: "test-secret",
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/threads/activity" {
				http.NotFound(w, r)
				return
			}
			requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"idle"}`)
		}),
	}

	createThreadStatusSyncerSession(ctx, t, st, "running-session", model.SessionStatusReady, model.SessionActivityStatusRunning)
	createThreadStatusSyncerSession(ctx, t, st, "queued-session", model.SessionStatusReady, model.SessionActivityStatusQueued)
	createThreadStatusSyncerSession(ctx, t, st, "unknown-session", model.SessionStatusReady, model.SessionActivityStatusUnknown)
	createThreadStatusSyncerSession(ctx, t, st, "idle-session", model.SessionStatusReady, model.SessionActivityStatusIdle)
	createThreadStatusSyncerSession(ctx, t, st, "attention-session", model.SessionStatusReady, model.SessionActivityStatusNeedsAttention)
	createThreadStatusSyncerSession(ctx, t, st, "stopped-running-session", model.SessionStatusStopped, model.SessionActivityStatusRunning)

	syncer := newThreadStatusSyncerForTest(st, provider)
	if err := syncer.syncNonTerminalSessions(ctx); err != nil {
		t.Fatalf("syncNonTerminalSessions failed: %v", err)
	}

	if got := requests.Load(); got != 3 {
		t.Fatalf("expected 3 activity requests for non-terminal ready sessions, got %d", got)
	}
	for _, id := range []string{"running-session", "queued-session", "unknown-session"} {
		session, err := st.GetSessionByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if session.ThreadStatus != model.SessionActivityStatusIdle {
			t.Fatalf("expected %s to refresh to idle, got %q", id, session.ThreadStatus)
		}
	}

	terminalCases := map[string]string{
		"idle-session":            model.SessionActivityStatusIdle,
		"attention-session":       model.SessionActivityStatusNeedsAttention,
		"stopped-running-session": model.SessionActivityStatusRunning,
	}
	for id, expected := range terminalCases {
		session, err := st.GetSessionByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if session.ThreadStatus != expected {
			t.Fatalf("expected %s to remain %q, got %q", id, expected, session.ThreadStatus)
		}
	}
}

func TestSessionThreadStatusSyncerStopsPollingTerminalSummary(t *testing.T) {
	ctx := context.Background()
	st := setupTestStoreForIdleMonitor(t)
	createThreadStatusSyncerFixtures(ctx, t, st)

	var requests atomic.Int32
	provider := &mockSandboxProvider{
		secret: "test-secret",
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/threads/activity" {
				http.NotFound(w, r)
				return
			}
			requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"needs_attention","reason":"pending_question","needsAttentionCount":1}`)
		}),
	}

	createThreadStatusSyncerSession(ctx, t, st, "test-session", model.SessionStatusReady, model.SessionActivityStatusRunning)

	syncer := newThreadStatusSyncerForTest(st, provider)
	if err := syncer.syncNonTerminalSessions(ctx); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	session, err := st.GetSessionByID(ctx, "test-session")
	if err != nil {
		t.Fatal(err)
	}
	if session.ThreadStatus != model.SessionActivityStatusNeedsAttention {
		t.Fatalf("expected status to refresh to needs_attention, got %q", session.ThreadStatus)
	}

	if err := syncer.syncNonTerminalSessions(ctx); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected terminal summary to fall out of poll set, got %d requests", got)
	}
}

func TestSessionThreadStatusSyncerDoesNotStartStoppedSandbox(t *testing.T) {
	ctx := context.Background()
	st := setupTestStoreForIdleMonitor(t)
	createThreadStatusSyncerFixtures(ctx, t, st)

	var requests atomic.Int32
	provider := &mockSandboxProvider{
		secret: "test-secret",
		status: sandbox.StatusStopped,
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			http.NotFound(w, r)
		}),
	}

	createThreadStatusSyncerSession(ctx, t, st, "test-session", model.SessionStatusReady, model.SessionActivityStatusRunning)

	syncer := newThreadStatusSyncerForTest(st, provider)
	if err := syncer.syncNonTerminalSessions(ctx); err != nil {
		t.Fatalf("syncNonTerminalSessions failed: %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("expected no agent requests for stopped sandbox, got %d", got)
	}

	session, err := st.GetSessionByID(ctx, "test-session")
	if err != nil {
		t.Fatal(err)
	}
	if session.ThreadStatus != model.SessionActivityStatusRunning {
		t.Fatalf("expected status to remain running when sandbox is stopped, got %q", session.ThreadStatus)
	}
}
