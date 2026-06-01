package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/mock"
	"github.com/obot-platform/discobot/server/internal/store"
)

// Use the config constant for test consistency
var testImage = config.DefaultSandboxImage()
var testEncryptionKey = []byte("0123456789abcdef0123456789abcdef")

// sandboxCreatingInitializer provides a SessionInitializer for tests
// that actually creates sandboxes (unlike the no-op testSessionInitializer in perform_commit_test.go)
type sandboxCreatingInitializer struct {
	sandboxSvc *SandboxService
}

func (t *sandboxCreatingInitializer) Initialize(ctx context.Context, sessionID string) error {
	// Check if sandbox already exists (mimics SessionService.Initialize behavior)
	state, err := t.sandboxSvc.loadProviderState(ctx, sessionID)
	if err != nil {
		return err
	}
	existingSandbox, err := t.sandboxSvc.provider.Get(ctx, state, sessionID)
	if err != nil && !errors.Is(err, sandbox.ErrNotFound) {
		return err
	}

	if existingSandbox != nil {
		// Sandbox exists - handle based on status
		switch existingSandbox.Status {
		case sandbox.StatusRunning:
			return nil // Already running
		case sandbox.StatusCreated, sandbox.StatusStopped:
			// Try to start it
			newState, err := t.sandboxSvc.provider.Start(ctx, state, sessionID)
			if err == nil {
				err = t.sandboxSvc.saveProviderStateIfChanged(ctx, sessionID, state, newState)
				state = newState
			}
			if err != nil {
				// Start failed - remove and recreate
				_, _ = t.sandboxSvc.provider.Remove(ctx, state, sessionID)
				return t.sandboxSvc.CreateForSession(ctx, sessionID)
			}
			return nil
		default:
			// Failed state - remove and recreate
			_, _ = t.sandboxSvc.provider.Remove(ctx, state, sessionID)
			return t.sandboxSvc.CreateForSession(ctx, sessionID)
		}
	}

	// No existing sandbox - create new one
	return t.sandboxSvc.CreateForSession(ctx, sessionID)
}

// setupTestStore creates an in-memory SQLite database for testing
func setupTestStore(t *testing.T) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Run migrations
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return store.New(db, nil)
}

func TestSandboxGitControlSocketEnabled(t *testing.T) {
	t.Parallel()

	localWorkspace := &model.Workspace{
		Path:       "/home/user/repo",
		SourceType: model.WorkspaceSourceTypeLocal,
	}
	if !sandboxGitControlSocketEnabled(localWorkspace, "/home/user/repo") {
		t.Fatal("expected local filesystem workspace with a workspace path to enable git control socket")
	}

	gitURLWorkspace := &model.Workspace{
		Path:       "https://example.com/org/repo.git",
		SourceType: model.WorkspaceSourceTypeGit,
	}
	if sandboxGitControlSocketEnabled(gitURLWorkspace, "") {
		t.Fatal("expected sandbox-cloned git URL workspace without a workspace path to disable git control socket")
	}
	if sandboxGitControlSocketEnabled(gitURLWorkspace, "/tmp/server-clone") {
		t.Fatal("expected remote git URL workspace to disable git control socket")
	}
}

// createTestSession creates a session with the given workspace path for testing
func createTestSession(t *testing.T, s *store.Store, sessionID, workspacePath string) {
	t.Helper()

	ctx := context.Background()

	// Create a workspace first
	workspace := &model.Workspace{
		ID:         "test-workspace",
		ProjectID:  "test-project",
		Path:       workspacePath,
		SourceType: "local",
		Status:     "ready",
	}
	if err := s.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create test workspace: %v", err)
	}

	// Create the session with workspace path set
	session := &model.Session{
		ID:            sessionID,
		ProjectID:     "test-project",
		WorkspaceID:   "test-workspace",
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusReady,
		WorkspacePath: &workspacePath,
	}
	if err := s.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
}

func TestGetSessionActivityIfRunningDoesNotStartStoppedSandbox(t *testing.T) {
	ctx := context.Background()
	st := setupTestStore(t)
	sessionID := "stopped-session"
	createTestSession(t, st, sessionID, t.TempDir())
	if err := st.UpdateSessionStatus(ctx, sessionID, model.SessionStatusStopped, nil); err != nil {
		t.Fatalf("failed to stop test session: %v", err)
	}

	provider := newImageIDAwareReconcileProvider(testImage, "image-id")
	svc := NewSandboxService(st, provider, &config.Config{}, nil, nil, nil, nil)

	_, err := svc.GetSessionActivityIfRunning(ctx, sessionID)
	if !errors.Is(err, sandbox.ErrNotRunning) {
		t.Fatalf("expected ErrNotRunning, got %v", err)
	}
	if provider.createCount != 0 || provider.startCount != 0 || provider.removeCount != 0 {
		t.Fatalf("activity check changed sandbox lifecycle: create=%d start=%d remove=%d", provider.createCount, provider.startCount, provider.removeCount)
	}
}

