package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

// PlatformDefaultProvider returns the default sandbox provider for the current OS.
// On macOS (darwin), the default is "vz" (Virtualization.framework).
// On Windows, the default is "hcs" (Host Compute System).
// On all other platforms, the default is "docker".
func PlatformDefaultProvider() string {
	if runtime.GOOS == "darwin" {
		return "vz"
	}
	if runtime.GOOS == "windows" {
		return "hcs"
	}
	return "docker"
}

// ProviderManager manages multiple sandbox providers and routes requests to the appropriate one.
type ProviderManager struct {
	mu                 sync.RWMutex
	store              *store.Store
	providers          map[string]Provider
	instanceProviders  map[string]Provider
	definitions        map[string]ProviderDefinition
	defaultProvider    string // Default provider name
	factories          map[string]ProviderFactory
	cache              map[string]cachedProvider
	nextProviderSubID  int
	providerUpdateSubs map[int]chan struct{}
}

// ProviderFactory builds a runtime provider from a project-scoped saved
// provider instance.
type ProviderFactory func(ctx context.Context, instance *model.SandboxProviderInstance) (Provider, error)

type cachedProvider struct {
	provider  Provider
	updatedAt time.Time
}

// NewProviderManager creates a new sandbox provider manager.
// The default provider is determined by the platform: "vz" on macOS, "docker" elsewhere.
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers:          make(map[string]Provider),
		instanceProviders:  make(map[string]Provider),
		definitions:        make(map[string]ProviderDefinition),
		defaultProvider:    PlatformDefaultProvider(),
		factories:          make(map[string]ProviderFactory),
		cache:              make(map[string]cachedProvider),
		providerUpdateSubs: make(map[int]chan struct{}),
	}
}

// SetStore configures the store used to resolve project-scoped provider
// instances for sessions.
func (m *ProviderManager) SetStore(s *store.Store) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = s
}

// RegisterProvider registers a provider with the given name.
func (m *ProviderManager) RegisterProvider(name string, provider Provider) {
	m.mu.Lock()
	m.providers[name] = provider
	if dp, ok := provider.(DefinitionProvider); ok {
		m.definitions[name] = dp.Definition()
	}
	m.mu.Unlock()

	m.notifyProviderUpdate()
}

// RegisterFactory registers a factory for project-scoped provider instances of
// the given type.
func (m *ProviderManager) RegisterFactory(providerType string, factory ProviderFactory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if factory == nil {
		delete(m.factories, providerType)
		return
	}
	m.factories[providerType] = factory
}

// RegisterProviderDefinition registers configurable provider type metadata
// without registering a process-wide provider instance.
func (m *ProviderManager) RegisterProviderDefinition(name string, definition ProviderDefinition) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.definitions[name] = definition
}

// SetDefault sets the default provider name.
func (m *ProviderManager) SetDefault(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultProvider = name
}

// DefaultProviderName returns the name of the current default provider.
func (m *ProviderManager) DefaultProviderName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultProvider
}

// EnsureDefaultAvailable checks that the default provider is registered.
// If not, it falls back to any registered provider. Returns false if no providers are registered.
func (m *ProviderManager) EnsureDefaultAvailable() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.providers[m.defaultProvider]; ok {
		return true
	}
	// Fall back to any registered provider
	for name := range m.providers {
		m.defaultProvider = name
		return true
	}
	return false
}

// GetProvider returns the provider with the given name.
func (m *ProviderManager) GetProvider(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if name == "" {
		name = m.defaultProvider
	}

	provider, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}

	return provider, nil
}

// GetDefault returns the default provider.
func (m *ProviderManager) GetDefault() Provider {
	provider, _ := m.GetProvider(m.defaultProvider)
	return provider
}

