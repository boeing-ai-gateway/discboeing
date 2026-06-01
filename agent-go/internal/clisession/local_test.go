package clisession

import (
	"context"
	"testing"

	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func TestLocalThreadsIncludePhase(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	workspace := t.TempDir()
	agent := agentimpl.NewDefaultAgent(store, nil, nil, workspace, agentimpl.MCPConfig{})
	local := NewLocal(agent, store, workspace)

	if _, err := store.CreateThreadInfo(workspace, thread.CreateThreadRequest{
		ID:    "thread-1",
		Phase: "review",
	}); err != nil {
		t.Fatalf("CreateThreadInfo() failed: %v", err)
	}

	threads, err := local.ListThreads(context.Background())
	if err != nil {
		t.Fatalf("ListThreads() failed: %v", err)
	}
	if len(threads) != 1 || threads[0].Phase != "review" {
		t.Fatalf("ListThreads() = %#v, want phase review", threads)
	}

	got, err := local.GetThread(context.Background(), "thread-1")
	if err != nil {
		t.Fatalf("GetThread() failed: %v", err)
	}
	if got.Phase != "review" {
		t.Fatalf("GetThread().Phase = %q, want review", got.Phase)
	}

	empty := ""
	got, err = local.UpdateThread(context.Background(), "thread-1", api.UpdateThreadRequest{Phase: &empty})
	if err != nil {
		t.Fatalf("UpdateThread() failed: %v", err)
	}
	if got.Phase != "" {
		t.Fatalf("UpdateThread().Phase = %q, want empty", got.Phase)
	}
}
