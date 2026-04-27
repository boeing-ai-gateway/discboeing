//go:build windows

package wsl

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/docker"
	"github.com/obot-platform/discobot/server/internal/startup"
)

// SessionProjectResolver maps session IDs to project IDs.
type SessionProjectResolver func(ctx context.Context, sessionID string) (projectID string, err error)

// Provider is the Windows WSL sandbox provider. It translates Windows paths,
// resolves bridge configuration, and delegates sandbox operations to an inner
// Docker provider once the managed WSL runtime exposes a reachable bridge.
type Provider struct {
	cfg                    *config.Config
	manager                *Manager
	sessionProjectResolver SessionProjectResolver
	systemManager          *startup.SystemManager

	mu             sync.RWMutex
	dockerProvider *docker.Provider
	dockerHost     string

	// idleMonitor shuts down the managed distro after it has had no running
	// Discobot sandboxes for the configured idle period.
	idleMonitor *sandbox.IdleRuntimeMonitor
}

// NewProvider creates a new WSL-backed sandbox provider.
func NewProvider(cfg *config.Config, resolver SessionProjectResolver, systemManager *startup.SystemManager) (*Provider, error) {
	if resolver == nil {
		return nil, fmt.Errorf("sessionProjectResolver is required")
	}

	provider := &Provider{
		cfg:                    cfg,
		manager:                NewManager(cfg),
		sessionProjectResolver: resolver,
		systemManager:          systemManager,
	}
	if cfg.WSLIdleTimeout > 0 {
		provider.idleMonitor = sandbox.NewIdleRuntimeMonitor(provider, "wsl", cfg.WSLIdleTimeout, cfg.IdleCheckInterval)
		provider.idleMonitor.Start(context.Background())
	}
	return provider, nil
}

// Status reports the WSL provider status.
func (p *Provider) Status() sandbox.ProviderStatus {
	return p.manager.Status()
}

// ImageExists reports whether the configured sandbox image is locally available
// inside the managed WSL Docker daemon.
func (p *Provider) ImageExists(ctx context.Context) bool {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil || !runtimeInfo.BridgeReady {
		return false
	}

	dockerProvider, err := p.dockerProviderForRuntime(runtimeInfo)
	if err != nil {
		return false
	}
	return dockerProvider.ImageExists(ctx)
}

// Image returns the configured session sandbox image.
func (p *Provider) Image() string {
	return p.cfg.SandboxImage
}

// Create creates a new sandbox through the inner Docker provider once the WSL
// bridge is available.
func (p *Provider) Create(ctx context.Context, sessionID string, opts sandbox.CreateOptions) (*sandbox.Sandbox, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	opts, err = p.translateCreateOptions(opts)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.Create(ctx, sessionID, opts)
}

// Start starts a sandbox through the inner Docker provider.
func (p *Provider) Start(ctx context.Context, sessionID string) error {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return err
	}
	return dockerProvider.Start(ctx, sessionID)
}

// Stop stops a sandbox through the inner Docker provider.
func (p *Provider) Stop(ctx context.Context, sessionID string, timeout time.Duration) error {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return err
	}
	return dockerProvider.Stop(ctx, sessionID, timeout)
}

// Remove removes a sandbox through the inner Docker provider.
func (p *Provider) Remove(ctx context.Context, sessionID string, opts ...sandbox.RemoveOption) error {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return err
	}
	return dockerProvider.Remove(ctx, sessionID, opts...)
}

// Get returns sandbox state through the inner Docker provider.
func (p *Provider) Get(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.Get(ctx, sessionID)
}

// GetSecret returns sandbox secrets through the inner Docker provider.
func (p *Provider) GetSecret(ctx context.Context, sessionID string) (string, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return "", err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return "", err
	}
	return dockerProvider.GetSecret(ctx, sessionID)
}

// List lists sandboxes through the inner Docker provider.
func (p *Provider) List(ctx context.Context) ([]*sandbox.Sandbox, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.List(ctx)
}

// Exec runs a command through the inner Docker provider.
func (p *Provider) Exec(ctx context.Context, sessionID string, cmd []string, opts sandbox.ExecOptions) (*sandbox.ExecResult, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.Exec(ctx, sessionID, cmd, opts)
}

// Attach creates a PTY through the inner Docker provider.
func (p *Provider) Attach(ctx context.Context, sessionID string, opts sandbox.AttachOptions) (sandbox.PTY, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.Attach(ctx, sessionID, opts)
}

// ExecStream creates a bidirectional stream through the inner Docker provider.
func (p *Provider) ExecStream(ctx context.Context, sessionID string, cmd []string, opts sandbox.ExecStreamOptions) (sandbox.Stream, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.ExecStream(ctx, sessionID, cmd, opts)
}

// AcquireHTTPClient returns a leased sandbox HTTP client through the inner Docker provider.
func (p *Provider) AcquireHTTPClient(ctx context.Context, sessionID string) (*sandbox.HTTPClientLease, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.AcquireHTTPClient(ctx, sessionID)
}

// Watch delegates event watching to the inner Docker provider.
func (p *Provider) Watch(ctx context.Context) (<-chan sandbox.StateEvent, error) {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return nil, err
	}
	return dockerProvider.Watch(ctx)
}

// Reconcile delegates provider reconciliation to the inner Docker provider.
func (p *Provider) Reconcile(ctx context.Context) error {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return err
	}
	return dockerProvider.Reconcile(ctx)
}

// RemoveProject delegates project cleanup to the inner Docker provider.
func (p *Provider) RemoveProject(ctx context.Context, projectID string) error {
	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return err
	}

	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return err
	}
	return dockerProvider.RemoveProject(ctx, projectID)
}