// ListProviders returns the names of all registered providers.
func (m *ProviderManager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var names []string
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// GetProviderStatus returns the status of a specific provider.
// If the provider implements StatusProvider, its Status() is called.
// Otherwise, a default "ready" status is returned.
func (m *ProviderManager) GetProviderStatus(name string) (ProviderStatus, bool) {
	m.mu.RLock()
	provider, ok := m.providers[name]
	m.mu.RUnlock()
	if !ok {
		return ProviderStatus{}, false
	}

	status := ProviderStatus{
		Available: true,
		State:     "ready",
	}
	if sp, ok := provider.(StatusProvider); ok {
		status = sp.Status()
	}

	_, status.SupportsResources = provider.(ProviderResourceManager)
	_, status.SupportsInspection = provider.(ProjectInspectionManager)
	_, status.SupportsClearCache = provider.(ProjectCacheManager)

	return status, true
}

// GetProviderDefinition returns the driver metadata for a registered provider.
func (m *ProviderManager) GetProviderDefinition(name string) (ProviderDefinition, bool) {
	m.mu.RLock()
	if definition, ok := m.definitions[name]; ok {
		m.mu.RUnlock()
		return definition, true
	}
	provider, ok := m.providers[name]
	m.mu.RUnlock()
	if !ok {
		return ProviderDefinition{}, false
	}
	if dp, ok := provider.(DefinitionProvider); ok {
		return dp.Definition(), true
	}
	return ProviderDefinition{
		Name:        name,
		Description: "Built-in " + name + " sandbox driver",
	}, true
}

// ListProviderDefinitions returns all registered provider type definitions.
func (m *ProviderManager) ListProviderDefinitions() map[string]ProviderDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()
	definitions := make(map[string]ProviderDefinition, len(m.definitions)+len(m.providers))
	maps.Copy(definitions, m.definitions)
	for name, provider := range m.providers {
		if _, ok := definitions[name]; ok {
			continue
		}
		if dp, ok := provider.(DefinitionProvider); ok {
			definitions[name] = dp.Definition()
			continue
		}
		definitions[name] = ProviderDefinition{
			Name:        name,
			Description: "Built-in " + name + " sandbox driver",
		}
	}
	return definitions
}

// ListProviderStatuses returns the status of all registered providers.
func (m *ProviderManager) ListProviderStatuses() map[string]ProviderStatus {
	providers := m.snapshotProviders()
	statuses := make(map[string]ProviderStatus, len(providers))
	for name, provider := range providers {
		status := ProviderStatus{
			Available: true,
			State:     "ready",
		}
		if sp, ok := provider.(StatusProvider); ok {
			status = sp.Status()
		}
		_, status.SupportsResources = provider.(ProviderResourceManager)
		_, status.SupportsInspection = provider.(ProjectInspectionManager)
		_, status.SupportsClearCache = provider.(ProjectCacheManager)
		statuses[name] = status
	}
	return statuses
}

// Shutdown gracefully shuts down all providers that support cleanup.
// Providers implementing a Close() method will have it called.
func (m *ProviderManager) Shutdown() {
	for name, provider := range m.snapshotWatchProviders() {
		// Check if provider implements Close() method using type assertion
		if closer, ok := provider.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				// Log error but continue shutting down other providers
				_ = err // Logging happens inside provider's Close()
			}
		}
		_ = name // Keep for potential logging
	}
}

func (m *ProviderManager) snapshotProviders() map[string]Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make(map[string]Provider, len(m.providers))
	maps.Copy(providers, m.providers)
	return providers
}

func (m *ProviderManager) snapshotWatchProviders() map[string]Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make(map[string]Provider, len(m.providers)+len(m.instanceProviders))
	maps.Copy(providers, m.providers)
	maps.Copy(providers, m.instanceProviders)
	return providers
}

// ResolveForSession returns the provider that should manage a session.
func (m *ProviderManager) ResolveForSession(ctx context.Context, sessionID string) (Provider, error) {
	m.mu.RLock()
	st := m.store
	m.mu.RUnlock()
	if st == nil {
		return nil, fmt.Errorf("sandbox provider manager store unavailable")
	}

	// Include soft-deleted sessions so deferred sandbox cleanup can still route to
	// the right provider after the session has been removed from the database.
	session, err := st.GetSessionByIDIncludingDeleted(ctx, sessionID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
		// Session is completely gone (not even soft-deleted) — fall back to the
		// default provider for best-effort sandbox cleanup.
		return m.GetProvider(m.DefaultProviderName())
	}

	providerID, err := st.GetSessionSandboxProviderIDIncludingDeleted(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session sandbox provider ID: %w", err)
	}
	if !providerID.Valid {
		return m.GetProvider(m.DefaultProviderName())
	}
	if providerID := strings.TrimSpace(providerID.String); providerID != "" {
		return m.resolveProjectProvider(ctx, st, session.ProjectID, providerID)
	}
	return m.resolveProjectDefault(ctx, st, session.ProjectID)
}