func TestSandboxServiceStartSyncsSessionStateFromSandboxEvents(t *testing.T) {
	ctx := context.Background()
	st := setupTestStore(t)
	sessionID := "event-session"
	createTestSession(t, st, sessionID, t.TempDir())

	events := make(chan sandbox.StateEvent, 1)
	provider := newImageIDAwareReconcileProvider(testImage, "image-id")
	provider.watchEvents = events
	provider.watchStarted = make(chan struct{})
	svc := NewSandboxService(st, provider, &config.Config{}, nil, nil, nil, nil)

	done := make(chan error, 1)
	go func() {
		done <- svc.Start(ctx)
	}()

	select {
	case <-provider.watchStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sandbox service watch to start")
	}

	events <- sandbox.StateEvent{SessionID: sessionID, Status: sandbox.StatusStopped}
	close(events)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("sandbox service start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sandbox service watch to stop")
	}

	session, err := st.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if session.SandboxStatus != model.SessionStatusStopped {
		t.Fatalf("session.SandboxStatus = %q, want %q", session.SandboxStatus, model.SessionStatusStopped)
	}
}

type imageIDAwareReconcileProvider struct {
	sandboxes         map[string]*sandbox.Sandbox
	configuredImage   string
	configuredImageID string
	createErr         error
	createCount       int
	startCount        int
	removeCount       int
	cleanupCalls      int
	watchEvents       chan sandbox.StateEvent
	watchStarted      chan struct{}
}

func newImageIDAwareReconcileProvider(image, imageID string) *imageIDAwareReconcileProvider {
	return &imageIDAwareReconcileProvider{
		sandboxes:         make(map[string]*sandbox.Sandbox),
		configuredImage:   image,
		configuredImageID: imageID,
	}
}

func (p *imageIDAwareReconcileProvider) ImageExists(_ context.Context) bool {
	return true
}

func (p *imageIDAwareReconcileProvider) Image() string {
	return p.configuredImage
}

func (p *imageIDAwareReconcileProvider) CurrentImageID(_ context.Context) (string, error) {
	return p.configuredImageID, nil
}

func (p *imageIDAwareReconcileProvider) CleanupUnusedImages(_ context.Context) error {
	p.cleanupCalls++
	return nil
}

func (p *imageIDAwareReconcileProvider) PrepareState(context.Context, string, sandbox.CreateOptions) ([]byte, error) {
	return nil, nil
}

func (p *imageIDAwareReconcileProvider) Create(_ context.Context, state []byte, sessionID string, _ sandbox.CreateOptions) (*sandbox.Sandbox, []byte, error) {
	if p.createErr != nil {
		return nil, state, p.createErr
	}
	if _, exists := p.sandboxes[sessionID]; exists {
		return nil, state, sandbox.ErrAlreadyExists
	}

	p.createCount++
	sb := &sandbox.Sandbox{
		ID:        fmt.Sprintf("recreated-%d", p.createCount),
		SessionID: sessionID,
		Status:    sandbox.StatusCreated,
		Image:     p.configuredImage,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			sandbox.MetadataImageID: p.configuredImageID,
		},
	}
	p.sandboxes[sessionID] = sb
	return sb, state, nil
}

func (p *imageIDAwareReconcileProvider) Start(_ context.Context, state []byte, sessionID string) ([]byte, error) {
	sb, ok := p.sandboxes[sessionID]
	if !ok {
		return state, sandbox.ErrNotFound
	}

	p.startCount++
	now := time.Now()
	sb.Status = sandbox.StatusRunning
	sb.StartedAt = &now
	return state, nil
}

func (p *imageIDAwareReconcileProvider) Stop(_ context.Context, state []byte, sessionID string, _ time.Duration) ([]byte, error) {
	sb, ok := p.sandboxes[sessionID]
	if !ok {
		return state, sandbox.ErrNotFound
	}

	sb.Status = sandbox.StatusStopped
	return state, nil
}

func (p *imageIDAwareReconcileProvider) Remove(_ context.Context, state []byte, sessionID string, _ ...sandbox.RemoveOption) ([]byte, error) {
	p.removeCount++
	delete(p.sandboxes, sessionID)
	return state, nil
}

