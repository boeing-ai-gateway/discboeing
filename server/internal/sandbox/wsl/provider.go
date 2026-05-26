//go:build windows

package wsl

import (
	"context"
	"fmt"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
	"github.com/obot-platform/discobot/server/internal/startup"
)

const (
	startupTaskWSLInstallID = "wsl-install"
	startupTaskWSLStartID   = "wsl-start"
)

// SessionProjectResolver maps session IDs to project IDs.
type SessionProjectResolver func(ctx context.Context, sessionID string) (projectID string, err error)

// Provider is the Windows WSL sandbox provider. It follows the same VM+Docker
// shape as the VZ provider: WSL manages the project runtime and the generic VM
// provider handles Docker-backed sandboxes inside that runtime.
type Provider struct {
	*vm.Provider

	resourceResolver vm.ProviderResourceResolver
}

// NewProvider creates a new WSL-backed sandbox provider.
func NewProvider(cfg *config.Config, resolver SessionProjectResolver, resourceResolver vm.ProviderResourceResolver, systemManager *startup.SystemManager) (*Provider, error) {
	if resolver == nil {
		return nil, fmt.Errorf("sessionProjectResolver is required")
	}

	vmManager := NewVMManager(cfg, systemManager)

	opts := []vm.Option{
		vm.WithPostVMSetup(vm.StartProxyContainer(cfg.SandboxImage)),
		vm.WithProviderResourceResolver(func(ctx context.Context, projectID string) (vm.ProviderResourceConfig, error) {
			resources := vm.ProviderResourceConfig{
				DataDiskGB: vmManager.manager.varDiskSizeGB(),
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
		vmManager,
		vm.SessionProjectResolver(resolver),
		systemManager,
		opts...,
	)
	return &Provider{Provider: provider, resourceResolver: resourceResolver}, nil
}

// ApplyProviderResourceUpdate only allows resizing the managed /var VHD.
func (p *Provider) ApplyProviderResourceUpdate(ctx context.Context, projectID string, req sandbox.UpdateProviderResourcesRequest) error {
	if req.MemoryMB != nil {
		return fmt.Errorf("memory resource updates are not supported for WSL2")
	}
	return p.Provider.ApplyProviderResourceUpdate(ctx, projectID, req)
}

func (p *Provider) Definition() sandbox.ProviderDefinition {
	return sandbox.ProviderDefinition{
		Name:        "WSL2",
		Icon:        "simple:linux",
		Description: "WSL2 sandbox driver",
		ConfigFields: []sandbox.ProviderConfigField{
			{Key: "distro", Label: "Distro", Type: "text", Placeholder: "Ubuntu", Description: "WSL distro used for sandbox execution."},
			{Key: "installRoot", Label: "Install root", Type: "text", Placeholder: "C:\\Users\\me\\AppData\\Local\\Discobot\\wsl", Description: "Optional location for Discobot-managed WSL data.", Advanced: true},
		},
	}
}
