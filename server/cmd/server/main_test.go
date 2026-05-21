package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/model"
)

func TestNewExeDevInstanceProviderUsesRemoteSandboxImageFallback(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	remoteImage := "ghcr.io/obot-platform/discobot-agent-api:test-remote"
	instanceConfig, err := json.Marshal(map[string]string{
		"token": "exe1.test-token",
	})
	if err != nil {
		t.Fatalf("failed to marshal instance config: %v", err)
	}

	provider, err := newExeDevInstanceProvider(context.Background(), &config.Config{SandboxImage: localImage, SandboxImageRemote: remoteImage}, nil, &model.SandboxProviderInstance{
		ProjectID: "test-project",
		Type:      "exedev",
		Config:    instanceConfig,
	})
	if err != nil {
		t.Fatalf("newExeDevInstanceProvider failed: %v", err)
	}

	if got := provider.Image(); got != remoteImage {
		t.Fatalf("expected provider image %q, got %q", remoteImage, got)
	}
}

func TestNewExeDevInstanceProviderFallsBackToDefaultRemoteImage(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	instanceConfig, err := json.Marshal(map[string]string{
		"token": "exe1.test-token",
	})
	if err != nil {
		t.Fatalf("failed to marshal instance config: %v", err)
	}

	provider, err := newExeDevInstanceProvider(context.Background(), &config.Config{SandboxImage: localImage}, nil, &model.SandboxProviderInstance{
		ProjectID: "test-project",
		Type:      "exedev",
		Config:    instanceConfig,
	})
	if err != nil {
		t.Fatalf("newExeDevInstanceProvider failed: %v", err)
	}

	if got, want := provider.Image(), config.DefaultSandboxImage(); got != want {
		t.Fatalf("expected provider image %q, got %q", want, got)
	}
}

func TestNewExeDevInstanceProviderUsesInstanceSandboxImageOverride(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	remoteImage := "ghcr.io/obot-platform/discobot-agent-api:test-remote"
	instanceImage := "ghcr.io/obot-platform/discobot-agent-api:test-instance"
	instanceConfig, err := json.Marshal(map[string]string{
		"token":        "exe1.test-token",
		"sandboxImage": instanceImage,
	})
	if err != nil {
		t.Fatalf("failed to marshal instance config: %v", err)
	}

	provider, err := newExeDevInstanceProvider(context.Background(), &config.Config{SandboxImage: localImage, SandboxImageRemote: remoteImage}, nil, &model.SandboxProviderInstance{
		ProjectID: "test-project",
		Type:      "exedev",
		Config:    instanceConfig,
	})
	if err != nil {
		t.Fatalf("newExeDevInstanceProvider failed: %v", err)
	}

	if got := provider.Image(); got != instanceImage {
		t.Fatalf("expected provider image %q, got %q", instanceImage, got)
	}
}

func TestSandboxImageForProviderUsesLocalImageForLocalProviders(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	remoteImage := "ghcr.io/obot-platform/discobot-agent-api:test-remote"
	cfg := &config.Config{
		SandboxImage:       localImage,
		SandboxImageRemote: remoteImage,
	}

	for _, providerName := range []string{"docker", "local", "wsl"} {
		if got := sandboxImageForProvider(cfg, providerName); got != localImage {
			t.Fatalf("provider %q image = %q, want %q", providerName, got, localImage)
		}
	}
}

func TestSandboxImageForProviderUsesRemoteImageForRemoteProviders(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	remoteImage := "ghcr.io/obot-platform/discobot-agent-api:test-remote"
	cfg := &config.Config{
		SandboxImage:       localImage,
		SandboxImageRemote: remoteImage,
	}

	for _, providerName := range []string{"vz", "exedev", "some-remote-provider"} {
		if got := sandboxImageForProvider(cfg, providerName); got != remoteImage {
			t.Fatalf("provider %q image = %q, want %q", providerName, got, remoteImage)
		}
	}
}

func TestSandboxImageForProviderUsesRemoteImageForRemoteDockerHost(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	remoteImage := "ghcr.io/obot-platform/discobot-agent-api:test-remote"
	cfg := &config.Config{
		SandboxImage:       localImage,
		SandboxImageRemote: remoteImage,
		DockerHost:         "ssh://docker.example.com",
	}

	if got := sandboxImageForProvider(cfg, "docker"); got != remoteImage {
		t.Fatalf("remote Docker image = %q, want %q", got, remoteImage)
	}
}

func TestSandboxImageForProviderRemoteFallsBackToDefaultImage(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	cfg := &config.Config{SandboxImage: localImage}

	if got, want := sandboxImageForProvider(cfg, "exedev"), config.DefaultSandboxImage(); got != want {
		t.Fatalf("remote provider image = %q, want %q", got, want)
	}
}

func TestConfigForSandboxProviderCopiesConfigAndSetsProviderImage(t *testing.T) {
	localImage := "discobot-local/discobot-agent-api:test-local"
	remoteImage := "ghcr.io/obot-platform/discobot-agent-api:test-remote"
	cfg := &config.Config{
		SandboxImage:       localImage,
		SandboxImageRemote: remoteImage,
		SandboxProvider:    "vz",
	}

	providerCfg := configForSandboxProvider(cfg, "vz")
	if providerCfg == cfg {
		t.Fatal("expected provider config to be a copy")
	}
	if providerCfg.SandboxImage != remoteImage {
		t.Fatalf("provider config image = %q, want %q", providerCfg.SandboxImage, remoteImage)
	}
	if cfg.SandboxImage != localImage {
		t.Fatalf("original config image = %q, want %q", cfg.SandboxImage, localImage)
	}
	if providerCfg.SandboxProvider != cfg.SandboxProvider {
		t.Fatalf("provider config did not preserve unrelated fields")
	}
}