func (p *imageIDAwareReconcileProvider) Get(_ context.Context, _ []byte, sessionID string) (*sandbox.Sandbox, error) {
	sb, ok := p.sandboxes[sessionID]
	if !ok {
		return nil, sandbox.ErrNotFound
	}
	return sb, nil
}

func (p *imageIDAwareReconcileProvider) GetSecret(_ context.Context, _ []byte, _ string) (string, error) {
	return "", sandbox.ErrNotFound
}

func (p *imageIDAwareReconcileProvider) List(_ context.Context) ([]*sandbox.Sandbox, error) {
	result := make([]*sandbox.Sandbox, 0, len(p.sandboxes))
	for _, sb := range p.sandboxes {
		result = append(result, sb)
	}
	return result, nil
}

func (p *imageIDAwareReconcileProvider) AcquireHTTPClient(_ context.Context, _ []byte, _ string) (*sandbox.HTTPClientLease, error) {
	return &sandbox.HTTPClientLease{Client: &http.Client{
		Transport: healthRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := newResponseRecorder()
			switch req.URL.Path {
			case "/health":
				rec.Header().Set("Content-Type", "application/json")
				_, _ = rec.Write([]byte(`{"configured":true}`))
			case "/configure":
				rec.Header().Set("Content-Type", "text/event-stream")
				_, _ = rec.Write([]byte("data: {\"status\":\"ready\"}\n\n"))
			default:
				rec.WriteHeader(http.StatusOK)
			}
			return rec.Result(), nil
		}),
	}}, nil
}

func (p *imageIDAwareReconcileProvider) Watch(ctx context.Context) (<-chan sandbox.StateEvent, error) {
	if p.watchEvents != nil {
		if p.watchStarted != nil {
			close(p.watchStarted)
		}
		out := make(chan sandbox.StateEvent)
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case event, ok := <-p.watchEvents:
					if !ok {
						return
					}
					select {
					case out <- event:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
		return out, nil
	}
	ch := make(chan sandbox.StateEvent)
	close(ch)
	return ch, nil
}

func (p *imageIDAwareReconcileProvider) Reconcile(_ context.Context) error {
	return nil
}

func (p *imageIDAwareReconcileProvider) RemoveProject(_ context.Context, _ string) error {
	return nil
}

func (p *imageIDAwareReconcileProvider) ClearCache(_ context.Context, _ string) error {
	return nil
}

func TestSandboxUsesExpectedImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		sb              *sandbox.Sandbox
		expectedImage   string
		expectedImageID string
		want            bool
	}{
		{
			name: "matches by image id",
			sb: &sandbox.Sandbox{
				Image: "ghcr.io/obot-platform/discobot:alpha86",
				Metadata: map[string]string{
					sandbox.MetadataImageID: "sha256:match",
				},
			},
			expectedImage:   "ghcr.io/obot-platform/discobot:alpha90",
			expectedImageID: "sha256:match",
			want:            true,
		},
		{
			name: "mismatches by image id",
			sb: &sandbox.Sandbox{
				Image: "ghcr.io/obot-platform/discobot:alpha86",
				Metadata: map[string]string{
					sandbox.MetadataImageID: "sha256:old",
				},
			},
			expectedImage:   "ghcr.io/obot-platform/discobot:alpha90",
			expectedImageID: "sha256:new",
			want:            false,
		},
		{
			name: "falls back to image reference when metadata missing",
			sb: &sandbox.Sandbox{
				Image:    "ghcr.io/obot-platform/discobot:alpha90",
				Metadata: map[string]string{},
			},
			expectedImage:   "ghcr.io/obot-platform/discobot:alpha90",
			expectedImageID: "sha256:new",
			want:            true,
		},
		{
			name:          "nil sandbox never matches",
			sb:            nil,
			expectedImage: "ghcr.io/obot-platform/discobot:alpha90",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sandboxUsesExpectedImage(tt.sb, tt.expectedImage, tt.expectedImageID); got != tt.want {
				t.Fatalf("sandboxUsesExpectedImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

type healthAwareProvider struct {
	*mock.Provider
	mu               sync.Mutex
	healthStatusCode int
	healthStatuses   []int
}

func newHealthAwareProvider(image string, healthStatusCode int) *healthAwareProvider {
	return &healthAwareProvider{
		Provider:         mock.NewProviderWithImage(image),
		healthStatusCode: healthStatusCode,
	}
}

func newSequencedHealthAwareProvider(image string, healthStatuses ...int) *healthAwareProvider {
	statusCode := http.StatusOK
	if len(healthStatuses) > 0 {
		statusCode = healthStatuses[len(healthStatuses)-1]
	}
	return &healthAwareProvider{
		Provider:         mock.NewProviderWithImage(image),
		healthStatusCode: statusCode,
		healthStatuses:   append([]int(nil), healthStatuses...),
	}
}

func (p *healthAwareProvider) nextHealthStatus() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.healthStatuses) == 0 {
		return p.healthStatusCode
	}
	statusCode := p.healthStatuses[0]
	p.healthStatuses = p.healthStatuses[1:]
	p.healthStatusCode = statusCode
	return statusCode
}

func (p *healthAwareProvider) AcquireHTTPClient(_ context.Context, _ []byte, _ string) (*sandbox.HTTPClientLease, error) {
	return &sandbox.HTTPClientLease{Client: &http.Client{
		Transport: healthRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			rec := newResponseRecorder()
			switch req.URL.Path {
			case "/health":
				rec.WriteHeader(p.nextHealthStatus())
			default:
				rec.WriteHeader(http.StatusOK)
			}
			return rec.Result(), nil
		}),
	}}, nil
}

