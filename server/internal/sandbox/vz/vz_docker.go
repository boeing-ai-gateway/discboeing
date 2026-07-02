//go:build darwin

package vz

import (
	"fmt"
	"time"

	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/vm"
)

// NewProvider creates a new VZ+Docker hybrid provider.
// It creates a VZ VMManager (which handles async image download if needed)
// and returns a generic vm.Provider that uses it for VM management.
func NewProvider(cfg *config.Config, vmConfig *vm.Config, resolver vm.SessionProjectResolver, resourceResolver vm.ProviderResourceResolver, systemManager vm.SystemManager) (*vm.Provider, error) {
	vmManager, err := NewVMManager(*vmConfig, systemManager, resourceResolver)
	if err != nil {
		return nil, fmt.Errorf("failed to create VZ VM manager: %w", err)
	}

	opts := []vm.Option{
		vm.WithPostVMSetup(vm.StartProxyContainer(cfg.SandboxImage)),
		vm.WithProviderResourceResolver(resourceResolver),
		vm.WithProviderName("vz"),
	}

	// Parse idle timeout from VM config
	if vmConfig.IdleTimeout != "" {
		idleTimeout, err := time.ParseDuration(vmConfig.IdleTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid idle timeout %q: %w", vmConfig.IdleTimeout, err)
		}
		if idleTimeout > 0 {
			opts = append(opts, vm.WithIdleTimeout(idleTimeout))
		}
	}

	return vm.NewProvider(cfg, vmManager, resolver, systemManager, opts...), nil
}
