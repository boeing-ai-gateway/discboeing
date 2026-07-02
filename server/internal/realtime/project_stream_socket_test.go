package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/database"
	"github.com/boeing-ai-gateway/discboeing/server/internal/events"
	"github.com/boeing-ai-gateway/discboeing/server/internal/model"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
	"github.com/boeing-ai-gateway/discboeing/server/internal/store"
)

func TestProjectEventHistorySkipsInitialReplayWithoutAfterID(t *testing.T) {
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

	history, err := socket.projectEventHistory(ctx, "", false)
	if err != nil {
		t.Fatalf("failed to load initial history: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected no initial history events, got %d", len(history))
	}

	history, err = socket.projectEventHistory(ctx, "", true)
	if err != nil {
		t.Fatalf("failed to load replay history: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 replay history events, got %d", len(history))
	}
	if history[0].ID != first.ID || history[1].ID != second.ID {
		t.Fatalf("expected replay history in order, got %q then %q", history[0].ID, history[1].ID)
	}

	history, err = socket.projectEventHistory(ctx, first.ID, false)
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
		ID:        fmt.Sprintf("event-%s", sessionID),
		Type:      events.EventTypeSessionUpdated,
		Timestamp: time.Now(),
		Data:      data,
	}
	if err := broker.Publish(context.Background(), projectID, event); err != nil {
		t.Fatalf("failed to publish event: %v", err)
	}
	return event
}

func TestProjectBrokerEventHydratesSessionAndWorkspace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, project, workspace := setupProjectStreamTestStore(ctx, t)
	broker := events.NewBroker(store, events.NewPoller(store, events.DefaultPollerConfig()))
	sessionSvc := service.NewSessionService(store, nil, nil, broker, nil)
	chatSvc := service.NewChatService(store, nil, sessionSvc, nil, broker, nil, nil)
	workspaceSvc := service.NewWorkspaceService(store, nil, nil, broker, nil)
	session := &model.Session{
		ID:            "session-1",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusReady,
	}
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	socket := &ProjectStreamSocket{
		chatService:      chatSvc,
		workspaceService: workspaceSvc,
		eventBroker:      broker,
		projectID:        project.ID,
		ctx:              ctx,
		cancel:           cancel,
		outgoing:         make(chan projectStreamSocketMessage, 2),
		subscriptions:    map[projectStreamSubscriptionKey]context.CancelFunc{},
	}

	sessionData, err := json.Marshal(events.SessionUpdatedData{SessionID: session.ID})
	if err != nil {
		t.Fatalf("failed to marshal session update: %v", err)
	}
	if !socket.writeProjectBrokerEvent(ctx, &events.Event{ID: "event-session", Type: events.EventTypeSessionUpdated, Data: sessionData}) {
		t.Fatal("failed to write session event")
	}
	sessionMessage := readProjectStreamTestMessage(t, socket.outgoing)
	assertProjectEventEnvelope(t, sessionMessage, "session_updated", "id", session.ID)

	workspaceData, err := json.Marshal(events.WorkspaceUpdatedData{WorkspaceID: workspace.ID})
	if err != nil {
		t.Fatalf("failed to marshal workspace update: %v", err)
	}
	if !socket.writeProjectBrokerEvent(ctx, &events.Event{ID: "event-workspace", Type: events.EventTypeWorkspaceUpdated, Data: workspaceData}) {
		t.Fatal("failed to write workspace event")
	}
	workspaceMessage := readProjectStreamTestMessage(t, socket.outgoing)
	assertProjectEventEnvelope(t, workspaceMessage, "workspace_updated", "id", workspace.ID)
}

func TestProjectBrokerEventWritesRemovedSessionSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, project, _ := setupProjectStreamTestStore(ctx, t)
	broker := events.NewBroker(store, events.NewPoller(store, events.DefaultPollerConfig()))
	sessionSvc := service.NewSessionService(store, nil, nil, broker, nil)
	chatSvc := service.NewChatService(store, nil, sessionSvc, nil, broker, nil, nil)
	socket := &ProjectStreamSocket{
		chatService:   chatSvc,
		projectID:     project.ID,
		ctx:           ctx,
		cancel:        cancel,
		outgoing:      make(chan projectStreamSocketMessage, 1),
		subscriptions: map[projectStreamSubscriptionKey]context.CancelFunc{},
	}

	sessionData, err := json.Marshal(events.SessionUpdatedData{
		SessionID:     "deleted-session",
		SandboxStatus: model.SessionStatusRemoved,
	})
	if err != nil {
		t.Fatalf("failed to marshal session update: %v", err)
	}
	if !socket.writeProjectBrokerEvent(ctx, &events.Event{ID: "event-session-removed", Type: events.EventTypeSessionUpdated, Data: sessionData}) {
		t.Fatal("failed to write session removed event")
	}

	sessionMessage := readProjectStreamTestMessage(t, socket.outgoing)
	assertProjectEventEnvelope(t, sessionMessage, "session_updated", "id", "deleted-session")
	assertProjectEventEnvelope(t, sessionMessage, "session_updated", "sandboxStatus", model.SessionStatusRemoved)
}

