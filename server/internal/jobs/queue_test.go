package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/store"
)

func TestQueueEnqueue_UsesScheduledAtFromPayload(t *testing.T) {
	ctx := context.Background()
	testStore := setupJobsTestStore(t)
	queue := NewQueue(testStore, &config.Config{JobMaxAttempts: 3})

	scheduledAt := time.Now().Add(24 * time.Hour).UTC().Round(time.Second)
	if err := queue.Enqueue(ctx, SessionSandboxDeletePayload{
		SessionID: "session-sandbox-job-1",
		DeleteAt:  scheduledAt,
	}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	job, err := testStore.GetJobByResourceID(ctx, ResourceTypeRetainedSandbox, "session-sandbox-job-1")
	if err != nil {
		t.Fatalf("failed to load queued job: %v", err)
	}
	if job.Type != string(JobTypeSessionSandboxDelete) {
		t.Fatalf("job type = %q, want %q", job.Type, JobTypeSessionSandboxDelete)
	}
	if !job.ScheduledAt.Equal(scheduledAt) {
		t.Fatalf("scheduled_at = %s, want %s", job.ScheduledAt, scheduledAt)
	}
}

func TestQueueEnqueue_SessionInitDoesNotRetry(t *testing.T) {
	ctx := context.Background()
	testStore := setupJobsTestStore(t)
	queue := NewQueue(testStore, &config.Config{JobMaxAttempts: 3})

	if err := queue.Enqueue(ctx, SessionInitPayload{
		ProjectID:   "project-1",
		SessionID:   "session-1",
		WorkspaceID: "workspace-1",
	}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	job, err := testStore.GetJobByResourceID(ctx, ResourceTypeSession, "session-1")
	if err != nil {
		t.Fatalf("failed to load queued job: %v", err)
	}
	if job.MaxAttempts != 1 {
		t.Fatalf("max_attempts = %d, want 1", job.MaxAttempts)
	}
}

func TestQueueEnqueue_SessionDeleteUsesHighPriority(t *testing.T) {
	ctx := context.Background()
	testStore := setupJobsTestStore(t)
	queue := NewQueue(testStore, &config.Config{JobMaxAttempts: 3})

	if err := queue.Enqueue(ctx, SessionDeletePayload{
		ProjectID: "project-1",
		SessionID: "session-1",
	}); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	job, err := testStore.GetJobByResourceID(ctx, ResourceTypeSession, "session-1")
	if err != nil {
		t.Fatalf("failed to load queued job: %v", err)
	}
	if job.Priority != 20 {
		t.Fatalf("priority = %d, want 20", job.Priority)
	}
}

func setupJobsTestStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return store.New(db, nil)
}
