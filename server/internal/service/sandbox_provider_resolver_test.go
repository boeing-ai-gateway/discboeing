package service

import (
	"context"
	"testing"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	mocksandbox "github.com/obot-platform/discobot/server/internal/sandbox/mock"
)

func TestSandboxProviderResolverUsesGlobalDefaultWhenProjectDefaultUnset(t *testing.T) {
	ctx := context.Background()
	st := setupTestStore(t)
	project := &model.Project{ID: "test-project", Name: "Test Project", Slug: "test-project"}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	defaultProvider := mocksandbox.NewProvider()
	otherProvider := mocksandbox.NewProvider()
	manager := sandbox.NewManager()
	manager.RegisterProvider("default", defaultProvider)
	manager.RegisterProvider("other", otherProvider)
	manager.SetDefault("default")

	resolver := NewSandboxProviderResolver(st, manager)
	provider, err := resolver.ResolveProjectDefault(ctx, project.ID)
	if err != nil {
		t.Fatalf("failed to resolve project default: %v", err)
	}
	if provider != defaultProvider {
		t.Fatalf("expected global default provider")
	}
}

func TestSandboxProviderResolverUsesProjectDefaultWhenSet(t *testing.T) {
	ctx := context.Background()
	st := setupTestStore(t)
	project := &model.Project{
		ID:                       "test-project",
		Name:                     "Test Project",
		Slug:                     "test-project",
		DefaultSandboxProviderID: "other",
	}
	if err := st.CreateProject(ctx, project); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	defaultProvider := mocksandbox.NewProvider()
	otherProvider := mocksandbox.NewProvider()
	manager := sandbox.NewManager()
	manager.RegisterProvider("default", defaultProvider)
	manager.RegisterProvider("other", otherProvider)
	manager.SetDefault("default")

	resolver := NewSandboxProviderResolver(st, manager)
	provider, err := resolver.ResolveProjectDefault(ctx, project.ID)
	if err != nil {
		t.Fatalf("failed to resolve project default: %v", err)
	}
	if provider != otherProvider {
		t.Fatalf("expected project default provider")
	}
}

func TestSandboxProviderResolverUsesGlobalDefaultWhenSessionProviderIDNull(t *testing.T) {
	ctx := context.Background()
	st := setupTestStore(t)
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

	defaultProvider := mocksandbox.NewProvider()
	otherProvider := mocksandbox.NewProvider()
	manager := sandbox.NewManager()
	manager.RegisterProvider("default", defaultProvider)
	manager.RegisterProvider("other", otherProvider)
	manager.SetDefault("default")

	resolver := NewSandboxProviderResolver(st, manager)
	provider, err := resolver.ResolveForSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to resolve session provider: %v", err)
	}
	if provider != defaultProvider {
		t.Fatalf("expected global default provider")
	}
}
