//go:build darwin

package main

import (
	"context"
	"log"

	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/vm"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/vz"
	"github.com/boeing-ai-gateway/discboeing/server/internal/startup"
)

func registerPrimarySandboxProvider(
	cfg *config.Config,
	sandboxProviderManager *sandbox.ProviderManager,
	sessionProjectResolver func(context.Context, string) (string, error),
	providerResourceResolver vm.ProviderResourceResolver,
	systemManager *startup.SystemManager,
) {
	vzCfg := &vm.Config{
		DataDir:       cfg.VZDataDir,
		ConsoleLogDir: cfg.VZConsoleLogDir,
		KernelPath:    cfg.VZKernelPath,
		InitrdPath:    cfg.VZInitrdPath,
		BaseDiskPath:  cfg.VZBaseDiskPath,
		ImageRef:      cfg.VZImageRef,
		HomeDir:       cfg.VZHomeDir,
		CPUCount:      cfg.VZCPUCount,
		MemoryMB:      cfg.VZMemoryMB,
		DataDiskGB:    cfg.VZDataDiskGB,
	}

	providerCfg := configForSandboxProvider(cfg, "vz")
	vmProvider, err := vz.NewProvider(providerCfg, vzCfg, sessionProjectResolver, providerResourceResolver, systemManager)
	if err != nil {
		log.Printf("Warning: Failed to initialize VZ sandbox provider: %v", err)
		return
	}

	sandboxProviderManager.RegisterProvider("vz", vmProvider)
	if vmProvider.IsReady() {
		log.Printf("VZ sandbox provider initialized and ready")
		return
	}

	log.Printf("VZ sandbox provider registered (images downloading in background)")
}
