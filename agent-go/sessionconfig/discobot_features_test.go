package sessionconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DiscobotFeatureConfiguration(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	mkdirAll(t, filepath.Join(root, ".discobot", "services"))

	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.DiscobotServicesConfigured {
		t.Error("expected services to be configured")
	}
	if cfg.DiscobotHooksConfigured {
		t.Error("expected hooks to be unconfigured")
	}

	mkdirAll(t, filepath.Join(root, ".discobot", "hooks"))
	cfg, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DiscobotHooksConfigured {
		t.Error("expected hooks to be configured")
	}
}

func TestFormatDiscobotServicesReminder(t *testing.T) {
	got := FormatDiscobotServicesReminder(false)

	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "`.discobot/services`") {
		t.Fatalf("expected services path, got %q", got)
	}
	if !strings.Contains(got, "Do not block or derail") {
		t.Fatalf("expected non-blocking guidance, got %q", got)
	}
	if got := FormatDiscobotServicesReminder(true); got != "" {
		t.Fatalf("expected empty reminder for configured services, got %q", got)
	}
}

func TestFormatDiscobotHooksReminder(t *testing.T) {
	got := FormatDiscobotHooksReminder(false)

	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "`.discobot/hooks`") {
		t.Fatalf("expected hooks path, got %q", got)
	}
	if !strings.Contains(got, "Do not block or derail") {
		t.Fatalf("expected non-blocking guidance, got %q", got)
	}
	if got := FormatDiscobotHooksReminder(true); got != "" {
		t.Fatalf("expected empty reminder for configured hooks, got %q", got)
	}
}
