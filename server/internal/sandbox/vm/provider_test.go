package vm

import (
	"context"
	"testing"
)

type testVMManager struct {
	resources ProjectResourceConfig
}

func (m *testVMManager) GetOrCreateVM(context.Context, string) (ProjectVM, error) { return nil, nil }
func (m *testVMManager) GetVM(string) (ProjectVM, bool)                           { return nil, false }
func (m *testVMManager) ListProjectIDs() []string                                 { return nil }
func (m *testVMManager) RemoveVM(string) error                                    { return nil }
func (m *testVMManager) Shutdown()                                                {}
func (m *testVMManager) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}
func (m *testVMManager) Err() error { return nil }
func (m *testVMManager) ProjectResources(context.Context, string) (ProjectResourceConfig, error) {
	return m.resources, nil
}

func TestGetProjectResourceInfoUsesVMManagerEffectiveResources(t *testing.T) {
	provider := &Provider{
		providerName: "vz",
		vmManager: &testVMManager{resources: ProjectResourceConfig{
			CPUCount:   8,
			MemoryMB:   16384,
			DataDiskGB: 250,
		}},
		projectResourceResolver: func(context.Context, string) (ProjectResourceConfig, error) {
			return ProjectResourceConfig{}, nil
		},
	}

	info, err := provider.GetProjectResourceInfo(context.Background(), "project-1")
	if err != nil {
		t.Fatalf("GetProjectResourceInfo failed: %v", err)
	}
	if info.Provider != "vz" {
		t.Fatalf("provider = %q, want vz", info.Provider)
	}
	if info.CPUCount != 8 {
		t.Fatalf("cpuCount = %d, want 8", info.CPUCount)
	}
	if info.MemoryMB != 16384 {
		t.Fatalf("memoryMB = %d, want 16384", info.MemoryMB)
	}
	if info.DataDiskGB != 250 {
		t.Fatalf("dataDiskGB = %d, want 250", info.DataDiskGB)
	}
}
