package config

import (
	"slices"
	"testing"
	"time"
)

func TestLoadDefaultsToEphemeralHTTPSMode(t *testing.T) {
	t.Setenv("HTTPS_PORT", "3443")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPSPort != 3443 {
		t.Fatalf("expected HTTPSPort 3443, got %d", cfg.HTTPSPort)
	}
	if cfg.HTTPSTLSMode != "ephemeral" {
		t.Fatalf("expected HTTPSTLSMode ephemeral, got %q", cfg.HTTPSTLSMode)
	}
}

func TestLoadSandboxImageRemote(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "discboeing-local/discboeing-agent-api:dev")
	t.Setenv("SANDBOX_IMAGE_REMOTE", "ghcr.io/boeing-ai-gateway/discboeing:main")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SandboxImage != "discboeing-local/discboeing-agent-api:dev" {
		t.Fatalf("expected SandboxImage to use local override, got %q", cfg.SandboxImage)
	}
	if cfg.SandboxImageRemote != "ghcr.io/boeing-ai-gateway/discboeing:main" {
		t.Fatalf("expected SandboxImageRemote to use remote override, got %q", cfg.SandboxImageRemote)
	}
}

func TestLoadSandboxImageRemoteDefaultsBlank(t *testing.T) {
	t.Setenv("SANDBOX_IMAGE", "discboeing-local/discboeing-agent-api:dev")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SandboxImageRemote != "" {
		t.Fatalf("expected SandboxImageRemote to default blank, got %q", cfg.SandboxImageRemote)
	}
}

func TestLoadRejectsIncompleteStaticHTTPSConfig(t *testing.T) {
	t.Setenv("HTTPS_PORT", "3443")
	t.Setenv("HTTPS_TLS_MODE", "static")

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to fail for incomplete static TLS config")
	}
}

func TestLoadRejectsInvalidHTTPSMode(t *testing.T) {
	t.Setenv("HTTPS_PORT", "3443")
	t.Setenv("HTTPS_TLS_MODE", "nope")

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to fail for invalid HTTPS mode")
	}
}

func TestLoadDefaultCORSOriginsIncludeHTTPAndHTTPSPorts(t *testing.T) {
	t.Setenv("PORT", "3007")
	t.Setenv("HTTPS_PORT", "3443")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{
		"http://localhost:3007",
		"http://*.localhost:3007",
		"https://localhost:3443",
		"https://*.localhost:3443",
		"http://localhost:3000",
		"http://*.localhost:3000",
		"http://localhost:3100",
		"http://*.localhost:3100",
	}
	for _, origin := range want {
		if !slices.Contains(cfg.CORSOrigins, origin) {
			t.Fatalf("expected CORS origins to contain %q, got %v", origin, cfg.CORSOrigins)
		}
	}
}

func TestLoadDefaultCORSOriginsUseHTTPSTLSHosts(t *testing.T) {
	t.Setenv("PORT", "3007")
	t.Setenv("HTTPS_PORT", "3443")
	t.Setenv("HTTPS_TLS_HOSTS", "example.com,www.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{
		"http://example.com:3007",
		"http://www.example.com:3007",
		"https://example.com:3443",
		"https://www.example.com:3443",
	}
	for _, origin := range want {
		if !slices.Contains(cfg.CORSOrigins, origin) {
			t.Fatalf("expected CORS origins to contain %q, got %v", origin, cfg.CORSOrigins)
		}
	}
	if slices.Contains(cfg.CORSOrigins, "http://localhost:3007") {
		t.Fatalf("did not expect localhost listener origin when HTTPS_TLS_HOSTS is explicitly set, got %v", cfg.CORSOrigins)
	}
}

