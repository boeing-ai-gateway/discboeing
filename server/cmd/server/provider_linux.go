//go:build linux

package main

import (
	"context"
	"log"

	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/docker"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/vm"
	"github.com/boeing-ai-gateway/discboeing/server/internal/startup"
)

func registerPrimarySandboxProvider(
	cfg *config.Config,
	sandboxProviderManager *sandbox.ProviderManager,
	sessionProjectResolver func(context.Context, string) (string, error),
	_ vm.ProviderResourceResolver,
	systemManager *startup.SystemManager,
) {
	providerCfg := configForSandboxProvider(cfg, "docker")
	dockerProvider, err := docker.NewProvider(providerCfg, sessionProjectResolver, docker.WithSystemManager(systemManager))
	if err != nil {
		log.Printf("Warning: Failed to initialize Docker sandbox provider: %v", err)
		return
	}

	sandboxProviderManager.RegisterProvider("docker", dockerProvider)
	log.Printf("Docker sandbox provider initialized (image: %s)", providerCfg.SandboxImage)
}
