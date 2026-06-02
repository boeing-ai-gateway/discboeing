package service

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/local"
	mocksandbox "github.com/obot-platform/discobot/server/internal/sandbox/mock"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
	"github.com/obot-platform/discobot/server/internal/store"
)

func testSandboxConfig() *config.Config {
	return &config.Config{EncryptionKey: []byte("12345678901234567890123456789012")}
}

type imageIDAwareSessionProvider struct {
	*mocksandbox.Provider
	base           *mocksandbox.Provider
	currentImageID string
	ops            []string
}

func (p *imageIDAwareSessionProvider) CurrentImageID(context.Context) (string, error) {
	return p.currentImageID, nil
}

func (p *imageIDAwareSessionProvider) Create(ctx context.Context, state []byte, sessionID string, opts sandbox.CreateOptions) (*sandbox.Sandbox, []byte, error) {
	p.ops = append(p.ops, "create")
	return p.base.Create(ctx, state, sessionID, opts)
}

func (p *imageIDAwareSessionProvider) Start(ctx context.Context, state []byte, sessionID string) ([]byte, error) {
	sbx, err := p.base.Get(ctx, state, sessionID)
	if err != nil {
		p.ops = append(p.ops, "start:<missing>")
	} else {
		p.ops = append(p.ops, "start:"+sbx.ID)
	}
	return p.base.Start(ctx, state, sessionID)
}

func (p *imageIDAwareSessionProvider) Remove(ctx context.Context, state []byte, sessionID string, opts ...sandbox.RemoveOption) ([]byte, error) {
	sbx, err := p.base.Get(ctx, state, sessionID)
	if err != nil {
		p.ops = append(p.ops, "remove:<missing>")
	} else {
		p.ops = append(p.ops, "remove:"+sbx.ID)
	}
	return p.base.Remove(ctx, state, sessionID, opts...)
}

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid alphanumeric",
			sessionID: "abc123",
			wantErr:   false,
		},
		{
			name:      "valid with hyphens",
			sessionID: "session-123-abc",
			wantErr:   false,
		},
		{
			name:      "valid UUID format",
			sessionID: "550e8400-e29b-41d4-a716-446655440000",
			wantErr:   false,
		},
		{
			name:      "valid at max length (65 chars)",
			sessionID: strings.Repeat("a", 65),
			wantErr:   false,
		},
		{
			name:      "empty string",
			sessionID: "",
			wantErr:   true,
			errMsg:    "session ID is required",
		},
		{
			name:      "exceeds max length (66 chars)",
			sessionID: strings.Repeat("a", 66),
			wantErr:   true,
			errMsg:    "exceeds maximum length",
		},
		{
			name:      "contains underscore",
			sessionID: "session_123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains space",
			sessionID: "session 123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains special characters",
			sessionID: "session@123!",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains dot",
			sessionID: "session.123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains slash",
			sessionID: "session/123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "only hyphens",
			sessionID: "---",
			wantErr:   false,
		},
		{
			name:      "single character",
			sessionID: "a",
			wantErr:   false,
		},
		{
			name:      "contains newline",
			sessionID: "session\n123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionID(tt.sessionID)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSessionID(%q) expected error, got nil", tt.sessionID)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSessionID(%q) error = %v, expected to contain %q", tt.sessionID, err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSessionID(%q) unexpected error: %v", tt.sessionID, err)
				}
			}
		})
	}
}

func TestSessionIDMaxLength(t *testing.T) {
	// Verify the constant is set to 65
	if SessionIDMaxLength != 65 {
		t.Errorf("SessionIDMaxLength = %d, want 65", SessionIDMaxLength)
	}
}

