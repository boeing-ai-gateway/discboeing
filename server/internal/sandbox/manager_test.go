package sandbox

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

func TestProviderProxyWatchMergesCurrentProviders(t *testing.T) {
	ctx := t.Context()

	manager := NewProviderManager()
	first := newWatchableProvider()
	second := newWatchableProvider()
	manager.RegisterProvider("first", first)
	manager.RegisterProvider("second", second)
	proxy := NewProviderProxy(manager)

	events, err := proxy.Watch(ctx)
	if err != nil {
		t.Fatalf("watch proxy: %v", err)
	}
	first.waitForWatch(t)
	second.waitForWatch(t)

	first.emit(StateEvent{SessionID: "session-1", Status: StatusRunning})
	assertStateEvent(t, events, "session-1", StatusRunning)

	second.emit(StateEvent{SessionID: "session-2", Status: StatusStopped})
	assertStateEvent(t, events, "session-2", StatusStopped)
}

func TestProviderProxyWatchSubscribesToLaterProviders(t *testing.T) {
	ctx := t.Context()

	manager := NewProviderManager()
	first := newWatchableProvider()
	manager.RegisterProvider("first", first)
	proxy := NewProviderProxy(manager)

	events, err := proxy.Watch(ctx)
	if err != nil {
		t.Fatalf("watch proxy: %v", err)
	}
	first.waitForWatch(t)

	second := newWatchableProvider()
	manager.RegisterProvider("second", second)
	second.waitForWatch(t)

	second.emit(StateEvent{SessionID: "session-2", Status: StatusRunning})
	assertStateEvent(t, events, "session-2", StatusRunning)
}

func TestProviderManagerResolveProjectDefault(t *testing.T) {
	ctx := context.Background()
	st := setupProviderManagerTestStore(t)
	project := &model.Project{ID: "test-project", Name: "Test Project", Slug: "test-project"}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	defaultProvider := newWatchableProvider()
	otherProvider := newWatchableProvider()
	manager := NewProviderManager()
	manager.SetStore(st)
	manager.RegisterProvider("default", defaultProvider)
	manager.RegisterProvider("other", otherProvider)
	manager.SetDefault("default")

	provider, err := manager.resolveProjectDefault(ctx, st, project.ID)
	if err != nil {
		t.Fatalf("failed to resolve project default: %v", err)
	}
	if provider != defaultProvider {
		t.Fatalf("expected global default provider")
	}
}

func TestProviderManagerResolveProjectConfiguredDefault(t *testing.T) {
	ctx := context.Background()
	st := setupProviderManagerTestStore(t)
	project := &model.Project{
		ID:                       "test-project",
		Name:                     "Test Project",
		Slug:                     "test-project",
		DefaultSandboxProviderID: "other",
	}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	defaultProvider := newWatchableProvider()
	otherProvider := newWatchableProvider()
	manager := NewProviderManager()
	manager.SetStore(st)
	manager.RegisterProvider("default", defaultProvider)
	manager.RegisterProvider("other", otherProvider)
	manager.SetDefault("default")

	provider, err := manager.resolveProjectDefault(ctx, st, project.ID)
	if err != nil {
		t.Fatalf("failed to resolve project default: %v", err)
	}
	if provider != otherProvider {
		t.Fatalf("expected project default provider")
	}
}

func TestProviderManagerResolveSessionUsesGlobalDefaultWhenSessionProviderIDNull(t *testing.T) {
	ctx := context.Background()
	st := setupProviderManagerTestStore(t)
	project := &model.Project{
		ID:                       "test-project",
		Name:                     "Test Project",
		Slug:                     "test-project",
		DefaultSandboxProviderID: "other",
	}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  project.ID,
		Path:       "/tmp/test-workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:          "test-session",
		ProjectID:   project.ID,
		WorkspaceID: workspace.ID,
		Name:        "Test Session",
		Status:      model.SessionStatusReady,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if err := st.DB().WithContext(ctx).Exec("UPDATE sessions SET sandbox_provider_id = NULL WHERE id = ?", session.ID).Error; err != nil {
		t.Fatalf("failed to set session sandbox provider ID to null: %v", err)
	}

	defaultProvider := newWatchableProvider()
	otherProvider := newWatchableProvider()
	manager := NewProviderManager()
	manager.SetStore(st)
	manager.RegisterProvider("default", defaultProvider)
	manager.RegisterProvider("other", otherProvider)
	manager.SetDefault("default")

	provider, err := manager.ResolveForSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to resolve session provider: %v", err)
	}
	if provider != defaultProvider {
		t.Fatalf("expected global default provider")
	}
}

