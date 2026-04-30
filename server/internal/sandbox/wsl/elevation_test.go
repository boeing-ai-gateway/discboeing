//go:build windows

package wsl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultWSLElevationHelperPathFindsNearbySrcTauriBinariesInDev(t *testing.T) {
	root := t.TempDir()
	helperPath := filepath.Join(root, "src-tauri", "binaries", wslElevationHelperBinaryName())
	if err := os.MkdirAll(filepath.Dir(helperPath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(helperPath, []byte("helper"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalExecutablePath := osExecutablePath
	originalGetwdPath := osGetwdPath
	t.Cleanup(func() {
		osExecutablePath = originalExecutablePath
		osGetwdPath = originalGetwdPath
	})

	osExecutablePath = func() (string, error) {
		return filepath.Join(root, "server", "tmp", "discobot-server.exe"), nil
	}
	osGetwdPath = func() (string, error) {
		return filepath.Join(root, "server"), nil
	}

	got, err := defaultWSLElevationHelperPath()
	if err != nil {
		t.Fatalf("defaultWSLElevationHelperPath() error = %v", err)
	}
	if got != helperPath {
		t.Fatalf("defaultWSLElevationHelperPath() = %q, want %q", got, helperPath)
	}
}
