package sessionconfig

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DiscboeingFeatureConfiguration(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	mkdirAll(t, filepath.Join(root, ".discboeing", "services"))

	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.DiscboeingServicesConfigured {
		t.Error("expected services to be configured")
	}
	if cfg.DiscboeingHooksConfigured {
		t.Error("expected hooks to be unconfigured")
	}

	mkdirAll(t, filepath.Join(root, ".discboeing", "hooks"))
	cfg, err = Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DiscboeingHooksConfigured {
		t.Error("expected hooks to be configured")
	}
}

func TestFormatDiscboeingServicesReminder(t *testing.T) {
	got := FormatDiscboeingServicesReminder(false)

	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "`.discboeing/services`") {
		t.Fatalf("expected services path, got %q", got)
	}
	if !strings.Contains(got, "Do not block or derail") {
		t.Fatalf("expected non-blocking guidance, got %q", got)
	}
	if got := FormatDiscboeingServicesReminder(true); got != "" {
		t.Fatalf("expected empty reminder for configured services, got %q", got)
	}
}

func TestFormatDiscboeingHooksReminder(t *testing.T) {
	got := FormatDiscboeingHooksReminder(false)

	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "`.discboeing/hooks`") {
		t.Fatalf("expected hooks path, got %q", got)
	}
	if !strings.Contains(got, "Do not block or derail") {
		t.Fatalf("expected non-blocking guidance, got %q", got)
	}
	if got := FormatDiscboeingHooksReminder(true); got != "" {
		t.Fatalf("expected empty reminder for configured hooks, got %q", got)
	}
}