func (m *ProviderManager) resolveProjectDefault(ctx context.Context, st *store.Store, projectID string) (Provider, error) {
	project, err := st.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	if project.DefaultSandboxProviderID != "" {
		return m.resolveProjectProvider(ctx, st, projectID, project.DefaultSandboxProviderID)
	}

	defaultProvider := m.DefaultProviderName()
	disabled, err := st.IsSandboxProviderDisabled(ctx, projectID, defaultProvider)
	if err != nil {
		return nil, err
	}
	if !disabled {
		return m.GetProvider(defaultProvider)
	}
	for providerName, status := range m.ListProviderStatuses() {
		if !status.Available {
			continue
		}
		disabled, err := st.IsSandboxProviderDisabled(ctx, projectID, providerName)
		if err != nil {
			return nil, err
		}
		if !disabled {
			return m.GetProvider(providerName)
		}
	}
	return nil, fmt.Errorf("all built-in sandbox providers are disabled for project %q", projectID)
}

func (m *ProviderManager) resolveProjectProvider(ctx context.Context, st *store.Store, projectID, providerID string) (Provider, error) {
	if _, ok := m.GetProviderStatus(providerID); ok {
		disabled, err := st.IsSandboxProviderDisabled(ctx, projectID, providerID)
		if err != nil {
			return nil, fmt.Errorf("failed to check sandbox provider status: %w", err)
		}
		if disabled {
			return nil, fmt.Errorf("sandbox provider %q is disabled for project %q", providerID, projectID)
		}
		return m.GetProvider(providerID)
	}

	instance, err := st.GetSandboxProviderInstance(ctx, projectID, providerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("sandbox provider instance %q not found", providerID)
		}
		return nil, fmt.Errorf("failed to get sandbox provider instance: %w", err)
	}
	if instance.Disabled {
		return nil, fmt.Errorf("sandbox provider instance %q is disabled for project %q", providerID, projectID)
	}

	m.mu.RLock()
	factory := m.factories[instance.Type]
	m.mu.RUnlock()
	if factory != nil {
		return m.cachedProvider(ctx, instance, factory)
	}
	return m.GetProvider(instance.Type)
}

func (m *ProviderManager) cachedProvider(ctx context.Context, instance *model.SandboxProviderInstance, factory ProviderFactory) (Provider, error) {
	m.mu.RLock()
	if cached, ok := m.cache[instance.ID]; ok && cached.updatedAt.Equal(instance.UpdatedAt) {
		provider := cached.provider
		m.mu.RUnlock()
		return provider, nil
	}
	m.mu.RUnlock()

	provider, err := factory(ctx, instance)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	if cached, ok := m.cache[instance.ID]; ok && cached.updatedAt.Equal(instance.UpdatedAt) {
		provider := cached.provider
		m.mu.Unlock()
		return provider, nil
	}
	m.cache[instance.ID] = cachedProvider{
		provider:  provider,
		updatedAt: instance.UpdatedAt,
	}
	m.instanceProviders[instance.ID] = provider
	m.mu.Unlock()

	m.notifyProviderUpdate()
	return provider, nil
}

func (m *ProviderManager) notifyProviderUpdate() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.providerUpdateSubs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (m *ProviderManager) subscribeProviderUpdates(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{}, 1)

	m.mu.Lock()
	id := m.nextProviderSubID
	m.nextProviderSubID++
	m.providerUpdateSubs[id] = ch
	m.mu.Unlock()

	context.AfterFunc(ctx, func() {
		m.mu.Lock()
		if _, ok := m.providerUpdateSubs[id]; ok {
			delete(m.providerUpdateSubs, id)
			close(ch)
		}
		m.mu.Unlock()
	})

	return ch
}

// ProviderProxy implements the Provider interface and routes to the appropriate provider.
// This is used when we need a single Provider interface but want to support multiple backends.
type ProviderProxy struct {
	manager *ProviderManager
}