type healthRoundTripFunc func(*http.Request) (*http.Response, error)

func (f healthRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type responseRecorder struct {
	header http.Header
	body   []byte
	status int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header), status: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	r.body = append(r.body, data...)
	return len(data), nil
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *responseRecorder) Result() *http.Response {
	return &http.Response{
		StatusCode: r.status,
		Header:     r.header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(r.body)),
	}
}

type countingInitializer struct {
	calls int
}

func (i *countingInitializer) Initialize(_ context.Context, _ string) error {
	i.calls++
	return nil
}

func TestSandboxService_CreateForSession(t *testing.T) {
	mockProvider := mock.NewProviderWithImage(testImage)
	testStore := setupTestStore(t)
	cfg := &config.Config{
		SandboxIdleTimeout: 30 * time.Minute,
		EncryptionKey:      testEncryptionKey,
	}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/home/user/workspace"

	// Create test session with workspace path
	createTestSession(t, testStore, sessionID, workspacePath)

	err := svc.CreateForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("CreateForSession failed: %v", err)
	}

	// Verify sandbox was created and started
	sb, err := mockProvider.Get(ctx, nil, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if sb.Status != sandbox.StatusRunning {
		t.Errorf("Expected status %s, got %s", sandbox.StatusRunning, sb.Status)
	}

	if sb.Image != testImage {
		t.Errorf("Expected image %s, got %s", testImage, sb.Image)
	}
}