// Close closes the inner Docker provider and stops the manager-owned runtime.
func (p *Provider) Close() error {
	if p.idleMonitor != nil {
		_ = p.idleMonitor.Stop(context.Background())
	}

	p.mu.Lock()
	dockerProvider := p.dockerProvider
	p.dockerProvider = nil
	p.dockerHost = ""
	p.mu.Unlock()

	if dockerProvider != nil {
		if err := dockerProvider.Close(); err != nil {
			return err
		}
	}
	return p.manager.Stop(context.Background())
}

// ListRuntimeIDs implements sandbox.IdleRuntimeController for the single managed
// WSL distro runtime.
func (p *Provider) ListRuntimeIDs(ctx context.Context) ([]string, error) {
	distro, found, err := p.manager.probeDistro(ctx)
	if err != nil {
		return nil, err
	}
	if !found || !strings.EqualFold(distro.State, "Running") {
		return nil, nil
	}
	return []string{p.cfg.WSLDistroName}, nil
}

// RunningSandboxCount implements sandbox.IdleRuntimeController for the managed
// WSL distro runtime.
func (p *Provider) RunningSandboxCount(ctx context.Context, runtimeID string) (int, error) {
	if runtimeID != p.cfg.WSLDistroName {
		return 0, nil
	}

	distro, found, err := p.manager.probeDistro(ctx)
	if err != nil {
		return 0, err
	}
	if !found || !strings.EqualFold(distro.State, "Running") {
		return 0, nil
	}

	runtimeInfo, err := p.manager.EnsureRunning(ctx)
	if err != nil {
		return 0, err
	}
	dockerProvider, err := p.requireDockerProvider(runtimeInfo)
	if err != nil {
		return 0, err
	}

	sandboxes, err := dockerProvider.List(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, sb := range sandboxes {
		if sb.Status == sandbox.StatusRunning {
			count++
		}
	}
	return count, nil
}

// StopRuntime implements sandbox.IdleRuntimeController for the managed WSL distro.
func (p *Provider) StopRuntime(ctx context.Context, runtimeID string) error {
	if runtimeID != p.cfg.WSLDistroName {
		return nil
	}

	p.mu.Lock()
	dockerProvider := p.dockerProvider
	p.dockerProvider = nil
	p.dockerHost = ""
	p.mu.Unlock()

	if dockerProvider != nil {
		_ = dockerProvider.Close()
	}
	return p.manager.Stop(ctx)
}

func (p *Provider) translateCreateOptions(opts sandbox.CreateOptions) (sandbox.CreateOptions, error) {
	if opts.WorkspacePath == "" {
		return opts, nil
	}

	translatedPath, err := TranslatePath(opts.WorkspacePath)
	if err != nil {
		return sandbox.CreateOptions{}, fmt.Errorf("translate workspace path %q: %w", opts.WorkspacePath, err)
	}
	opts.WorkspacePath = translatedPath
	return opts, nil
}

func (p *Provider) requireDockerProvider(runtimeInfo *RuntimeInfo) (*docker.Provider, error) {
	if runtimeInfo == nil {
		return nil, fmt.Errorf("missing WSL runtime info")
	}
	if !runtimeInfo.BridgeReady {
		return nil, bridgeNotReadyError(runtimeInfo)
	}
	return p.dockerProviderForRuntime(runtimeInfo)
}

func (p *Provider) dockerProviderForRuntime(runtimeInfo *RuntimeInfo) (*docker.Provider, error) {
	if runtimeInfo == nil {
		return nil, fmt.Errorf("missing WSL runtime info")
	}
	if runtimeInfo.BridgeDockerHost == "" {
		return nil, fmt.Errorf("WSL bridge did not provide a Docker host")
	}

	p.mu.RLock()
	if p.dockerProvider != nil && p.dockerHost == runtimeInfo.BridgeDockerHost {
		dockerProvider := p.dockerProvider
		p.mu.RUnlock()
		return dockerProvider, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.dockerProvider != nil && p.dockerHost == runtimeInfo.BridgeDockerHost {
		return p.dockerProvider, nil
	}
	if p.dockerProvider != nil {
		_ = p.dockerProvider.Close()
		p.dockerProvider = nil
		p.dockerHost = ""
	}

	cfg := *p.cfg
	cfg.DockerHost = runtimeInfo.BridgeDockerHost

	dockerProvider, err := docker.NewProvider(&cfg, docker.SessionProjectResolver(p.sessionProjectResolver), docker.WithSystemManager(p.systemManager))
	if err != nil {
		return nil, fmt.Errorf("create inner Docker provider for WSL bridge %q: %w", runtimeInfo.BridgeDockerHost, err)
	}

	p.dockerProvider = dockerProvider
	p.dockerHost = runtimeInfo.BridgeDockerHost
	return dockerProvider, nil
}

func bridgeNotReadyError(runtimeInfo *RuntimeInfo) error {
	if runtimeInfo == nil {
		return fmt.Errorf("wsl docker bridge is not ready yet")
	}
	if runtimeInfo.BridgeDockerHost != "" {
		return fmt.Errorf("wsl docker bridge is not ready yet (configured host: %s)", runtimeInfo.BridgeDockerHost)
	}
	if runtimeInfo.BridgeType == BridgeTypeTCP && runtimeInfo.BridgePort == 0 {
		return fmt.Errorf("wsl docker bridge is not ready yet (tcp bridge is configured for a random port and runtime assignment is not implemented yet)")
	}
	return fmt.Errorf("wsl docker bridge is not ready yet")
}
