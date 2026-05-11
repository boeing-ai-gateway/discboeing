//go:build windows

package main

import (
	"context"
	"log"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
	"github.com/obot-platform/discobot/server/internal/sandbox/wsl"
	"github.com/obot-platform/discobot/server/internal/startup"
)

func registerPrimarySandboxProvider(
	cfg *config.Config,
	sandboxManager *sandbox.Manager,
	sessionProjectResolver func(context.Context, string) (string, error),
	_ vm.ProjectResourceResolver,
	systemManager *startup.SystemManager,
) {
	if cfg.WSLRootfsPath != "" {
		log.Printf("WSL runtime rootfs source: local archive %s", cfg.WSLRootfsPath)
	} else {
		log.Printf("WSL runtime rootfs source: image %s", cfg.WSLImageRef)
	}

	wslProvider, err := wsl.NewProvider(configForSandboxProvider(cfg, "wsl"), sessionProjectResolver, systemManager)
	if err != nil {
		log.Printf("Warning: Failed to initialize WSL sandbox provider: %v", err)
		return
	}

	sandboxManager.RegisterProvider("wsl", wslProvider)
	log.Printf("WSL sandbox provider initialized (state: %s)", wslProvider.Status().State)
}
