package wsl

import "testing"

func TestStateStoreRequestVarDiskSize(t *testing.T) {
	store := NewStateStore(t.TempDir())

	if err := store.RequestVarDiskSize(250, 100, "runtime-1"); err != nil {
		t.Fatalf("RequestVarDiskSize() error = %v", err)
	}

	state, found, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if !found {
		t.Fatal("Read() found = false, want true")
	}
	if state.VarDiskSizeGB != 100 {
		t.Fatalf("VarDiskSizeGB = %d, want 100", state.VarDiskSizeGB)
	}
	if state.DesiredVarDiskSizeGB != 250 {
		t.Fatalf("DesiredVarDiskSizeGB = %d, want 250", state.DesiredVarDiskSizeGB)
	}
	if state.VarDiskResizeRequestedBy != "runtime-1" {
		t.Fatalf("VarDiskResizeRequestedBy = %q, want runtime-1", state.VarDiskResizeRequestedBy)
	}
	if state.UpdatedAt == "" {
		t.Fatal("UpdatedAt is empty")
	}
}
