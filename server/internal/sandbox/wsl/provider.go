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
}

// NewProvider creates a new WSL-backed sandbox provider.
func NewProvider(cfg *config.Config, resolver SessionProjectResolver, systemManager *startup.SystemManager) (*Provider, error) {
	if resolver == nil {
		return nil, fmt.Errorf("sessionProjectResolver is required")
	}

	vmManager := NewVMManager(cfg, systemManager)

	opts := []vm.Option{
		vm.WithPostVMSetup(vm.StartProxyContainer(cfg.SandboxImage)),
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
	return &Provider{Provider: provider}, nil
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