// NewProviderProxy creates a new provider proxy.
func NewProviderProxy(manager *ProviderManager) *ProviderProxy {
	return &ProviderProxy{
		manager: manager,
	}
}

// ListProviders returns the names of all available providers.
func (p *ProviderProxy) ListProviders() []string {
	return p.manager.ListProviders()
}

// ImageExists checks if the image exists in the default provider.
func (p *ProviderProxy) ImageExists(ctx context.Context) bool {
	provider := p.manager.GetDefault()
	if provider == nil {
		return false
	}
	return provider.ImageExists(ctx)
}

// Image returns the image name from the default provider.
func (p *ProviderProxy) Image() string {
	provider := p.manager.GetDefault()
	if provider == nil {
		return ""
	}
	return provider.Image()
}

// DefaultProvider returns the current default provider.
func (p *ProviderProxy) DefaultProvider() Provider {
	return p.manager.GetDefault()
}

func (p *ProviderProxy) providerForSession(ctx context.Context, sessionID string) (Provider, error) {
	provider, err := p.manager.ResolveForSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider for session: %w", err)
	}
	return provider, nil
}

// PrepareState returns provider state using the provider determined by providerGetter.
func (p *ProviderProxy) PrepareState(ctx context.Context, sessionID string, opts CreateOptions) ([]byte, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return provider.PrepareState(ctx, sessionID, opts)
}

// Create creates a sandbox using the provider determined by providerGetter.
func (p *ProviderProxy) Create(ctx context.Context, state []byte, sessionID string, opts CreateOptions) (*Sandbox, []byte, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}

	return provider.Create(ctx, state, sessionID, opts)
}

// Start starts a sandbox using the provider determined by providerGetter.
func (p *ProviderProxy) Start(ctx context.Context, state []byte, sessionID string) ([]byte, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return state, err
	}

	return provider.Start(ctx, state, sessionID)
}

// Stop stops a sandbox using the provider determined by providerGetter.
func (p *ProviderProxy) Stop(ctx context.Context, state []byte, sessionID string, timeout time.Duration) ([]byte, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return state, err
	}

	return provider.Stop(ctx, state, sessionID, timeout)
}

// Remove removes a sandbox using the provider determined by providerGetter.
func (p *ProviderProxy) Remove(ctx context.Context, state []byte, sessionID string, opts ...RemoveOption) ([]byte, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return state, err
	}

	return provider.Remove(ctx, state, sessionID, opts...)
}

// Get gets a sandbox using the provider determined by providerGetter.
func (p *ProviderProxy) Get(ctx context.Context, state []byte, sessionID string) (*Sandbox, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return provider.Get(ctx, state, sessionID)
}

// GetSecret gets the secret using the provider determined by providerGetter.
func (p *ProviderProxy) GetSecret(ctx context.Context, state []byte, sessionID string) (string, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return "", err
	}

	return provider.GetSecret(ctx, state, sessionID)
}

// List lists all sandboxes across all providers.
func (p *ProviderProxy) List(ctx context.Context) ([]*Sandbox, error) {
	var allSandboxes []*Sandbox

	for _, provider := range p.manager.snapshotWatchProviders() {
		sandboxes, err := provider.List(ctx)
		if err != nil {
			continue // Skip providers that error
		}
		allSandboxes = append(allSandboxes, sandboxes...)
	}

	return allSandboxes, nil
}

// AcquireHTTPClient returns a leased HTTP client using the provider determined by providerGetter.
func (p *ProviderProxy) AcquireHTTPClient(ctx context.Context, state []byte, sessionID string) (*HTTPClientLease, error) {
	provider, err := p.providerForSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return provider.AcquireHTTPClient(ctx, state, sessionID)
}