func TestSandboxService_EnsureSandboxReady_ReconcilesAfterWaitWhenHealthProbeFails(t *testing.T) {
	provider := newHealthAwareProvider(testImage, http.StatusServiceUnavailable)
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	initializer := &countingInitializer{}
	svc.SetSessionInitializer(initializer)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sessionID := "test-session-wait-reconcile"
	createTestSession(t, testStore, sessionID, "/workspace")
	if err := testStore.UpdateSessionStatus(ctx, sessionID, model.SessionStatusInitializing, nil); err != nil {
		t.Fatalf("failed to set session initializing: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = testStore.UpdateSessionStatus(context.Background(), sessionID, model.SessionStatusReady, nil)
	}()

	if err := svc.ensureSandboxReady(ctx, sessionID); err != nil {
		t.Fatalf("ensureSandboxReady failed: %v", err)
	}
	if initializer.calls != 1 {
		t.Fatalf("expected exactly one reconciliation, got %d", initializer.calls)
	}
}

func TestSandboxService_EnsureSandboxReady_ReconcilesErroredSession(t *testing.T) {
	provider := newHealthAwareProvider(testImage, http.StatusOK)
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	initializer := &countingInitializer{}
	svc.SetSessionInitializer(initializer)

	ctx := context.Background()
	sessionID := "test-session-error-reconcile"
	createTestSession(t, testStore, sessionID, "/workspace")
	errorMessage := "sandbox creation failed: image not found"
	if err := testStore.UpdateSessionStatus(ctx, sessionID, model.SessionStatusError, &errorMessage); err != nil {
		t.Fatalf("failed to set session error: %v", err)
	}

	if err := svc.ensureSandboxReady(ctx, sessionID); err != nil {
		t.Fatalf("ensureSandboxReady failed: %v", err)
	}
	if initializer.calls != 1 {
		t.Fatalf("expected one reconciliation, got %d", initializer.calls)
	}
}

func TestSandboxService_EnsureSandboxReady_DoesNotReconcileCreateFailedSession(t *testing.T) {
	provider := newHealthAwareProvider(testImage, http.StatusOK)
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	initializer := &countingInitializer{}
	svc.SetSessionInitializer(initializer)

	ctx := context.Background()
	sessionID := "test-session-create-failed-no-reconcile"
	createTestSession(t, testStore, sessionID, "/workspace")
	errorMessage := "sandbox creation failed: image not found"
	if err := testStore.UpdateSessionStatus(ctx, sessionID, model.SessionStatusCreateFailed, &errorMessage); err != nil {
		t.Fatalf("failed to set session create_failed: %v", err)
	}

	err := svc.ensureSandboxReady(ctx, sessionID)
	if err == nil {
		t.Fatal("expected ensureSandboxReady to fail for create_failed session")
	}
	if !strings.Contains(err.Error(), errorMessage) {
		t.Fatalf("expected stored session error, got %v", err)
	}
	if initializer.calls != 0 {
		t.Fatalf("expected no reconciliation, got %d", initializer.calls)
	}
}

func TestSessionService_Initialize_WaitsForSandboxHealthAfterStart(t *testing.T) {
	provider := newSequencedHealthAwareProvider(testImage, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK)
	testStore := setupTestStore(t)
	cfg := &config.Config{
		SandboxIdleTimeout: 30 * time.Minute,
		EncryptionKey:      testEncryptionKey,
	}
	sandboxSvc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	ctx := context.Background()
	workspace := &model.Workspace{
		ID:         "workspace-health-wait",
		ProjectID:  "test-project",
		Path:       "/workspace-health-wait",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	session := &model.Session{
		ID:            "session-health-wait",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := sessionSvc.Initialize(ctx, session.ID); err != nil {
		t.Fatalf("expected initialize to wait for transient sandbox health failures, got %v", err)
	}

	updatedSession, err := testStore.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if updatedSession.SandboxStatus != model.SessionStatusReady {
		t.Fatalf("expected session status %q, got %q", model.SessionStatusReady, updatedSession.SandboxStatus)
	}
}

func TestSessionService_Initialize_WaitsBeyondGenericRetryBudgetForSandboxHealth(t *testing.T) {
	healthStatuses := make([]int, 0, 19)
	for range 18 {
		healthStatuses = append(healthStatuses, http.StatusServiceUnavailable)
	}
	healthStatuses = append(healthStatuses, http.StatusOK)

	provider := newSequencedHealthAwareProvider(testImage, healthStatuses...)
	testStore := setupTestStore(t)
	cfg := &config.Config{
		SandboxIdleTimeout: 30 * time.Minute,
		EncryptionKey:      testEncryptionKey,
	}
	sandboxSvc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	ctx := context.Background()
	workspace := &model.Workspace{
		ID:         "workspace-health-wait-long",
		ProjectID:  "test-project",
		Path:       "/workspace-health-wait-long",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	session := &model.Session{
		ID:            "session-health-wait-long",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := sessionSvc.Initialize(ctx, session.ID); err != nil {
		t.Fatalf("expected initialize to keep waiting for longer transient sandbox health failures, got %v", err)
	}
}

func TestSessionService_Initialize_FailsWhenSandboxHealthProbeFails(t *testing.T) {
	provider := newHealthAwareProvider(testImage, http.StatusServiceUnavailable)
	testStore := setupTestStore(t)
	cfg := &config.Config{
		SandboxIdleTimeout: 30 * time.Minute,
		EncryptionKey:      testEncryptionKey,
	}
	sandboxSvc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	ctx := context.Background()
	workspace := &model.Workspace{
		ID:         "workspace-health-fail",
		ProjectID:  "test-project",
		Path:       "/workspace-health-fail",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	session := &model.Session{
		ID:            "session-health-fail",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	err := sessionSvc.Initialize(ctx, session.ID)
	if err == nil {
		t.Fatal("expected initialize to fail when sandbox health probe fails")
	}
	if !strings.Contains(err.Error(), "sandbox health check failed") {
		t.Fatalf("expected sandbox health check error, got %v", err)
	}

	updatedSession, err := testStore.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if updatedSession.SandboxStatus != model.SessionStatusError {
		t.Fatalf("expected session status %q, got %q", model.SessionStatusError, updatedSession.SandboxStatus)
	}
}

func TestSessionService_Initialize_ConfiguresRunningBootstrapSandbox(t *testing.T) {
	ctx := context.Background()
	provider := mock.NewProviderWithImage(testImage)
	testStore := setupTestStore(t)
	cfg := &config.Config{
		SandboxIdleTimeout: 30 * time.Minute,
		EncryptionKey:      testEncryptionKey,
	}
	sandboxSvc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	sessionID := "session-running-bootstrap"
	createTestSession(t, testStore, sessionID, "/workspace-running-bootstrap")
	if err := testStore.UpdateSessionStatus(ctx, sessionID, model.SessionStatusInitializing, nil); err != nil {
		t.Fatalf("failed to set session initializing: %v", err)
	}

	if _, _, err := provider.Create(ctx, nil, sessionID, sandbox.CreateOptions{}); err != nil {
		t.Fatalf("failed to create existing sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, sessionID); err != nil {
		t.Fatalf("failed to start existing sandbox: %v", err)
	}

	configured := false
	configureRequests := 0
	provider.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.Header().Set("Content-Type", "application/json")
			if !configured {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"code":"AGENT_NOT_CONFIGURED","configured":false}`))
				return
			}
			_, _ = w.Write([]byte(`{"configured":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/configure":
			configureRequests++
			configured = true
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"status\":\"ready\"}\n\n"))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	provider.RemoveFunc = func(context.Context, []byte, string, ...sandbox.RemoveOption) ([]byte, error) {
		t.Fatal("bootstrap sandbox should not be removed")
		return nil, nil
	}

	if err := sessionSvc.Initialize(ctx, sessionID); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if configureRequests != 1 {
		t.Fatalf("configure requests = %d, want 1", configureRequests)
	}
	updatedSession, err := testStore.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if updatedSession.SandboxStatus != model.SessionStatusReady {
		t.Fatalf("expected session status %q, got %q", model.SessionStatusReady, updatedSession.SandboxStatus)
	}
}

func TestReconcileSandboxes_UsesImageIDAndRunsCleanup(t *testing.T) {
	provider := newImageIDAwareReconcileProvider("ghcr.io/obot-platform/discobot:latest", "sha256:new")
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	svc.SetSessionInitializer(&sandboxCreatingInitializer{sandboxSvc: svc})

	ctx := context.Background()
	sessionID := "test-session-image-id"
	workspacePath := "/workspace"
	createTestSession(t, testStore, sessionID, workspacePath)

	provider.sandboxes[sessionID] = &sandbox.Sandbox{
		ID:        "old-sandbox",
		SessionID: sessionID,
		Status:    sandbox.StatusRunning,
		Image:     provider.configuredImage,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			sandbox.MetadataImageID: "sha256:old",
		},
	}

	if err := svc.ReconcileSandboxes(ctx); err != nil {
		t.Fatalf("ReconcileSandboxes failed: %v", err)
	}

	sb, err := provider.Get(ctx, nil, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if sb.ID == "old-sandbox" {
		t.Fatalf("expected sandbox to be recreated when image ID changed")
	}

	if got := sb.Metadata[sandbox.MetadataImageID]; got != provider.configuredImageID {
		t.Fatalf("expected recreated sandbox image ID %s, got %s", provider.configuredImageID, got)
	}

	if provider.cleanupCalls != 1 {
		t.Fatalf("expected cleanup to run once, got %d", provider.cleanupCalls)
	}
}

func TestReconcileSandboxes_RemovesStoppedOutdatedSandboxWithoutRestart(t *testing.T) {
	provider := newImageIDAwareReconcileProvider("ghcr.io/obot-platform/discobot:latest", "sha256:new")
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	svc.SetSessionInitializer(&sandboxCreatingInitializer{sandboxSvc: svc})

	ctx := context.Background()
	sessionID := "test-session-stopped-image-id"
	workspacePath := "/workspace"
	createTestSession(t, testStore, sessionID, workspacePath)

	provider.sandboxes[sessionID] = &sandbox.Sandbox{
		ID:        "old-stopped-sandbox",
		SessionID: sessionID,
		Status:    sandbox.StatusStopped,
		Image:     provider.configuredImage,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			sandbox.MetadataImageID: "sha256:old",
		},
	}

	if err := svc.ReconcileSandboxes(ctx); err != nil {
		t.Fatalf("ReconcileSandboxes failed: %v", err)
	}

	if _, err := provider.Get(ctx, nil, sessionID); !errors.Is(err, sandbox.ErrNotFound) {
		t.Fatalf("expected stopped outdated sandbox to be removed, got %v", err)
	}
	if provider.createCount != 0 {
		t.Fatalf("expected stopped outdated sandbox not to be recreated, got %d creates", provider.createCount)
	}
	if provider.startCount != 0 {
		t.Fatalf("expected stopped outdated sandbox not to be restarted, got %d starts", provider.startCount)
	}
	if provider.removeCount != 1 {
		t.Fatalf("expected stopped outdated sandbox to be removed once, got %d removes", provider.removeCount)
	}
}

func TestReconcileSandboxes_MarksUpgradeCreateFailureRetryable(t *testing.T) {
	provider := newImageIDAwareReconcileProvider("ghcr.io/obot-platform/discobot:latest", "sha256:new")
	provider.createErr = errors.New("no such image")
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)
	svc.SetSessionInitializer(&sandboxCreatingInitializer{sandboxSvc: svc})

	ctx := context.Background()
	sessionID := "test-session-upgrade-create-failure"
	createTestSession(t, testStore, sessionID, "/workspace")

	provider.sandboxes[sessionID] = &sandbox.Sandbox{
		ID:        "old-running-sandbox",
		SessionID: sessionID,
		Status:    sandbox.StatusRunning,
		Image:     provider.configuredImage,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			sandbox.MetadataImageID: "sha256:old",
		},
	}

	if err := svc.ReconcileSandboxes(ctx); err != nil {
		t.Fatalf("ReconcileSandboxes failed: %v", err)
	}

	session, err := testStore.GetSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if session.SandboxStatus != model.SessionStatusStopped {
		t.Fatalf("expected upgrade create failure to leave session retryable as %q, got %q", model.SessionStatusStopped, session.SandboxStatus)
	}
	if session.ErrorMessage == nil || !strings.Contains(*session.ErrorMessage, "no such image") {
		t.Fatalf("expected stored image upgrade error, got %v", session.ErrorMessage)
	}
	if _, err := provider.Get(ctx, nil, sessionID); !errors.Is(err, sandbox.ErrNotFound) {
		t.Fatalf("expected failed replacement sandbox to be absent, got %v", err)
	}
}

func TestReconcileSandboxes_PreservesRetainedDeletedSessionSandboxes(t *testing.T) {
	provider := newImageIDAwareReconcileProvider("ghcr.io/obot-platform/discobot:latest", "sha256:new")
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, provider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "retained-session-1"
	provider.sandboxes[sessionID] = &sandbox.Sandbox{
		ID:        "retained-sandbox",
		SessionID: sessionID,
		Status:    sandbox.StatusStopped,
		Image:     provider.configuredImage,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			sandbox.MetadataImageID: "sha256:old",
		},
	}

	resourceType := jobs.ResourceTypeRetainedSandbox
	resourceID := sessionID
	if err := testStore.CreateJob(ctx, &model.Job{
		Type:         string(jobs.JobTypeSessionSandboxDelete),
		Status:       string(model.JobStatusPending),
		Priority:     1,
		Payload:      []byte(`{"sessionId":"retained-session-1"}`),
		ResourceType: &resourceType,
		ResourceID:   &resourceID,
		ScheduledAt:  time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("failed to create retained sandbox delete job: %v", err)
	}

	if err := svc.ReconcileSandboxes(ctx); err != nil {
		t.Fatalf("ReconcileSandboxes failed: %v", err)
	}

	if _, err := provider.Get(ctx, nil, sessionID); err != nil {
		t.Fatalf("expected retained sandbox to be preserved, got error: %v", err)
	}
}

func TestSandboxService_CreateForSession_AlreadyExists(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/workspace"

	// Create test session
	createTestSession(t, testStore, sessionID, workspacePath)

	// Create first time
	err := svc.CreateForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("First CreateForSession failed: %v", err)
	}

	// Try to create again - should fail
	err = svc.CreateForSession(ctx, sessionID)
	if err == nil {
		t.Error("Expected error when creating duplicate sandbox")
	}
}

func TestSandboxService_GetForSession(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/workspace"

	// Create test session
	createTestSession(t, testStore, sessionID, workspacePath)

	// Create sandbox
	err := svc.CreateForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("CreateForSession failed: %v", err)
	}

	// Get sandbox
	sb, err := svc.GetForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetForSession failed: %v", err)
	}

	if sb.SessionID != sessionID {
		t.Errorf("Expected sessionID %s, got %s", sessionID, sb.SessionID)
	}
}

func TestSandboxService_GetForSession_NotFound(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()

	_, err := svc.GetForSession(ctx, "nonexistent")
	if err != sandbox.ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestSandboxService_EnsureSandboxReady_CreatesNew(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)
	// Provide a session initializer for the test fallback path
	svc.SetSessionInitializer(&sandboxCreatingInitializer{sandboxSvc: svc})

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/workspace"

	// Create test session
	createTestSession(t, testStore, sessionID, workspacePath)

	// GetClient should create sandbox if not exists (session is "ready")
	_, err := svc.GetClient(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	sb, err := mockProvider.Get(ctx, nil, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if sb.Status != sandbox.StatusRunning {
		t.Errorf("Expected status %s, got %s", sandbox.StatusRunning, sb.Status)
	}
}

func TestSandboxService_EnsureSandboxReady_AlreadyRunning(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/workspace"

	// Create test session
	createTestSession(t, testStore, sessionID, workspacePath)

	// Create and start
	err := svc.CreateForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("CreateForSession failed: %v", err)
	}

	// GetClient on already running sandbox should succeed
	_, err = svc.GetClient(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
}

func TestSandboxService_EnsureSandboxReady_StartsStopped(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)
	// Provide a session initializer for the test fallback path
	svc.SetSessionInitializer(&sandboxCreatingInitializer{sandboxSvc: svc})

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/workspace"

	// Create test session
	createTestSession(t, testStore, sessionID, workspacePath)

	// Create and start
	err := svc.CreateForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("CreateForSession failed: %v", err)
	}

	// Stop the sandbox
	err = svc.StopForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("StopForSession failed: %v", err)
	}

	// GetClient should restart the stopped sandbox
	_, err = svc.GetClient(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	sb, err := mockProvider.Get(ctx, nil, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if sb.Status != sandbox.StatusRunning {
		t.Errorf("Expected status %s, got %s", sandbox.StatusRunning, sb.Status)
	}
}

func TestSandboxService_DestroyForSession(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/workspace"

	// Create test session
	createTestSession(t, testStore, sessionID, workspacePath)

	// Create sandbox
	err := svc.CreateForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("CreateForSession failed: %v", err)
	}

	// Destroy sandbox
	err = svc.DestroyForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("DestroyForSession failed: %v", err)
	}

	// Verify sandbox is gone
	_, err = mockProvider.Get(ctx, nil, sessionID)
	if err != sandbox.ErrNotFound {
		t.Errorf("Expected ErrNotFound after destroy, got %v", err)
	}
}

