package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/store"
)

// SandboxProviderFactory builds a runtime provider from a project-scoped saved
// provider instance.
type SandboxProviderFactory func(ctx context.Context, instance *model.SandboxProviderInstance) (sandbox.Provider, error)

// SandboxProviderResolver resolves the runtime sandbox provider for a project or
// session. Process-wide providers, such as Docker, are read from the manager;
// project-scoped provider instances are built from their saved configuration.
type SandboxProviderResolver struct {
	store     *store.Store
	manager   *sandbox.Manager
	factories map[string]SandboxProviderFactory
	mu        sync.Mutex
	cache     map[string]cachedSandboxProvider
}

type cachedSandboxProvider struct {
	provider  sandbox.Provider
	updatedAt time.Time
}

func NewSandboxProviderResolver(s *store.Store, manager *sandbox.Manager) *SandboxProviderResolver {
	return &SandboxProviderResolver{
		store:     s,
		manager:   manager,
		factories: map[string]SandboxProviderFactory{},
		cache:     map[string]cachedSandboxProvider{},
	}
}

func (r *SandboxProviderResolver) RegisterFactory(providerType string, factory SandboxProviderFactory) {
	if factory == nil {
		delete(r.factories, providerType)
		return
	}
	r.factories[providerType] = factory
}

func (r *SandboxProviderResolver) ResolveForSession(ctx context.Context, sessionID string) (sandbox.Provider, error) {
	// Include soft-deleted sessions so deferred sandbox cleanup can still route to
	// the right provider after the session has been removed from the database.
	session, err := r.store.GetSessionByIDIncludingDeleted(ctx, sessionID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		// Session is completely gone (not even soft-deleted) — fall back to the
		// default provider for best-effort sandbox cleanup.
		return r.manager.GetProvider(r.manager.DefaultProviderName())
	}

	if session.SandboxProviderID != "" {
		return r.ResolveProjectProvider(ctx, session.ProjectID, session.SandboxProviderID)
	}
	return r.ResolveProjectDefault(ctx, session.ProjectID)
}

func (r *SandboxProviderResolver) ResolveProjectDefault(ctx context.Context, projectID string) (sandbox.Provider, error) {
	project, err := r.store.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	if project.DefaultSandboxProviderID != "" {
		return r.ResolveProjectProvider(ctx, projectID, project.DefaultSandboxProviderID)
	}

	defaultProvider := r.manager.DefaultProviderName()
	disabled, err := r.store.IsSandboxProviderDisabled(ctx, projectID, defaultProvider)
	if err != nil {
		return nil, err
	}
	if !disabled {
		return r.manager.GetProvider(defaultProvider)
	}
	for providerName, status := range r.manager.ListProviderStatuses() {
		if !status.Available {
			continue
		}
		disabled, err := r.store.IsSandboxProviderDisabled(ctx, projectID, providerName)
		if err != nil {
			return nil, err
		}
		if !disabled {
			return r.manager.GetProvider(providerName)
		}
	}
	return nil, fmt.Errorf("all built-in sandbox providers are disabled for project %q", projectID)
}

func (r *SandboxProviderResolver) ResolveProjectProvider(ctx context.Context, projectID, providerID string) (sandbox.Provider, error) {
	if _, ok := r.manager.GetProviderStatus(providerID); ok {
		disabled, err := r.store.IsSandboxProviderDisabled(ctx, projectID, providerID)
		if err != nil {
			return nil, fmt.Errorf("failed to check sandbox provider status: %w", err)
		}
		if disabled {
			return nil, fmt.Errorf("sandbox provider %q is disabled for project %q", providerID, projectID)
		}
		return r.manager.GetProvider(providerID)
	}

	instance, err := r.store.GetSandboxProviderInstance(ctx, projectID, providerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("sandbox provider instance %q not found", providerID)
		}
		return nil, fmt.Errorf("failed to get sandbox provider instance: %w", err)
	}
	if instance.Disabled {
		return nil, fmt.Errorf("sandbox provider instance %q is disabled for project %q", providerID, projectID)
	}
	if factory := r.factories[instance.Type]; factory != nil {
		return r.cachedProvider(ctx, instance, factory)
	}
	return r.manager.GetProvider(instance.Type)
}

func (r *SandboxProviderResolver) cachedProvider(ctx context.Context, instance *model.SandboxProviderInstance, factory SandboxProviderFactory) (sandbox.Provider, error) {
	r.mu.Lock()
	if cached, ok := r.cache[instance.ID]; ok && cached.updatedAt.Equal(instance.UpdatedAt) {
		provider := cached.provider
		r.mu.Unlock()
		return provider, nil
	}

	provider, err := factory(ctx, instance)
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}

	r.cache[instance.ID] = cachedSandboxProvider{
		provider:  provider,
		updatedAt: instance.UpdatedAt,
	}
	r.mu.Unlock()
	return provider, nil
}