func TestSessionSubscriptionRequiresSessionID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	socket := &ProjectStreamSocket{
		ctx:           ctx,
		cancel:        cancel,
		outgoing:      make(chan projectStreamSocketMessage, 1),
		subscriptions: map[projectStreamSubscriptionKey]context.CancelFunc{},
	}

	socket.startSessionSubscription(projectStreamSubscriptionRequest{Stream: "session"})

	message := readProjectStreamTestMessage(t, socket.outgoing)
	if message.Type != "error" || message.Stream != "session" || message.Error != "sessionId is required" {
		t.Fatalf("message = %#v, want sessionId error", message)
	}
}

func TestWriteSessionSnapshotEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, project, workspace := setupProjectStreamTestStore(ctx, t)
	broker := events.NewBroker(store, events.NewPoller(store, events.DefaultPollerConfig()))
	sessionSvc := service.NewSessionService(store, nil, nil, broker, nil)
	chatSvc := service.NewChatService(store, nil, sessionSvc, nil, broker, nil, nil)
	session := &model.Session{
		ID:            "session-1",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusReady,
	}
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	socket := &ProjectStreamSocket{
		chatService:   chatSvc,
		projectID:     project.ID,
		ctx:           ctx,
		cancel:        cancel,
		outgoing:      make(chan projectStreamSocketMessage, 1),
		subscriptions: map[projectStreamSubscriptionKey]context.CancelFunc{},
	}

	if !socket.writeSessionSnapshotEvent(ctx, session.ID) {
		t.Fatal("failed to write session snapshot event")
	}

	message := readProjectStreamTestMessage(t, socket.outgoing)
	if message.Type != "event" || message.Stream != "session" || message.SessionID != session.ID || message.Event != "session_updated" {
		t.Fatalf("message = %#v, want session_updated event", message)
	}
	data, ok := message.Data.(*service.Session)
	if !ok {
		t.Fatalf("message data = %T, want *service.Session", message.Data)
	}
	if data.ID != session.ID || data.ProjectID != project.ID {
		t.Fatalf("session data = %#v, want session/project IDs", data)
	}
}

func setupProjectStreamTestStore(ctx context.Context, t *testing.T) (*store.Store, *model.Project, *model.Workspace) {
	t.Helper()

	cfg := &config.Config{
		DatabaseDSN:    fmt.Sprintf("sqlite3://%s/test.db", t.TempDir()),
		DatabaseDriver: "sqlite",
	}
	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	store := store.New(db.DB, db.ReadDB)
	project := &model.Project{Name: "Test Project", Slug: "test-project"}
	if err := store.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ProjectID: project.ID,
		Path:      "/tmp/project",
	}
	if err := store.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	return store, project, workspace
}

func assertProjectEventEnvelope(t *testing.T, message projectStreamSocketMessage, eventType, field, want string) {
	t.Helper()
	if message.Type != "event" || message.Stream != "project-events" || message.Event != eventType {
		t.Fatalf("message = %#v, want project event %q", message, eventType)
	}
	var envelope struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	var raw []byte
	switch data := message.Data.(type) {
	case string:
		raw = []byte(data)
	default:
		var err error
		raw, err = json.Marshal(message.Data)
		if err != nil {
			t.Fatalf("failed to marshal message data: %v", err)
		}
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("failed to unmarshal event envelope: %v", err)
	}
	if envelope.Type != eventType {
		t.Fatalf("envelope type = %q, want %q", envelope.Type, eventType)
	}
	var data map[string]any
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal event data: %v", err)
	}
	if got, _ := data[field].(string); got != want {
		t.Fatalf("data[%q] = %q, want %q", field, got, want)
	}
}

func readProjectStreamTestMessage(t *testing.T, messages <-chan projectStreamSocketMessage) projectStreamSocketMessage {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for project stream message")
		return projectStreamSocketMessage{}
	}
}