// Watch watches all providers and merges events.
func (p *ProviderProxy) Watch(ctx context.Context) (<-chan StateEvent, error) {
	merged := make(chan StateEvent, 100)
	go func() {
		watchCtx, cancel := context.WithCancel(ctx)
		var wg sync.WaitGroup
		defer func() {
			cancel()
			wg.Wait()
			close(merged)
		}()

		providerUpdates := p.manager.subscribeProviderUpdates(watchCtx)
		watched := map[string]struct{}{}
		startNewProviderWatches := func() {
			for name, provider := range p.manager.snapshotWatchProviders() {
				if _, ok := watched[name]; ok {
					continue
				}
				ch, err := provider.Watch(watchCtx)
				if err != nil {
					continue // Skip providers that can't be watched.
				}
				watched[name] = struct{}{}

				wg.Go(func() {
					for event := range ch {
						select {
						case merged <- event:
						case <-watchCtx.Done():
							return
						}
					}
				})
			}
		}

		startNewProviderWatches()

		for {
			select {
			case _, ok := <-providerUpdates:
				if !ok {
					return
				}
				startNewProviderWatches()
			case <-watchCtx.Done():
				return
			}
		}
	}()

	return merged, nil
}

// Reconcile delegates to all providers.
func (p *ProviderProxy) Reconcile(ctx context.Context) error {
	for name, provider := range p.manager.snapshotWatchProviders() {
		if err := provider.Reconcile(ctx); err != nil {
			log.Printf("Warning: Failed to reconcile provider %s: %v", name, err)
		}
	}
	return nil
}

// RemoveProject delegates to all providers.
func (p *ProviderProxy) RemoveProject(ctx context.Context, projectID string) error {
	for name, provider := range p.manager.snapshotWatchProviders() {
		if err := provider.RemoveProject(ctx, projectID); err != nil {
			log.Printf("Warning: Failed to remove resources for provider %s: %v", name, err)
		}
	}
	return nil
}

// GetProviderResourceInfo delegates to the default provider when supported.
func (p *ProviderProxy) GetProviderResourceInfo(ctx context.Context, projectID string) (*ProviderResourceInfo, error) {
	provider := p.manager.GetDefault()
	if provider == nil {
		return nil, fmt.Errorf("no sandbox provider available")
	}

	resourceManager, ok := provider.(ProviderResourceManager)
	if !ok {
		return nil, ErrProviderResourcesUnsupported
	}

	return resourceManager.GetProviderResourceInfo(ctx, projectID)
}

// ApplyProviderResourceUpdate delegates to the default provider when supported.
func (p *ProviderProxy) ApplyProviderResourceUpdate(ctx context.Context, projectID string, req UpdateProviderResourcesRequest) error {
	provider := p.manager.GetDefault()
	if provider == nil {
		return fmt.Errorf("no sandbox provider available")
	}

	resourceManager, ok := provider.(ProviderResourceManager)
	if !ok {
		return ErrProviderResourcesUnsupported
	}

	return resourceManager.ApplyProviderResourceUpdate(ctx, projectID, req)
}

// GetProjectInspectionInfo delegates to the default provider when supported.
func (p *ProviderProxy) GetProjectInspectionInfo(ctx context.Context, projectID string) (*ProjectInspectionInfo, error) {
	provider := p.manager.GetDefault()
	if provider == nil {
		return nil, fmt.Errorf("no sandbox provider available")
	}

	inspectionManager, ok := provider.(ProjectInspectionManager)
	if !ok {
		return nil, ErrProjectInspectionUnsupported
	}

	return inspectionManager.GetProjectInspectionInfo(ctx, projectID)
}

// AttachProjectInspection delegates to the default provider when supported.
func (p *ProviderProxy) AttachProjectInspection(ctx context.Context, projectID string, opts AttachOptions) (PTY, error) {
	provider := p.manager.GetDefault()
	if provider == nil {
		return nil, fmt.Errorf("no sandbox provider available")
	}

	inspectionManager, ok := provider.(ProjectInspectionManager)
	if !ok {
		return nil, ErrProjectInspectionUnsupported
	}

	return inspectionManager.AttachProjectInspection(ctx, projectID, opts)
}

// ClearCache delegates to the default provider when supported.
func (p *ProviderProxy) ClearCache(ctx context.Context, projectID string) error {
	provider := p.manager.GetDefault()
	if provider == nil {
		return fmt.Errorf("no sandbox provider available")
	}

	cacheManager, ok := provider.(ProjectCacheManager)
	if !ok {
		return ErrProjectCacheUnsupported
	}

	return cacheManager.ClearCache(ctx, projectID)
}
