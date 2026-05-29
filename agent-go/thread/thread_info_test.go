package thread

import (
	"strings"
	"testing"
)

func TestThreadInfoPhaseCreateAndUpdate(t *testing.T) {
	store := NewStore(t.TempDir())

	info, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID:    "thread-1",
		Phase: " Review ",
	})
	if err != nil {
		t.Fatalf("CreateThreadInfo() failed: %v", err)
	}
	if info.Phase != "review" {
		t.Fatalf("created phase = %q, want review", info.Phase)
	}

	emptyPhase := ""
	info, err = store.UpdateThreadInfo("thread-1", UpdateThreadRequest{
		Phase: &emptyPhase,
	})
	if err != nil {
		t.Fatalf("UpdateThreadInfo() failed: %v", err)
	}
	if info.Phase != "" {
		t.Fatalf("updated phase = %q, want empty", info.Phase)
	}
}

func TestThreadInfoRejectsInvalidPhase(t *testing.T) {
	store := NewStore(t.TempDir())

	if _, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID:    "thread-1",
		Phase: "ship",
	}); err == nil || !strings.Contains(err.Error(), "invalid thread phase") {
		t.Fatalf("CreateThreadInfo() error = %v, want invalid thread phase", err)
	}

	if _, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID: "thread-2",
	}); err != nil {
		t.Fatalf("CreateThreadInfo(thread-2) failed: %v", err)
	}
	invalid := "ship"
	if _, err := store.UpdateThreadInfo("thread-2", UpdateThreadRequest{
		Phase: &invalid,
	}); err == nil || !strings.Contains(err.Error(), "invalid thread phase") {
		t.Fatalf("UpdateThreadInfo() error = %v, want invalid thread phase", err)
	}
}
