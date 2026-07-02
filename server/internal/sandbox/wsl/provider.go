//go:build windows

package wsl

import (
	"context"
	"fmt"

	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/vm"
	"github.com/boeing-ai-gateway/discboeing/server/internal/startup"
)

const (
	startupTaskWSLStartID = "wsl-start"
)

// SessionProjectResolver maps session IDs to project IDs.
type SessionProjectResolver func(ctx context.Context, sessionID string) (projectID string, err error)

// Provider is the Windows WSL sandbox provider. It follows the same VM+Docker
// shape as the VZ provider: WSL manages the project runtime and the generic VM
// provider handles Docker-backed sandboxes inside that runtime.
type Provider struct {
	*vm.Provider

	distroManager    *DistroManager
	resourceResolver vm.ProviderResourceResolver
}

// NewProvider creates a new WSL-backed sandbox provider.
func NewProvider(cfg *config.Config, resolver SessionProjectResolver, resourceResolver vm.ProviderResourceResolver, systemManager *startup.SystemManager) (*Provider, error) {
	if resolver == nil {
		return nil, fmt.Errorf("sessionProjectResolver is required")
	}

	distroManager := NewDistroManager(cfg, systemManager)

	opts := []vm.Option{
		vm.WithProviderResourceResolver(func(ctx context.Context, projectID string) (vm.ProviderResourceConfig, error) {
			resources := vm.ProviderResourceConfig{
				DataDiskGB: distroManager.varDiskSizeGB(),
			}
			if resourceResolver == nil {
				return resources, nil
			}
			resolved, err := resourceResolver(ctx, projectID)
			if err != nil {
				return vm.ProviderResourceConfig{}, err
			}
			if resolved.DataDiskGB > 0 {
				resources.DataDiskGB = resolved.DataDiskGB
			}
			return resources, nil
		}),
		vm.WithProviderName("wsl"),
	}
	if cfg.WSLIdleTimeout > 0 {
		opts = append(opts, vm.WithIdleTimeout(cfg.WSLIdleTimeout))
	}

	provider := vm.NewProvider(
		cfg,
		distroManager,
		vm.SessionProjectResolver(resolver),
		systemManager,
		opts...,
	)
	return &Provider{Provider: provider, distroManager: distroManager, resourceResolver: resourceResolver}, nil
}

// ApplyProviderResourceUpdate records managed /var VHD resize requests for the
// next WSL startup. The startup script applies the host resize while the managed
// runtime is stopped instead of resizing under a running sandbox.
func (p *Provider) ApplyProviderResourceUpdate(ctx context.Context, _ string, req sandbox.UpdateProviderResourcesRequest) error {
	if req.MemoryMB != nil {
		return fmt.Errorf("memory resource updates are not supported for WSL2")
	}
	if req.DataDiskGB == nil {
		return nil
	}
	if p.distroManager == nil {
		return fmt.Errorf("WSL manager is not initialized")
	}
	return p.distroManager.RequestVarDiskResize(ctx, *req.DataDiskGB)
}

func (p *Provider) Definition() sandbox.ProviderDefinition {
	return sandbox.ProviderDefinition{
		Name:        "WSL2",
		Icon:        "simple:linux",
		Description: "WSL2 sandbox driver",
		ConfigFields: []sandbox.ProviderConfigField{
			{Key: "distro", Label: "Distro", Type: "text", Placeholder: "Ubuntu", Description: "WSL distro used for sandbox execution."},
			{Key: "installRoot", Label: "Install root", Type: "text", Placeholder: "C:\\Users\\me\\AppData\\Local\\Discboeing\\wsl", Description: "Optional location for Discboeing-managed WSL data.", Advanced: true},
		},
	}
}
