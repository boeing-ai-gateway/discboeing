//go:build windows

package wsl

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
)

func TestProviderSupportsProjectInspection(_ *testing.T) {
	var _ sandbox.ProjectInspectionManager = (*Provider)(nil)
}

func TestProviderSupportsProviderResources(_ *testing.T) {
	var _ sandbox.ProviderResourceManager = (*Provider)(nil)
}

func TestProviderRejectsMemoryResourceUpdates(t *testing.T) {
	provider := &Provider{}
	memoryMB := 4096

	err := provider.ApplyProviderResourceUpdate(context.Background(), "project-1", sandbox.UpdateProviderResourcesRequest{
		MemoryMB: &memoryMB,
	})
	if err == nil {
		t.Fatal("ApplyProviderResourceUpdate() error = nil, want error")
	}
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
	manager := NewVMManager(testConfig(t), nil)
	projectVM := &projectVM{manager: manager, projectID: "project-1"}
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

func TestProjectVMWorkspaceMountSourceTranslatesWindowsPath(t *testing.T) {
	projectVM := &projectVM{projectID: "project-1"}

	got, err := projectVM.WorkspaceMountSource(`E:\src\discobot`)
	if err != nil {
		t.Fatalf("WorkspaceMountSource() error = %v", err)
	}
	if got != "/mnt/e/src/discobot" {
		t.Fatalf("WorkspaceMountSource() = %q, want %q", got, "/mnt/e/src/discobot")
	}
}

func TestProjectVMDockerDialerUsesLongLivedBridge(t *testing.T) {
	manager := NewVMManager(testConfig(t), nil)
	projectVM := &projectVM{manager: manager, projectID: "project-1"}
	bridge := &countingDockerBridge{t: t, running: true}

	originalStartWSLDockerBridge := startWSLDockerBridge
	t.Cleanup(func() {
		startWSLDockerBridge = originalStartWSLDockerBridge
	})

	starts := 0
	startWSLDockerBridge = func(_ context.Context, distro string) (dockerBridge, error) {
		starts++
		if distro != "Discobot" {
			return nil, fmt.Errorf("distro = %q, want Discobot", distro)
		}
		return bridge, nil
	}

	for range 2 {
		conn, err := projectVM.DockerDialer()(context.Background(), "", "")
		if err != nil {
			t.Fatalf("DockerDialer() error = %v", err)
		}
		_ = conn.Close()
	}

	if starts != 1 {
		t.Fatalf("bridge starts = %d, want 1", starts)
	}
	if got := bridge.Dials(); got != 2 {
		t.Fatalf("bridge dials = %d, want 2", got)
	}
}

func TestProjectVMPortDialerUsesLocalTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	accepted := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			accepted <- err
			return
		}
		_ = conn.Close()
		accepted <- nil
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	projectVM := &projectVM{projectID: "project-1"}
	conn, err := projectVM.PortDialer(uint32(port))(context.Background(), "", "")
	if err != nil {
		t.Fatalf("PortDialer() error = %v", err)
	}
	_ = conn.Close()

	if err := <-accepted; err != nil {
		t.Fatalf("accept: %v", err)
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

type countingDockerBridge struct {
	t *testing.T

	mu      sync.Mutex
	running bool
	dials   int
}

func (b *countingDockerBridge) Dial(_ context.Context) (net.Conn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.running {
		return nil, fmt.Errorf("bridge is not running")
	}
	b.dials++

	client, server := net.Pipe()
	b.t.Cleanup(func() {
		_ = server.Close()
	})
	return client, nil
}

func (b *countingDockerBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = false
	return nil
}

func (b *countingDockerBridge) Running() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func (b *countingDockerBridge) Dials() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dials
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