func TestProviderManagerResolveInstanceProviderAddsWatchProvider(t *testing.T) {
	ctx := context.Background()
	st := setupProviderManagerTestStore(t)
	project := &model.Project{ID: "test-project", Name: "Test Project", Slug: "test-project"}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  project.ID,
		Path:       "/tmp/test-workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := st.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	instance := &model.SandboxProviderInstance{
		ID:        "instance-provider",
		ProjectID: project.ID,
		Type:      "custom",
		Name:      "Custom Provider",
	}
	if err := st.CreateSandboxProviderInstance(ctx, instance); err != nil {
		t.Fatalf("failed to create provider instance: %v", err)
	}
	session := &model.Session{
		ID:                "test-session",
		ProjectID:         project.ID,
		WorkspaceID:       workspace.ID,
		Name:              "Test Session",
		Status:            model.SessionStatusReady,
		SandboxProviderID: instance.ID,
	}
	if err := st.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	manager := NewProviderManager()
	manager.SetStore(st)
	instanceProvider := newWatchableProvider()
	manager.RegisterProvider("default", newWatchableProvider())
	manager.SetDefault("default")
	manager.RegisterFactory("custom", func(context.Context, *model.SandboxProviderInstance) (Provider, error) {
		return instanceProvider, nil
	})
	proxy := NewProviderProxy(manager)
	events, err := proxy.Watch(t.Context())
	if err != nil {
		t.Fatalf("watch proxy: %v", err)
	}

	provider, err := manager.ResolveForSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to resolve session provider: %v", err)
	}
	if provider != instanceProvider {
		t.Fatalf("expected instance provider")
	}
	instanceProvider.waitForWatch(t)

	instanceProvider.emit(StateEvent{SessionID: session.ID, Status: StatusRunning})
	assertStateEvent(t, events, session.ID, StatusRunning)
}

func setupProviderManagerTestStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "provider-manager.db")), &gorm.Config{
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

func assertStateEvent(t *testing.T, events <-chan StateEvent, sessionID string, status Status) {
	t.Helper()

	select {
	case event := <-events:
		if event.SessionID != sessionID || event.Status != status {
			t.Fatalf("expected event %s/%s, got %s/%s", sessionID, status, event.SessionID, event.Status)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event %s/%s", sessionID, status)
	}
}

type watchableProvider struct {
	mu      sync.Mutex
	started chan struct{}
	subs    []chan StateEvent
}

func newWatchableProvider() *watchableProvider {
	return &watchableProvider{
		started: make(chan struct{}, 10),
	}
}

func (p *watchableProvider) waitForWatch(t *testing.T) {
	t.Helper()

	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for provider watch")
	}
}

func (p *watchableProvider) emit(event StateEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, sub := range p.subs {
		sub <- event
	}
}

func (p *watchableProvider) ImageExists(context.Context) bool { return true }
func (p *watchableProvider) Image() string                    { return "test-image" }
func (p *watchableProvider) List(context.Context) ([]*Sandbox, error) {
	return nil, nil
}
func (p *watchableProvider) Watch(ctx context.Context) (<-chan StateEvent, error) {
	ch := make(chan StateEvent, 10)
	p.mu.Lock()
	p.subs = append(p.subs, ch)
	p.mu.Unlock()
	p.started <- struct{}{}

	go func() {
		<-ctx.Done()
		close(ch)
	}()

	return ch, nil
}
func (p *watchableProvider) Reconcile(context.Context) error { return nil }
func (p *watchableProvider) RemoveProject(context.Context, string) error {
	return nil
}
func (p *watchableProvider) PrepareState(context.Context, string, CreateOptions) ([]byte, error) {
	return nil, nil
}
func (p *watchableProvider) Create(context.Context, []byte, string, CreateOptions) (*Sandbox, []byte, error) {
	return nil, nil, nil
}
func (p *watchableProvider) Start(context.Context, []byte, string) ([]byte, error) {
	return nil, nil
}
func (p *watchableProvider) Stop(context.Context, []byte, string, time.Duration) ([]byte, error) {
	return nil, nil
}
func (p *watchableProvider) Remove(context.Context, []byte, string, ...RemoveOption) ([]byte, error) {
	return nil, nil
}
func (p *watchableProvider) Get(context.Context, []byte, string) (*Sandbox, error) {
	return nil, ErrNotFound
}
func (p *watchableProvider) GetSecret(context.Context, []byte, string) (string, error) {
	return "", ErrNotFound
}
func (p *watchableProvider) AcquireHTTPClient(context.Context, []byte, string) (*HTTPClientLease, error) {
	return &HTTPClientLease{Client: http.DefaultClient}, nil
}
