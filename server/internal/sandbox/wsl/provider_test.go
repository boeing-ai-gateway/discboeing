//go:build windows

package wsl

import (
	"testing"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
)

func TestProviderSupportsProjectInspection(t *testing.T) {
	var _ sandbox.ProjectInspectionManager = (*Provider)(nil)
}

func TestProviderDefinition(t *testing.T) {
	provider := &Provider{}
	definition := provider.Definition()
	if definition.Name != "WSL2" {
		t.Fatalf("Definition().Name = %q, want %q", definition.Name, "WSL2")
	}
	if definition.Icon != "simple:linux" {
		t.Fatalf("Definition().Icon = %q, want %q", definition.Icon, "simple:linux")
	}
}

func TestProjectVMDialersExist(t *testing.T) {
	projectVM := &projectVM{projectID: "project-1"}
	if projectVM.ProjectID() != "project-1" {
		t.Fatalf("ProjectID() = %q", projectVM.ProjectID())
	}
	if projectVM.DockerDialer() == nil {
		t.Fatal("DockerDialer() returned nil")
	}
	if projectVM.PortDialer(3002) == nil {
		t.Fatal("PortDialer() returned nil")
	}
	if err := projectVM.Shutdown(); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestVMManagerReady(t *testing.T) {
	manager := NewVMManager(testConfig(t), nil)
	select {
	case <-manager.Ready():
	default:
		t.Fatal("Ready() was not closed")
	}
	if err := manager.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	stateDir := t.TempDir()
	return &config.Config{
		WSLDistroName:  "Discobot",
		WSLInstallDir:  t.TempDir(),
		WSLStateDir:    stateDir,
		WSLVarDiskPath: stateDir + "\\var.vhdx",
		WSLImageRef:    "test-image",
	}
}
