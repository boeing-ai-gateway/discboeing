package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVisibleEnvSnapshotWithoutWorkspaceEnvFile(t *testing.T) {
	workspaceRoot := t.TempDir()

	env := visibleEnvSnapshot(workspaceRoot, func() map[string]string {
		return map[string]string{"API_KEY": "secret"}
	})

	if env["API_KEY"] != "secret" {
		t.Fatalf("API_KEY = %q, want secret", env["API_KEY"])
	}
}

func TestVisibleEnvSnapshotMergesWorkspaceAndCredentialEnv(t *testing.T) {
	workspaceRoot := t.TempDir()
	envDir := filepath.Join(workspaceRoot, ".discobot")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "env"), []byte("WORKSPACE=from-file\nSHARED=from-file\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	env := visibleEnvSnapshot(workspaceRoot, func() map[string]string {
		return map[string]string{"CREDENTIAL": "from-cred", "SHARED": "from-cred"}
	})

	if env["WORKSPACE"] != "from-file" {
		t.Fatalf("WORKSPACE = %q, want from-file", env["WORKSPACE"])
	}
	if env["CREDENTIAL"] != "from-cred" {
		t.Fatalf("CREDENTIAL = %q, want from-cred", env["CREDENTIAL"])
	}
	if env["SHARED"] != "from-cred" {
		t.Fatalf("SHARED = %q, want from-cred", env["SHARED"])
	}
}