func TestInitializeSessionGitURLPassesCloneInputsToSandbox(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	project := &model.Project{ID: "project-git", Name: "git project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	workspace := &model.Workspace{
		ID:         "workspace-git",
		ProjectID:  project.ID,
		Path:       "https://example.com/org/repo.git",
		SourceType: model.WorkspaceSourceTypeGit,
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	dbSession := &model.Session{
		ID:            "session-git",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		Name:          "Git Session",
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := svc.Initialize(ctx, dbSession.ID); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	opts, ok := provider.GetCreateOptions(dbSession.ID)
	if !ok {
		t.Fatalf("expected sandbox create options for session %s", dbSession.ID)
	}
	if opts.WorkspacePath != "" {
		t.Fatalf("WorkspacePath = %q, want empty for git URL workspace", opts.WorkspacePath)
	}
	if opts.WorkspaceSource != workspace.Path {
		t.Fatalf("WorkspaceSource = %q, want %q", opts.WorkspaceSource, workspace.Path)
	}
	if opts.WorkspaceTargetRef != defaultSessionTargetRef {
		t.Fatalf("WorkspaceTargetRef = %q, want %q", opts.WorkspaceTargetRef, defaultSessionTargetRef)
	}
	legacySessionEnv := "SESSION" + "_ID"
	if _, ok := opts.Env[legacySessionEnv]; ok {
		t.Fatalf("Env[%s] = %q, want unset", legacySessionEnv, opts.Env[legacySessionEnv])
	}
	if opts.Env["DISCOBOT_SESSION_ID"] != dbSession.ID {
		t.Fatalf("Env[DISCOBOT_SESSION_ID] = %q, want %q", opts.Env["DISCOBOT_SESSION_ID"], dbSession.ID)
	}
	for _, key := range []string{
		"DISCOBOT_PROJECT_ID",
		"WORKSPACE_SOURCE",
		"WORKSPACE_SOURCE_TYPE",
		"DISCOBOT_WORKSPACE_SOURCE_TYPE",
		"DISCOBOT_ENABLE_GIT_CONTROL_SOCKET",
		"WORKSPACE_TARGET_REF",
	} {
		if opts.Env[key] != "" {
			t.Fatalf("Env[%s] = %q, want empty because dynamic config is sent to /configure", key, opts.Env[key])
		}
	}
	if opts.Env["DISCOBOT_SECRET"] == "" {
		t.Fatal("Env[DISCOBOT_SECRET] is empty")
	}

	stored, err := testStore.GetSessionByID(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if stored.WorkspacePath != nil && *stored.WorkspacePath != "" {
		t.Fatalf("stored workspace path = %q, want empty", *stored.WorkspacePath)
	}
}

func TestInitializeSkipsSessionAlreadyRemoving(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	project := &model.Project{ID: "project-removing", Name: "removing project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:        "workspace-removing",
		ProjectID: project.ID,
		Path:      "/tmp/workspace",
		Status:    model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	dbSession := &model.Session{
		ID:            "session-removing",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		Name:          "Removing Session",
		SandboxStatus: model.SessionStatusRemoving,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := svc.Initialize(ctx, dbSession.ID); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if _, ok := provider.GetCreateOptions(dbSession.ID); ok {
		t.Fatalf("expected no sandbox create options for removing session")
	}
	stored, err := testStore.GetSessionByID(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if stored.SandboxStatus != model.SessionStatusRemoving {
		t.Fatalf("sandbox status = %q, want %q", stored.SandboxStatus, model.SessionStatusRemoving)
	}
}

func TestInitializeSessionWithUserUsesTrustKey(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	user := &model.User{
		ID:         "user-trust-key",
		Email:      "user@example.com",
		Provider:   "test",
		ProviderID: "user-trust-key",
	}
	if err := testStore.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	project := &model.Project{ID: "project-trust-key", Name: "trust key project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "workspace-trust-key",
		ProjectID:  project.ID,
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	dbSession := &model.Session{
		ID:              "session-trust-key",
		ProjectID:       project.ID,
		WorkspaceID:     workspace.ID,
		CreatedByUserID: &user.ID,
		Name:            "Trust Key Session",
		SandboxStatus:   model.SessionStatusInitializing,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := svc.Initialize(ctx, dbSession.ID); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	opts, ok := provider.GetCreateOptions(dbSession.ID)
	if !ok {
		t.Fatalf("expected sandbox create options for session %s", dbSession.ID)
	}
	if opts.SharedSecret != "" {
		t.Fatalf("SharedSecret = %q, want empty for trust-key auth", opts.SharedSecret)
	}
	if opts.Env["DISCOBOT_SECRET"] != "" {
		t.Fatalf("Env[DISCOBOT_SECRET] = %q, want empty for trust-key auth", opts.Env["DISCOBOT_SECRET"])
	}
	if opts.Env["DISCOBOT_TRUST_KEY"] == "" {
		t.Fatal("Env[DISCOBOT_TRUST_KEY] is empty")
	}
	storedUser, err := testStore.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if storedUser.SandboxPublicKey != opts.Env["DISCOBOT_TRUST_KEY"] {
		t.Fatalf("SandboxPublicKey = %q, want trust key %q", storedUser.SandboxPublicKey, opts.Env["DISCOBOT_TRUST_KEY"])
	}
	if storedUser.EncryptedSandboxPrivateKey == "" {
		t.Fatal("EncryptedSandboxPrivateKey is empty")
	}
}

func TestInitializeMarksCreateFailureTerminal(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	provider.CreateFunc = func(context.Context, []byte, string, sandbox.CreateOptions) (*sandbox.Sandbox, []byte, error) {
		return nil, nil, errors.New("provider quota exceeded")
	}
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	project := &model.Project{ID: "project-create-failed", Name: "create failed project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "workspace-create-failed",
		ProjectID:  project.ID,
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	dbSession := &model.Session{
		ID:            "session-create-failed",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		Name:          "Create Failed Session",
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := svc.Initialize(ctx, dbSession.ID); err == nil {
		t.Fatal("expected Initialize to fail")
	}
	stored, err := testStore.GetSessionByID(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if stored.SandboxStatus != model.SessionStatusCreateFailed {
		t.Fatalf("status = %q, want %q", stored.SandboxStatus, model.SessionStatusCreateFailed)
	}
	if stored.ErrorMessage == nil || !strings.Contains(*stored.ErrorMessage, "provider quota exceeded") {
		t.Fatalf("error message = %v", stored.ErrorMessage)
	}
}

func TestInitializeMarksRecreateFailureRecoverable(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	provider.CreateFunc = func(context.Context, []byte, string, sandbox.CreateOptions) (*sandbox.Sandbox, []byte, error) {
		return nil, nil, errors.New("provider quota exceeded")
	}
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	project := &model.Project{ID: "project-recreate-error", Name: "recreate error project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "workspace-recreate-error",
		ProjectID:  project.ID,
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	workspacePath := "/workspace"
	dbSession := &model.Session{
		ID:            "session-recreate-error",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		Name:          "Recreate Error Session",
		SandboxStatus: model.SessionStatusReinitializing,
		WorkspacePath: &workspacePath,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := svc.Initialize(ctx, dbSession.ID); err == nil {
		t.Fatal("expected Initialize to fail")
	}
	stored, err := testStore.GetSessionByID(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if stored.SandboxStatus != model.SessionStatusError {
		t.Fatalf("status = %q, want %q", stored.SandboxStatus, model.SessionStatusError)
	}
	if stored.ErrorMessage == nil || !strings.Contains(*stored.ErrorMessage, "provider quota exceeded") {
		t.Fatalf("error message = %v", stored.ErrorMessage)
	}
}

func TestStopSessionResetsCreateFailedWithoutSandbox(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	project := &model.Project{ID: "project-stop-create-failed", Name: "stop create failed project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "workspace-stop-create-failed",
		ProjectID:  project.ID,
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	errorMessage := "sandbox creation failed: no such image"
	dbSession := &model.Session{
		ID:            "session-stop-create-failed",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		Name:          "Stop Create Failed Session",
		SandboxStatus: model.SessionStatusCreateFailed,
		ErrorMessage:  &errorMessage,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session, err := svc.StopSession(ctx, project.ID, dbSession.ID)
	if err != nil {
		t.Fatalf("StopSession failed: %v", err)
	}
	if session.SandboxStatus != model.SessionStatusStopped {
		t.Fatalf("status = %q, want %q", session.SandboxStatus, model.SessionStatusStopped)
	}
	if session.ErrorMessage != "" {
		t.Fatalf("expected stop reset to clear error message, got %q", session.ErrorMessage)
	}
}

func TestStopSessionResetsErroredNotRunningSandbox(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	project := &model.Project{ID: "project-stop-error", Name: "stop error project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	workspace := &model.Workspace{
		ID:         "workspace-stop-error",
		ProjectID:  project.ID,
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	errorMessage := "sandbox failed"
	dbSession := &model.Session{
		ID:            "session-stop-error",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		Name:          "Stop Error Session",
		SandboxStatus: model.SessionStatusError,
		ErrorMessage:  &errorMessage,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if _, _, err := provider.Create(ctx, nil, dbSession.ID, sandbox.CreateOptions{}); err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	session, err := svc.StopSession(ctx, project.ID, dbSession.ID)
	if err != nil {
		t.Fatalf("StopSession failed: %v", err)
	}
	if session.SandboxStatus != model.SessionStatusStopped {
		t.Fatalf("status = %q, want %q", session.SandboxStatus, model.SessionStatusStopped)
	}
}

func TestInitializeRecreatesStoppedSandboxWhenImageIDChanges(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	baseProvider := mocksandbox.NewProviderWithImage("ghcr.io/obot-platform/discobot:alpha90")
	provider := &imageIDAwareSessionProvider{
		Provider:       baseProvider,
		base:           baseProvider,
		currentImageID: "sha256:new",
	}
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	svc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	project := &model.Project{ID: "project-stale", Name: "stale project"}
	if err := testStore.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	workspace := &model.Workspace{
		ID:         "workspace-stale",
		ProjectID:  project.ID,
		Path:       "/workspace",
		SourceType: model.WorkspaceSourceTypeLocal,
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	dbSession := &model.Session{
		ID:            "session-stale",
		ProjectID:     project.ID,
		WorkspaceID:   workspace.ID,
		Name:          "Stale Session",
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	oldSandbox, _, err := provider.base.Create(ctx, nil, dbSession.ID, sandbox.CreateOptions{SharedSecret: "test-secret"})
	if err != nil {
		t.Fatalf("failed to create stale sandbox: %v", err)
	}
	oldSandbox.ID = "old-stopped-sandbox"
	oldSandbox.Status = sandbox.StatusStopped
	oldSandbox.Image = provider.Image()
	oldSandbox.Metadata = map[string]string{
		sandbox.MetadataImageID: "sha256:old",
	}

	if err := svc.Initialize(ctx, dbSession.ID); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if got, want := provider.ops, []string{"remove:old-stopped-sandbox", "create", "start:mock-session-stale"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("operation order = %v, want %v", got, want)
	}

	sbx, err := provider.Get(ctx, nil, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to get recreated sandbox: %v", err)
	}
	if sbx.ID == "old-stopped-sandbox" {
		t.Fatalf("expected stopped stale sandbox to be recreated")
	}
	if sbx.Status != sandbox.StatusRunning {
		t.Fatalf("sandbox status = %s, want %s", sbx.Status, sandbox.StatusRunning)
	}
}

func TestSandboxClonesGitWorkspace(t *testing.T) {
	if !sandboxClonesGitWorkspace(mocksandbox.NewProvider()) {
		t.Fatal("mock sandbox should support sandbox-owned git clones")
	}

	localAgentBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to resolve test binary path: %v", err)
	}
	localProvider, err := local.NewProvider(&config.Config{LocalAgentBinary: localAgentBinary})
	if err != nil {
		t.Fatalf("failed to create local provider: %v", err)
	}
	if sandboxClonesGitWorkspace(localProvider) {
		t.Fatal("local sandbox provider should not use sandbox-owned git clones")
	}
}

func TestSessionServiceGetSessionSyncsNameFromPrimaryThread(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	workspace := &model.Workspace{
		ID:         "workspace-1",
		ProjectID:  "project-1",
		Path:       "/workspace",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	dbSession := &model.Session{
		ID:            "session-1",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusReady,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if _, _, err := provider.Create(ctx, nil, dbSession.ID, sandbox.CreateOptions{SharedSecret: "test-secret", WorkspacePath: workspace.Path}); err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, dbSession.ID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	client, err := sandboxSvc.GetClient(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to get sandbox client: %v", err)
	}
	if _, err := client.CreateThread(ctx, &sandboxapi.CreateThreadRequest{ID: dbSession.ID, Name: "Primary thread name"}); err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	session, err := sessionSvc.GetSession(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if session.Name != "Primary thread name" {
		t.Fatalf("expected synced session name, got %q", session.Name)
	}

	stored, err := testStore.GetSessionByID(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if stored.Name != "Primary thread name" {
		t.Fatalf("expected stored session name to be updated, got %q", stored.Name)
	}
}

func TestSessionServiceListSessionsByProjectDoesNotSyncNameFromPrimaryThread(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := mocksandbox.NewProvider()
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	workspace := &model.Workspace{
		ID:         "workspace-2",
		ProjectID:  "project-2",
		Path:       "/workspace-2",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	dbSession := &model.Session{
		ID:            "session-2",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusReady,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if _, _, err := provider.Create(ctx, nil, dbSession.ID, sandbox.CreateOptions{SharedSecret: "test-secret", WorkspacePath: workspace.Path}); err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	if _, err := provider.Start(ctx, nil, dbSession.ID); err != nil {
		t.Fatalf("failed to start sandbox: %v", err)
	}

	client, err := sandboxSvc.GetClient(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to get sandbox client: %v", err)
	}
	if _, err := client.CreateThread(ctx, &sandboxapi.CreateThreadRequest{ID: dbSession.ID, Name: "Listed thread name"}); err != nil {
		t.Fatalf("failed to create thread: %v", err)
	}

	sessions, err := sessionSvc.ListSessionsByProject(ctx, workspace.ProjectID)
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "" {
		t.Fatalf("expected list to avoid live thread name sync, got %q", sessions[0].Name)
	}
}

func TestSessionThreadStatusSyncerStaleSnapshotDoesNotLowerNewerSnapshot(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	syncer := NewSessionThreadStatusSyncer(testStore, nil, nil, nil, time.Hour)

	workspace := &model.Workspace{
		ID:         "workspace-stale-thread-status",
		ProjectID:  "project-stale-thread-status",
		Path:       "/workspace-stale-thread-status",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	dbSession := &model.Session{
		ID:            "session-stale-thread-status",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusReady,
		ThreadStatus:  model.SessionActivityStatusIdle,
	}
	if err := testStore.CreateSession(ctx, dbSession); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	staleObservedAt := time.Now().UTC().Add(-time.Minute)
	newerObservedAt := time.Now().UTC()
	if err := syncer.applyActivitySnapshot(ctx, workspace.ProjectID, dbSession.ID, &sandboxapi.SessionActivityResponse{
		Status: model.SessionActivityStatusRunning,
	}, newerObservedAt); err != nil {
		t.Fatalf("failed to apply running snapshot: %v", err)
	}
	if err := syncer.applyActivitySnapshot(ctx, workspace.ProjectID, dbSession.ID, &sandboxapi.SessionActivityResponse{
		Status: model.SessionActivityStatusIdle,
	}, staleObservedAt); err != nil {
		t.Fatalf("failed to apply stale idle snapshot: %v", err)
	}
	stored, err := testStore.GetSessionByID(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if stored.ThreadStatus != model.SessionActivityStatusRunning {
		t.Fatalf("thread status after stale snapshot = %q, want %q", stored.ThreadStatus, model.SessionActivityStatusRunning)
	}

	freshObservedAt := time.Now().UTC().Add(time.Minute)
	if err := syncer.applyActivitySnapshot(ctx, workspace.ProjectID, dbSession.ID, &sandboxapi.SessionActivityResponse{
		Status: model.SessionActivityStatusIdle,
	}, freshObservedAt); err != nil {
		t.Fatalf("failed to apply fresh idle snapshot: %v", err)
	}
	stored, err = testStore.GetSessionByID(ctx, dbSession.ID)
	if err != nil {
		t.Fatalf("failed to reload session after fresh snapshot: %v", err)
	}
	if stored.ThreadStatus != model.SessionActivityStatusIdle {
		t.Fatalf("thread status after fresh snapshot = %q, want %q", stored.ThreadStatus, model.SessionActivityStatusIdle)
	}
}

func TestStoppedSessionIncludesStoredNeedsAttentionStatus(t *testing.T) {
	t.Parallel()

	session := sessionThreadStatusFromModel(&model.Session{
		SandboxStatus: model.SessionStatusStopped,
		ThreadStatus:  model.SessionActivityStatusNeedsAttention,
	})
	if session == nil {
		t.Fatal("expected stopped session to include non-idle thread status")
	}
	if session.Status != model.SessionActivityStatusNeedsAttention {
		t.Fatalf("thread status = %q, want %q", session.Status, model.SessionActivityStatusNeedsAttention)
	}

	idleSession := sessionThreadStatusFromModel(&model.Session{
		SandboxStatus: model.SessionStatusStopped,
		ThreadStatus:  model.SessionActivityStatusIdle,
	})
	if idleSession != nil {
		t.Fatalf("expected stopped idle session to omit thread status, got %#v", idleSession)
	}
}

// TestMapSessionFieldCoverage ensures all model.Session fields are properly mapped to service.Session.
// This test uses reflection to verify complete field mapping and will fail if:
// 1. A field exists in model.Session but not in service.Session
// 2. A field exists but is not populated in mapSession
// This prevents bugs where new fields are added to the model but forgotten in the mapping.
func TestMapSessionFieldCoverage(t *testing.T) {
	// Create a fully populated model.Session with non-nil values
	strPtr := func(s string) *string { return &s }

	createdAt := time.Date(2026, time.March, 20, 8, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.March, 21, 9, 45, 0, 0, time.UTC)
	createdByUserID := "user-123"

	modelSession := &model.Session{
		ID:                   "test-id",
		ProjectID:            "test-project",
		WorkspaceID:          "test-workspace",
		CreatedByUserID:      &createdByUserID,
		Name:                 "test-name",
		DisplayName:          strPtr("Test Display"),
		Description:          strPtr("Test Description"),
		SandboxStatus:        "ready",
		SandboxStatusMessage: strPtr("sandbox progress"),
		ThreadStatus:         model.SessionActivityStatusNeedsAttention,
		CommitStatus:         model.CommitStatusCompleted,
		CommitOperation:      strPtr(model.CommitOperationCommit),
		CommitError:          strPtr("commit error"),
		TargetRef:            strPtr("HEAD"),
		AppliedCommit:        strPtr("applied456"),
		ErrorMessage:         strPtr("error message"),
		WorkspacePath:        strPtr("/path/to/workspace"),
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}

	// Create a mock SessionService (nil is fine since mapSession doesn't use it)
	svc := &SessionService{}

	// Map the session
	result := svc.mapSession(modelSession)

	// Define field mappings: model field -> service field
	// This map documents the expected mapping between model and service layer
	fieldMappings := map[string]string{
		"ID":                   "ID",
		"ProjectID":            "ProjectID",
		"WorkspaceID":          "WorkspaceID",
		"SandboxProviderID":    "ProviderID",
		"CreatedByUserID":      "CreatedByUserID",
		"Name":                 "Name",
		"DisplayName":          "DisplayName",
		"Description":          "Description",
		"SandboxStatus":        "SandboxStatus",
		"SandboxStatusMessage": "SandboxStatusMessage",
		"ThreadStatus":         "ThreadStatus",
		"CommitStatus":         "CommitStatus",
		"CommitOperation":      "CommitOperation",
		"CommitError":          "CommitError",
		"TargetRef":            "TargetRef",
		"AppliedCommit":        "AppliedCommit",
		"ErrorMessage":         "ErrorMessage",
		"WorkspacePath":        "WorkspacePath",
		"CreatedAt":            "CreatedAt",
		// Excluded fields (not part of API response):
		// - UpdatedAt: mapped to Timestamp
		// - Project, Workspace, Messages, SessionCommitLogs: relationships, not serialized
		// - Files: always initialized as empty array in mapSession
	}

	// Use reflection to verify all documented fields are mapped
	modelType := reflect.TypeFor[model.Session]()
	serviceType := reflect.TypeFor[Session]()

	// Check all model fields
	for modelField := range modelType.Fields() {
		modelFieldName := modelField.Name

		// Skip GORM metadata fields and relationship fields
		if modelFieldName == "UpdatedAt" ||
			modelFieldName == "DeletedAt" ||
			modelFieldName == "Project" || modelFieldName == "Workspace" ||
			modelFieldName == "Messages" || modelFieldName == "SessionCommitLogs" {
			continue
		}

		// Check if field is documented in fieldMappings
		serviceFieldName, mapped := fieldMappings[modelFieldName]
		if !mapped {
			t.Errorf("Model field %s is not documented in fieldMappings - add it or document why it's excluded", modelFieldName)
			continue
		}

		// Verify service type has the corresponding field
		serviceField, found := serviceType.FieldByName(serviceFieldName)
		if !found {
			t.Errorf("Service.Session missing field %s (maps from model.Session.%s)", serviceFieldName, modelFieldName)
			continue
		}

		// Verify the field was actually populated (not zero value)
		resultValue := reflect.ValueOf(*result)
		serviceValue := resultValue.FieldByName(serviceFieldName)

		// Special case for Files which is always initialized as empty array
		if serviceFieldName == "Files" {
			continue
		}

		// For string fields that come from pointers, verify they're not empty
		// (since we populated all pointers with non-empty values)
		if serviceField.Type.Kind() == reflect.String {
			if modelField.Type.Kind() == reflect.Pointer {
				// This field comes from a pointer, should be populated
				if serviceValue.String() == "" {
					t.Errorf("Field %s is empty but model.%s was set to a non-nil pointer", serviceFieldName, modelFieldName)
				}
			}
		}
	}

	// Verify specific field values to ensure mapping is correct
	if result.ID != "test-id" {
		t.Errorf("ID = %q, want %q", result.ID, "test-id")
	}
	if result.DisplayName != "Test Display" {
		t.Errorf("DisplayName = %q, want %q", result.DisplayName, "Test Display")
	}
	if result.CreatedAt != createdAt.Format(time.RFC3339) {
		t.Errorf("CreatedAt = %q, want %q", result.CreatedAt, createdAt.Format(time.RFC3339))
	}
	if result.Timestamp != updatedAt.Format(time.RFC3339) {
		t.Errorf("Timestamp = %q, want %q", result.Timestamp, updatedAt.Format(time.RFC3339))
	}
	if result.ThreadStatus == nil || result.ThreadStatus.Status != model.SessionActivityStatusNeedsAttention {
		t.Errorf("ThreadStatus = %#v, want status %q", result.ThreadStatus, model.SessionActivityStatusNeedsAttention)
	}

	// Verify Files is initialized (not nil)
	if result.Files == nil {
		t.Error("Files should be initialized to empty array, got nil")
	}
}

func TestSessionServicePerformDeletion_EnqueuesDeferredSandboxCleanup(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := &deferredCleanupProvider{Provider: mocksandbox.NewProvider()}

	workspace := &model.Workspace{
		ID:         "workspace-delete-1",
		ProjectID:  "project-delete-1",
		Path:       "/workspace-delete-1",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	session := &model.Session{
		ID:            "session-delete-1",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusReady,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	var stoppedSessionID string
	var stopTimeout time.Duration
	provider.StopFunc = func(_ context.Context, state []byte, sessionID string, timeout time.Duration) ([]byte, error) {
		stoppedSessionID = sessionID
		stopTimeout = timeout
		return state, nil
	}

	var queuedPayload jobs.SessionSandboxDeletePayload
	enqueuer := &mockJobEnqueuer{enqueueFunc: func(_ context.Context, payload jobs.JobPayload) error {
		var ok bool
		queuedPayload, ok = payload.(jobs.SessionSandboxDeletePayload)
		if !ok {
			t.Fatalf("unexpected payload type %T", payload)
		}
		return nil
	}}

	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, enqueuer)
	sessionSvc.SetSandboxCleanupDelay(30 * 24 * time.Hour)

	before := time.Now()
	if err := sessionSvc.PerformDeletion(ctx, workspace.ProjectID, session.ID); err != nil {
		t.Fatalf("PerformDeletion failed: %v", err)
	}
	after := time.Now()

	if stoppedSessionID != session.ID {
		t.Fatalf("stopped session ID = %q, want %q", stoppedSessionID, session.ID)
	}
	if stopTimeout != 10*time.Second {
		t.Fatalf("stop timeout = %s, want %s", stopTimeout, 10*time.Second)
	}
	if queuedPayload.SessionID != session.ID {
		t.Fatalf("queued session ID = %q, want %q", queuedPayload.SessionID, session.ID)
	}
	if queuedPayload.DeleteAt.Before(before.Add(30*24*time.Hour)) || queuedPayload.DeleteAt.After(after.Add(30*24*time.Hour)) {
		t.Fatalf("queued delete time %s outside expected retention window", queuedPayload.DeleteAt)
	}
	if _, err := testStore.GetSessionByID(ctx, session.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected session to be deleted, got err=%v", err)
	}
}

func TestSessionServiceDeleteSession_RequeuesRemovingSession(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)

	workspace := &model.Workspace{
		ID:         "workspace-delete-requeue",
		ProjectID:  "project-delete-requeue",
		Path:       "/workspace-delete-requeue",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:            "session-delete-requeue",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		Name:          "Delete Requeue",
		SandboxStatus: model.SessionStatusRemoving,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	var queuedPayload jobs.SessionDeletePayload
	enqueuer := &mockJobEnqueuer{enqueueFunc: func(_ context.Context, payload jobs.JobPayload) error {
		var ok bool
		queuedPayload, ok = payload.(jobs.SessionDeletePayload)
		if !ok {
			t.Fatalf("unexpected payload type %T", payload)
		}
		return nil
	}}

	sessionSvc := NewSessionService(testStore, nil, nil, nil, nil)
	if err := sessionSvc.DeleteSession(ctx, workspace.ProjectID, session.ID, enqueuer); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if queuedPayload.SessionID != session.ID {
		t.Fatalf("queued session ID = %q, want %q", queuedPayload.SessionID, session.ID)
	}
}

func TestSessionServiceDeleteSessionMarksCreateFailedPayload(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)

	workspace := &model.Workspace{
		ID:         "workspace-delete-create-failed-payload",
		ProjectID:  "project-delete-create-failed-payload",
		Path:       "/workspace-delete-create-failed-payload",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:            "session-delete-create-failed-payload",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		Name:          "Delete Create Failed Payload",
		SandboxStatus: model.SessionStatusCreateFailed,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	var queuedPayload jobs.SessionDeletePayload
	enqueuer := &mockJobEnqueuer{enqueueFunc: func(_ context.Context, payload jobs.JobPayload) error {
		var ok bool
		queuedPayload, ok = payload.(jobs.SessionDeletePayload)
		if !ok {
			t.Fatalf("unexpected payload type %T", payload)
		}
		return nil
	}}

	sessionSvc := NewSessionService(testStore, nil, nil, nil, nil)
	if err := sessionSvc.DeleteSession(ctx, workspace.ProjectID, session.ID, enqueuer); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	if !queuedPayload.CreateFailed {
		t.Fatal("expected create failed delete payload")
	}
}

func TestSessionServicePerformDeletion_RemovesCreateFailedSandboxImmediately(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := &deferredCleanupProvider{Provider: mocksandbox.NewProvider()}

	workspace := &model.Workspace{
		ID:         "workspace-delete-create-failed",
		ProjectID:  "project-delete-create-failed",
		Path:       "/workspace-delete-create-failed",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	session := &model.Session{
		ID:            "session-delete-create-failed",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusRemoving,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	provider.StopFunc = func(_ context.Context, state []byte, _ string, _ time.Duration) ([]byte, error) {
		t.Fatal("did not expect stop for create-failed deletion")
		return state, nil
	}
	var queued bool
	enqueuer := &mockJobEnqueuer{enqueueFunc: func(context.Context, jobs.JobPayload) error {
		queued = true
		return nil
	}}

	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, enqueuer)
	if err := sessionSvc.PerformDeletionFromDeleteJob(ctx, workspace.ProjectID, session.ID, true); err != nil {
		t.Fatalf("PerformDeletionFromDeleteJob failed: %v", err)
	}
	if len(provider.removeCalls) != 1 {
		t.Fatalf("expected one immediate sandbox removal, got %v", provider.removeCalls)
	}
	if provider.removeCalls[0].sessionID != session.ID {
		t.Fatalf("removed session ID = %q", provider.removeCalls[0].sessionID)
	}
	if !provider.removeCalls[0].cfg.RemoveVolumes {
		t.Fatal("expected immediate sandbox removal to delete volumes")
	}
	if queued {
		t.Fatal("did not expect deferred sandbox cleanup to be enqueued")
	}
	if _, err := testStore.GetSessionByID(ctx, session.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected session to be deleted, got err=%v", err)
	}
}

func TestSessionServicePerformDeletion_ContinuesWhenSandboxStopFails(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := &deferredCleanupProvider{Provider: mocksandbox.NewProvider()}

	workspace := &model.Workspace{
		ID:         "workspace-delete-stop-fails",
		ProjectID:  "project-delete-stop-fails",
		Path:       "/workspace-delete-stop-fails",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	session := &model.Session{
		ID:            "session-delete-stop-fails",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusRemoving,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	provider.StopFunc = func(_ context.Context, state []byte, _ string, _ time.Duration) ([]byte, error) {
		return state, context.DeadlineExceeded
	}

	var queued bool
	enqueuer := &mockJobEnqueuer{enqueueFunc: func(_ context.Context, payload jobs.JobPayload) error {
		if _, ok := payload.(jobs.SessionSandboxDeletePayload); !ok {
			t.Fatalf("unexpected payload type %T", payload)
		}
		queued = true
		return nil
	}}

	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, enqueuer)
	if err := sessionSvc.PerformDeletion(ctx, workspace.ProjectID, session.ID); err != nil {
		t.Fatalf("PerformDeletion failed: %v", err)
	}

	if !queued {
		t.Fatal("expected deferred sandbox cleanup to be enqueued")
	}
	if _, err := testStore.GetSessionByID(ctx, session.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected session to be deleted, got err=%v", err)
	}
}

func TestSessionServicePerformDeferredSandboxDeletion_SkipsWhenSessionExists(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := &deferredCleanupProvider{Provider: mocksandbox.NewProvider()}
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	workspace := &model.Workspace{
		ID:         "workspace-delete-2",
		ProjectID:  "project-delete-2",
		Path:       "/workspace-delete-2",
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := testStore.CreateWorkspace(ctx, workspace); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}

	session := &model.Session{
		ID:            "session-delete-2",
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		SandboxStatus: model.SessionStatusReady,
	}
	if err := testStore.CreateSession(ctx, session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if err := sessionSvc.PerformDeferredSandboxDeletion(ctx, session.ID); err != nil {
		t.Fatalf("PerformDeferredSandboxDeletion failed: %v", err)
	}
	if len(provider.removeCalls) != 0 {
		t.Fatalf("expected no sandbox removals, got %v", provider.removeCalls)
	}
}

func TestSessionServicePerformDeferredSandboxDeletion_RemovesWhenSessionStaysDeleted(t *testing.T) {
	ctx := context.Background()
	testStore := setupTestStore(t)
	provider := &deferredCleanupProvider{Provider: mocksandbox.NewProvider()}
	sandboxSvc := NewSandboxService(testStore, provider, testSandboxConfig(), nil, nil, nil, nil)
	sessionSvc := NewSessionService(testStore, nil, sandboxSvc, nil, nil)

	if err := sessionSvc.PerformDeferredSandboxDeletion(ctx, "session-delete-3"); err != nil {
		t.Fatalf("PerformDeferredSandboxDeletion failed: %v", err)
	}
	if len(provider.removeCalls) != 1 {
		t.Fatalf("expected one sandbox removal, got %v", provider.removeCalls)
	}
	if provider.removeCalls[0].sessionID != "session-delete-3" {
		t.Fatalf("removed session ID = %q", provider.removeCalls[0].sessionID)
	}
	if !provider.removeCalls[0].cfg.RemoveVolumes {
		t.Fatal("expected deferred sandbox removal to delete volumes")
	}
}

type deferredCleanupProvider struct {
	*mocksandbox.Provider
	removeCalls []removeCall
}

type removeCall struct {
	sessionID string
	cfg       sandbox.RemoveConfig
}

func (p *deferredCleanupProvider) Remove(ctx context.Context, state []byte, sessionID string, opts ...sandbox.RemoveOption) ([]byte, error) {
	if p.RemoveFunc != nil {
		return p.RemoveFunc(ctx, state, sessionID, opts...)
	}
	p.removeCalls = append(p.removeCalls, removeCall{sessionID: sessionID, cfg: sandbox.ParseRemoveOptions(opts)})
	return state, nil
}
