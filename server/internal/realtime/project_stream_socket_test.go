package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/database"
	"github.com/obot-platform/discobot/server/internal/events"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

func TestProjectEventHistoryReplaysAllWithoutAfterID(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		DatabaseDSN:    fmt.Sprintf("sqlite3://%s/test.db", t.TempDir()),
		DatabaseDriver: "sqlite",
	}
	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	store := store.New(db.DB, db.ReadDB)
	project := &model.Project{Name: "Test Project", Slug: "test-project"}
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	poller := events.NewPoller(store, events.DefaultPollerConfig())
	broker := events.NewBroker(store, poller)
	socket := &ProjectStreamSocket{eventBroker: broker, projectID: project.ID}

	first := publishProjectEvent(t, broker, project.ID, "session-1")
	time.Sleep(time.Millisecond)
	second := publishProjectEvent(t, broker, project.ID, "session-2")

	history, err := socket.projectEventHistory(ctx, "")
	if err != nil {
		t.Fatalf("failed to load full history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history events, got %d", len(history))
	}
	if history[0].ID != first.ID || history[1].ID != second.ID {
		t.Fatalf("expected full history in order, got %q then %q", history[0].ID, history[1].ID)
	}

	history, err = socket.projectEventHistory(ctx, first.ID)
	if err != nil {
		t.Fatalf("failed to load resumed history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 resumed history event, got %d", len(history))
	}
	if history[0].ID != second.ID {
		t.Fatalf("expected resumed history to contain %q, got %q", second.ID, history[0].ID)
	}
}

func publishProjectEvent(t *testing.T, broker *events.Broker, projectID, sessionID string) *events.Event {
	t.Helper()
	data, err := json.Marshal(events.SessionUpdatedData{SessionID: sessionID})
	if err != nil {
		t.Fatalf("failed to marshal event data: %v", err)
	}
	event := &events.Event{
		ID:        fmt.Sprintf("%s-%d", sessionID, time.Now().UnixNano()),
		Type:      events.EventTypeSessionUpdated,
		Timestamp: time.Now(),
		Data:      data,
	}
	if err := broker.Publish(context.Background(), projectID, event); err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}
	return event
}
