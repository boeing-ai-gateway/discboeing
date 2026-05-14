package integration

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/service"
)

const expectedSessionTargetRef = "HEAD"

// TestSessionInitialize_SetsTargetRefOnFirstInit verifies that
// WorkspacePath is populated and TargetRef defaults to HEAD during first initialization.
func TestSessionInitialize_SetsTargetRefOnFirstInit(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")

	// Create a workspace with a real git repo.
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	expectedCommit := getGitHead(t, workspace.Path)

	session := &model.Session{
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := ts.Store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	freshSession, err := ts.Store.GetSessionByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if freshSession.WorkspacePath != nil {
		t.Errorf("Expected WorkspacePath to be nil before init, got %v", *freshSession.WorkspacePath)
	}
	if freshSession.TargetRef != nil {
		t.Errorf("Expected TargetRef to be nil before init, got %v", *freshSession.TargetRef)
	}

	gitSvc := service.NewGitService(ts.Store, ts.GitProvider)
	sessionSvc := service.NewSessionService(ts.Store, gitSvc, ts.SandboxService, nil, nil)

	ctx := context.Background()
	if err := sessionSvc.Initialize(ctx, session.ID); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	updatedSession, err := ts.Store.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}
	if updatedSession.WorkspacePath == nil || *updatedSession.WorkspacePath == "" {
		t.Error("Expected WorkspacePath to be set after init")
	}
	assertSessionTargetRef(t, updatedSession, expectedSessionTargetRef)

	createOpts, ok := ts.MockSandbox.GetCreateOptions(session.ID)
	if !ok {
		t.Fatalf("Expected sandbox create options for session %s", session.ID)
	}
	if createOpts.WorkspaceCommit != expectedCommit {
		t.Errorf("Expected sandbox WorkspaceCommit %s, got %s", expectedCommit, createOpts.WorkspaceCommit)
	}
}

// TestSessionInitialize_UsesCurrentWorkspaceCommitForSandboxReconcile verifies that
// reconcile preserves stored session metadata but recreates the sandbox from the
// workspace's current HEAD.
func TestSessionInitialize_UsesCurrentWorkspaceCommitForSandboxReconcile(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")

	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	initialCommit := getGitHead(t, workspace.Path)

	session := &model.Session{
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := ts.Store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	gitSvc := service.NewGitService(ts.Store, ts.GitProvider)
	sessionSvc := service.NewSessionService(ts.Store, gitSvc, ts.SandboxService, nil, nil)

	ctx := context.Background()
	if err := sessionSvc.Initialize(ctx, session.ID); err != nil {
		t.Fatalf("First Initialize failed: %v", err)
	}

	afterFirstInit, err := ts.Store.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session after first init: %v", err)
	}
	if afterFirstInit.WorkspacePath == nil || *afterFirstInit.WorkspacePath == "" {
		t.Fatal("Expected WorkspacePath to be set after first init")
	}
	assertSessionTargetRef(t, afterFirstInit, expectedSessionTargetRef)
	originalPath := *afterFirstInit.WorkspacePath

	firstCreateOpts, ok := ts.MockSandbox.GetCreateOptions(session.ID)
	if !ok {
		t.Fatalf("Expected sandbox create options for first init")
	}
	if firstCreateOpts.WorkspaceCommit != initialCommit {
		t.Fatalf("Expected first sandbox WorkspaceCommit %s, got %s", initialCommit, firstCreateOpts.WorkspaceCommit)
	}

	makeCommit(t, workspace.Path, "second.txt", "Second commit")
	newCommit := getGitHead(t, workspace.Path)
	if newCommit == initialCommit {
		t.Fatal("Expected new commit to be different from initial commit")
	}

	afterFirstInit.SandboxStatus = model.SessionStatusError
	if err := ts.Store.UpdateSession(ctx, afterFirstInit); err != nil {
		t.Fatalf("Failed to update session status: %v", err)
	}
	if _, err := ts.MockSandbox.Remove(ctx, nil, session.ID); err != nil {
		t.Fatalf("Failed to remove sandbox: %v", err)
	}

	if err := sessionSvc.Initialize(ctx, session.ID); err != nil {
		t.Fatalf("Second Initialize (reconcile) failed: %v", err)
	}

	afterReconcile, err := ts.Store.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session after reconcile: %v", err)
	}
	if afterReconcile.WorkspacePath == nil || *afterReconcile.WorkspacePath != originalPath {
		t.Errorf("Expected WorkspacePath to be preserved as %s, got %v", originalPath, afterReconcile.WorkspacePath)
	}
	assertSessionTargetRef(t, afterReconcile, expectedSessionTargetRef)

	reconcileCreateOpts, ok := ts.MockSandbox.GetCreateOptions(session.ID)
	if !ok {
		t.Fatalf("Expected sandbox create options after reconcile")
	}
	if reconcileCreateOpts.WorkspaceCommit != newCommit {
		t.Errorf("Expected reconcile sandbox WorkspaceCommit %s, got %s", newCommit, reconcileCreateOpts.WorkspaceCommit)
	}
}