func TestSandboxService_DestroyForSession_NotFound(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()

	// Destroy nonexistent sandbox should not error (idempotent)
	err := svc.DestroyForSession(ctx, "nonexistent")
	if err != nil {
		t.Errorf("DestroyForSession should be idempotent, got: %v", err)
	}
}

func TestSandboxService_Attach(t *testing.T) {
	mockProvider := mock.NewProvider()
	execServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/exec" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"test-exec","status":"running"}`))
		case strings.HasPrefix(r.URL.Path, "/exec/") && strings.HasSuffix(r.URL.Path, "/attach"):
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "done")
			if err := conn.Write(r.Context(), websocket.MessageBinary, []byte("$ ")); err != nil {
				return
			}
			for {
				if _, _, err := conn.Read(r.Context()); err != nil {
					return
				}
			}
		case r.URL.Path == "/exec/test-exec/kill" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer execServer.Close()
	execAddr := strings.TrimPrefix(execServer.URL, "http://")
	mockProvider.HTTPClient = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, execAddr)
		},
	}}
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "test-session-1"
	workspacePath := "/workspace"

	// Create test session
	createTestSession(t, testStore, sessionID, workspacePath)

	// Create sandbox
	err := svc.CreateForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("CreateForSession failed: %v", err)
	}

	// Attach PTY
	pty, err := svc.Attach(ctx, sessionID, 24, 80, "", "", nil)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}
	defer pty.Close()

	// Write to PTY
	_, err = pty.Write([]byte("test"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Read from PTY
	buf := make([]byte, 1024)
	n, err := pty.Read(buf)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if n == 0 {
		t.Error("Expected some output from PTY")
	}
}

func TestSandboxService_CreateForSession_NoWorkspacePath(t *testing.T) {
	mockProvider := mock.NewProvider()
	testStore := setupTestStore(t)
	cfg := &config.Config{EncryptionKey: testEncryptionKey}
	svc := NewSandboxService(testStore, mockProvider, cfg, nil, nil, nil, nil)

	ctx := context.Background()
	sessionID := "test-session-no-path"

	// Create workspace without setting workspace path on session
	workspace := &model.Workspace{
		ID:         "test-workspace-2",
		ProjectID:  "test-project",
		Path:       "/some/path",
		SourceType: "local",
		Status:     "ready",
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create test workspace: %v", err)
	}

	// Create session WITHOUT workspace path (simulating a session that hasn't been initialized)
	session := &model.Session{
		ID:            sessionID,
		ProjectID:     "test-project",
		WorkspaceID:   "test-workspace-2",
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusInitializing,
		// WorkspacePath is nil - not set
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	// CreateForSession should fail because workspace path is not set
	err := svc.CreateForSession(ctx, sessionID)
	if err == nil {
		t.Error("Expected error when session has no workspace path")
	}
}
