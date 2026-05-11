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