// TestSessionInitialize_EnsuresWorkspaceOnReconcile verifies that
// EnsureWorkspaceRepo is called even during reconcile so sandbox recreation can
// use the current workspace commit.
func TestSessionInitialize_EnsuresWorkspaceOnReconcile(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")

	workspace := ts.CreateTestWorkspaceWithGitRepo(project)
	workspacePath := workspace.Path
	workspaceCommit := getGitHead(t, workspace.Path)
	targetRef := expectedSessionTargetRef
	session := &model.Session{
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusError,
		WorkspacePath: &workspacePath,
		TargetRef:     &targetRef,
	}
	if err := ts.Store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	gitSvc := service.NewGitService(ts.Store, ts.GitProvider)
	sessionSvc := service.NewSessionService(ts.Store, gitSvc, ts.SandboxService, nil, nil)

	ctx := context.Background()
	if err := sessionSvc.Initialize(ctx, session.ID); err != nil {
		t.Fatalf("Initialize (reconcile) failed: %v", err)
	}

	sbx, err := ts.MockSandbox.Get(ctx, nil, session.ID)
	if err != nil {
		t.Fatalf("Failed to get sandbox after reconcile: %v", err)
	}
	if sbx == nil {
		t.Fatal("Expected sandbox to exist after reconcile")
	}

	updatedSession, err := ts.Store.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if updatedSession.SandboxStatus != model.SessionStatusReady {
		t.Errorf("Expected session status to be 'ready', got '%s'", updatedSession.SandboxStatus)
	}
	assertSessionTargetRef(t, updatedSession, expectedSessionTargetRef)

	createOpts, ok := ts.MockSandbox.GetCreateOptions(session.ID)
	if !ok {
		t.Fatalf("Expected sandbox create options for reconcile")
	}
	if createOpts.WorkspaceCommit != workspaceCommit {
		t.Errorf("Expected sandbox WorkspaceCommit %s, got %s", workspaceCommit, createOpts.WorkspaceCommit)
	}
}

// TestMapSession_IncludesTargetRefAndWorkspacePath verifies that mapSession includes
// the WorkspacePath and TargetRef fields in the service.Session.
func TestMapSession_IncludesTargetRefAndWorkspacePath(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspaceWithGitRepo(project)

	workspacePath := workspace.Path
	targetRef := expectedSessionTargetRef
	session := &model.Session{
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusReady,
		WorkspacePath: &workspacePath,
		TargetRef:     &targetRef,
	}
	if err := ts.Store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	gitSvc := service.NewGitService(ts.Store, ts.GitProvider)
	sessionSvc := service.NewSessionService(ts.Store, gitSvc, ts.SandboxService, nil, nil)

	ctx := context.Background()
	svcSession, err := sessionSvc.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if svcSession.WorkspacePath != workspacePath {
		t.Errorf("Expected WorkspacePath %s, got %s", workspacePath, svcSession.WorkspacePath)
	}
	if svcSession.TargetRef != targetRef {
		t.Errorf("Expected TargetRef %s, got %s", targetRef, svcSession.TargetRef)
	}
}

// TestSessionInitialize_NoGitService verifies initialization works without git service
// (fallback path for testing scenarios).
func TestSessionInitialize_NoGitService(t *testing.T) {
	ts := NewTestServer(t)
	user := ts.CreateTestUser("test@example.com")
	project := ts.CreateTestProject(user, "Test Project")
	workspace := ts.CreateTestWorkspace(project, "/some/local/path")
	session := &model.Session{
		ProjectID:     workspace.ProjectID,
		WorkspaceID:   workspace.ID,
		Name:          "Test Session",
		SandboxStatus: model.SessionStatusInitializing,
	}
	if err := ts.Store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	sessionSvc := service.NewSessionService(ts.Store, nil, ts.SandboxService, nil, nil)

	ctx := context.Background()
	if err := sessionSvc.Initialize(ctx, session.ID); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	updatedSession, err := ts.Store.GetSessionByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if updatedSession.WorkspacePath == nil || *updatedSession.WorkspacePath != workspace.Path {
		t.Errorf("Expected WorkspacePath to be %s, got %v", workspace.Path, updatedSession.WorkspacePath)
	}
	assertSessionTargetRef(t, updatedSession, expectedSessionTargetRef)

	createOpts, ok := ts.MockSandbox.GetCreateOptions(session.ID)
	if !ok {
		t.Fatalf("Expected sandbox create options for session %s", session.ID)
	}
	if createOpts.WorkspaceCommit != "" {
		t.Errorf("Expected empty sandbox WorkspaceCommit without git service, got %q", createOpts.WorkspaceCommit)
	}
}

func assertSessionTargetRef(t *testing.T, session *model.Session, want string) {
	t.Helper()
	if session.TargetRef == nil {
		t.Fatalf("Expected TargetRef %s, got nil", want)
	}
	if *session.TargetRef != want {
		t.Fatalf("Expected TargetRef %s, got %s", want, *session.TargetRef)
	}
}

// Helper functions

func getGitHead(t *testing.T, repoPath string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get git HEAD: %v", err)
	}
	return string(out[:len(out)-1]) // trim newline
}

func makeCommit(t *testing.T, repoPath, filename, message string) {
	t.Helper()

	filepath := repoPath + "/" + filename
	if err := os.WriteFile(filepath, []byte(message+"\n"), 0644); err != nil {
		t.Fatalf("Failed to create file %s: %v", filename, err)
	}

	cmd := exec.Command("git", "add", filename)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}
}
