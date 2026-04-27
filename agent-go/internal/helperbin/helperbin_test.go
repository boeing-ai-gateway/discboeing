package helperbin

import (
	"path/filepath"
	"testing"
)

func TestScriptPathForOS_WindowsUsesPs1Extension(t *testing.T) {
	got := ScriptPathForOS("windows", "read-thread")
	if filepath.Ext(got) != ".ps1" {
		t.Fatalf("expected .ps1 extension, got %q", got)
	}
}

func TestScriptPathForOS_WindowsPreservesExistingExtension(t *testing.T) {
	got := ScriptPathForOS("windows", "read-thread.ps1")
	if filepath.Ext(got) != ".ps1" {
		t.Fatalf("expected .ps1 extension, got %q", got)
	}
}

func TestScriptPathForOS_NonWindowsUsesBareName(t *testing.T) {
	got := ScriptPathForOS("linux", "read-thread")
	if filepath.Ext(got) != "" {
		t.Fatalf("expected no extension, got %q", got)
	}
}