func TestLoadCORSOriginsExpandsPortTemplates(t *testing.T) {
	t.Setenv("PORT", "3007")
	t.Setenv("HTTPS_PORT", "3443")
	t.Setenv("CORS_ORIGINS", "http://localhost:{HTTP_PORT},https://localhost:{HTTPS_PORT},https://*.localhost:{HTTPS_PORT}")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{
		"http://localhost:3007",
		"https://localhost:3443",
		"https://*.localhost:3443",
	}
	if !slices.Equal(cfg.CORSOrigins, want) {
		t.Fatalf("expected CORS origins %v, got %v", want, cfg.CORSOrigins)
	}
}

func TestLoadCORSOriginsDropsHTTPSPlaceholdersWhenDisabled(t *testing.T) {
	t.Setenv("PORT", "3007")
	t.Setenv("HTTPS_PORT", "0")
	t.Setenv("CORS_ORIGINS", "http://localhost:{HTTP_PORT},https://localhost:{HTTPS_PORT}")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{"http://localhost:3007"}
	if !slices.Equal(cfg.CORSOrigins, want) {
		t.Fatalf("expected CORS origins %v, got %v", want, cfg.CORSOrigins)
	}
}

func TestLoadDefaultsSessionSandboxCleanupDelay(t *testing.T) {
	t.Setenv("SESSION_SANDBOX_CLEANUP_DELAY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SessionSandboxCleanupDelay != time.Minute {
		t.Fatalf("expected SessionSandboxCleanupDelay 1m, got %s", cfg.SessionSandboxCleanupDelay)
	}
}

func TestLoadSessionSandboxCleanupDelayFromEnv(t *testing.T) {
	t.Setenv("SESSION_SANDBOX_CLEANUP_DELAY", "720h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SessionSandboxCleanupDelay != 720*time.Hour {
		t.Fatalf("expected SessionSandboxCleanupDelay 720h, got %s", cfg.SessionSandboxCleanupDelay)
	}
}

func TestLoadDesktopShellSettingsFromGenericEnv(t *testing.T) {
	t.Setenv("DISCBOEING_DESKTOP_RUNTIME", "electron")
	t.Setenv("DISCBOEING_DESKTOP_SECRET", "desktop-secret")
	t.Setenv("DISCBOEING_DESKTOP_ICON_PATH", `C:\Program Files\Discboeing\icon.ico`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.DesktopMode {
		t.Fatal("expected DesktopMode to be enabled")
	}
	if cfg.DesktopRuntime != "electron" {
		t.Fatalf("expected DesktopRuntime electron, got %q", cfg.DesktopRuntime)
	}
	if cfg.DesktopSecret != "desktop-secret" {
		t.Fatalf("expected DesktopSecret desktop-secret, got %q", cfg.DesktopSecret)
	}
	if cfg.DesktopIconPath != `C:\Program Files\Discboeing\icon.ico` {
		t.Fatalf("expected DesktopIconPath to be loaded, got %q", cfg.DesktopIconPath)
	}
}

func TestLoadWSLDistroNameDefaultsToDisplayName(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.WSLDistroName != "Discboeing" {
		t.Fatalf("expected WSLDistroName Discboeing, got %q", cfg.WSLDistroName)
	}
}

func TestLoadDesktopShellSettingsRequireSecretWhenRuntimeSet(t *testing.T) {
	t.Setenv("DISCBOEING_SECRET", "")
	t.Setenv("DISCBOEING_DESKTOP_SECRET", "")
	t.Setenv("DISCBOEING_DESKTOP_RUNTIME", "electron")

	_, err := Load()
	if err == nil {
		t.Fatal("expected Load() to fail when desktop runtime is set without a secret")
	}
}

func TestLoadDockerWSLDistroFromEnv(t *testing.T) {
	t.Setenv("DISCBOEING_DOCKER_WSL_DISTRO", "Ubuntu-24.04")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DockerWSLDistro != "Ubuntu-24.04" {
		t.Fatalf("expected DockerWSLDistro %q, got %q", "Ubuntu-24.04", cfg.DockerWSLDistro)
	}
}
