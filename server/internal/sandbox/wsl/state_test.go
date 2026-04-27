package wsl

import (
	"path/filepath"
	"testing"
)

func TestStateStoreSaveLoadClear(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "wsl-state"))

	original := RuntimeState{
		DistroName: "discobot",
		BridgeType: BridgeTypeTCP,
		BridgePort: 23755,
		ImageRef:   "ghcr.io/obot-platform/discobot-vz:test",
	}
	if err := store.Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Version != runtimeStateVersion {
		t.Fatalf("Load() version = %d, want %d", loaded.Version, runtimeStateVersion)
	}
	if loaded.DistroName != original.DistroName {
		t.Fatalf("Load() distro = %q, want %q", loaded.DistroName, original.DistroName)
	}
	if loaded.BridgeType != original.BridgeType {
		t.Fatalf("Load() bridge type = %q, want %q", loaded.BridgeType, original.BridgeType)
	}
	if loaded.BridgePort != original.BridgePort {
		t.Fatalf("Load() bridge port = %d, want %d", loaded.BridgePort, original.BridgePort)
	}
	if loaded.ImageRef != original.ImageRef {
		t.Fatalf("Load() image ref = %q, want %q", loaded.ImageRef, original.ImageRef)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("Load() UpdatedAt is zero")
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	cleared, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after Clear error = %v", err)
	}
	if cleared != (RuntimeState{}) {
		t.Fatalf("Load() after Clear = %#v, want zero value", cleared)
	}
}

func TestStateStoreLoadMissingFile(t *testing.T) {
	store := NewStateStore(filepath.Join(t.TempDir(), "missing-state"))
	state, err := store.Load()
	if err != nil {
		t.Fatalf("Load() missing file error = %v", err)
	}
	if state != (RuntimeState{}) {
		t.Fatalf("Load() missing file = %#v, want zero value", state)
	}
}
