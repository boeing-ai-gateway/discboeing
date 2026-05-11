package sandbox

import (
	"context"
	"fmt"
	"log"
	"maps"
	"runtime"
	"time"
)

// PlatformDefaultProvider returns the default sandbox provider for the current OS.
// On macOS (darwin), the default is "vz" (Virtualization.framework).
// On Windows, the default is "wsl".
// On all other platforms, the default is "docker".
func PlatformDefaultProvider() string {
	if runtime.GOOS == "darwin" {
		return "vz"
	}
	if runtime.GOOS == "windows" {
		return "wsl"
	}
	return "docker"
}

// Manager manages multiple sandbox providers and routes requests to the appropriate one.
type Manager struct {
	providers       map[string]Provider
	definitions     map[string]ProviderDefinition
	defaultProvider string // Default provider name
}

// NewManager creates a new sandbox provider manager.
// The default provider is determined by the platform: "vz" on macOS, "docker" elsewhere.
func NewManager() *Manager {
	return &Manager{
		providers:       make(map[string]Provider),
		definitions:     make(map[string]ProviderDefinition),
		defaultProvider: PlatformDefaultProvider(),
	}
}

// RegisterProvider registers a provider with the given name.
func (m *Manager) RegisterProvider(name string, provider Provider) {
	m.providers[name] = provider
	if dp, ok := provider.(DefinitionProvider); ok {
		m.definitions[name] = dp.Definition()
	}
}

// RegisterProviderDefinition registers configurable provider type metadata
// without registering a process-wide provider instance.
func (m *Manager) RegisterProviderDefinition(name string, definition ProviderDefinition) {
	m.definitions[name] = definition
}

// SetDefault sets the default provider name.
func (m *Manager) SetDefault(name string) {
	m.defaultProvider = name
}

// DefaultProviderName returns the name of the current default provider.
func (m *Manager) DefaultProviderName() string {
	return m.defaultProvider
}

// EnsureDefaultAvailable checks that the default provider is registered.
// If not, it falls back to any registered provider. Returns false if no providers are registered.
func (m *Manager) EnsureDefaultAvailable() bool {
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
func (m *Manager) GetProvider(name string) (Provider, error) {
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
func (m *Manager) GetDefault() Provider {
	provider, _ := m.GetProvider(m.defaultProvider)
	return provider
}

// ListProviders returns the names of all registered providers.
func (m *Manager) ListProviders() []string {
	var names []string
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// GetProviderStatus returns the status of a specific provider.
// If the provider implements StatusProvider, its Status() is called.
// Otherwise, a default "ready" status is returned.
func (m *Manager) GetProviderStatus(name string) (ProviderStatus, bool) {
	provider, ok := m.providers[name]
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

	_, status.SupportsResources = provider.(ProjectResourceManager)
	_, status.SupportsInspection = provider.(ProjectInspectionManager)
	_, status.SupportsClearCache = provider.(ProjectCacheManager)

	return status, true
}

// GetProviderDefinition returns the driver metadata for a registered provider.
func (m *Manager) GetProviderDefinition(name string) (ProviderDefinition, bool) {
	if definition, ok := m.definitions[name]; ok {
		return definition, true
	}
	provider, ok := m.providers[name]
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
func (m *Manager) ListProviderDefinitions() map[string]ProviderDefinition {
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
func (m *Manager) ListProviderStatuses() map[string]ProviderStatus {
	statuses := make(map[string]ProviderStatus, len(m.providers))
	for name := range m.providers {
		status, _ := m.GetProviderStatus(name)
		statuses[name] = status
	}
	return statuses
}

// Shutdown gracefully shuts down all providers that support cleanup.
// Providers implementing a Close() method will have it called.
func (m *Manager) Shutdown() {
	for name, provider := range m.providers {
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

// ProviderProxy implements the Provider interface and routes to the appropriate provider.
// This is used when we need a single Provider interface but want to support multiple backends.
type ProviderProxy struct {
	manager        *Manager
	providerGetter func(ctx context.Context, sessionID string) (Provider, error)
}

// NewProviderProxy creates a new provider proxy that uses providerGetter to determine
// which provider to use for each session.
func NewProviderProxy(manager *Manager, providerGetter func(ctx context.Context, sessionID string) (Provider, error)) *ProviderProxy {
	return &ProviderProxy{
		manager:        manager,
		providerGetter: providerGetter,
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
	provider, err := p.providerGetter(ctx, sessionID)
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

	for _, provider := range p.manager.providers {
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

	// Start watching all providers
	var channels []<-chan StateEvent
	for _, provider := range p.manager.providers {
		ch, err := provider.Watch(ctx)
		if err != nil {
			continue // Skip providers that can't be watched
		}
		channels = append(channels, ch)
	}

	// Merge all channels
	go func() {
		defer close(merged)

		// Use a WaitGroup to wait for all goroutines
		cases := make([]<-chan StateEvent, len(channels))
		copy(cases, channels)

		for _, ch := range cases {
			go func(c <-chan StateEvent) {
				for event := range c {
					select {
					case merged <- event:
					case <-ctx.Done():
						return
					}
				}
			}(ch)
		}

		// Wait for context cancellation
		<-ctx.Done()
	}()

	return merged, nil
}

// Reconcile delegates to all providers.
func (p *ProviderProxy) Reconcile(ctx context.Context) error {
	for name, provider := range p.manager.providers {
		if err := provider.Reconcile(ctx); err != nil {
			log.Printf("Warning: Failed to reconcile provider %s: %v", name, err)
		}
	}
	return nil
}

// RemoveProject delegates to all providers.
func (p *ProviderProxy) RemoveProject(ctx context.Context, projectID string) error {
	for name, provider := range p.manager.providers {
		if err := provider.RemoveProject(ctx, projectID); err != nil {
			log.Printf("Warning: Failed to remove project resources for provider %s: %v", name, err)
		}
	}
	return nil
}

// GetProjectResourceInfo delegates to the default provider when supported.
func (p *ProviderProxy) GetProjectResourceInfo(ctx context.Context, projectID string) (*ProjectResourceInfo, error) {
	provider := p.manager.GetDefault()
	if provider == nil {
		return nil, fmt.Errorf("no sandbox provider available")
	}

	resourceManager, ok := provider.(ProjectResourceManager)
	if !ok {
		return nil, ErrProjectResourcesUnsupported
	}

	return resourceManager.GetProjectResourceInfo(ctx, projectID)
}

// ApplyProjectResourceUpdate delegates to the default provider when supported.
func (p *ProviderProxy) ApplyProjectResourceUpdate(ctx context.Context, projectID string, req UpdateProjectResourcesRequest) error {
	provider := p.manager.GetDefault()
	if provider == nil {
		return fmt.Errorf("no sandbox provider available")
	}

	resourceManager, ok := provider.(ProjectResourceManager)
	if !ok {
		return ErrProjectResourcesUnsupported
	}

	return resourceManager.ApplyProjectResourceUpdate(ctx, projectID, req)
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
