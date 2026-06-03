package server

import (
	"context"
	"encoding/json"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/config"
	"github.com/obot-platform/discobot/agent-go/message"
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

func TestEmitThreadUpdateFallsBackToEphemeralWithoutActiveCompletion(t *testing.T) {
	conversations := agent.NewConversationManager(testAgent{})
	ephemeral, unsubscribe := conversations.SubscribeEphemeral()
	defer unsubscribe()

	want := message.ThreadUpdateChunk{
		Data: message.ThreadUpdateData{
			Thread: message.ThreadUpdateInfo{
				ID: "thread-1",
				PromptQueue: []message.ThreadQueuedPromptInfo{
					{ID: "queue-1"},
				},
			},
		},
	}

	emitThreadUpdate(conversations, "thread-1", want)

	select {
	case got := <-ephemeral:
		update, ok := got.(message.ThreadUpdateChunk)
		if !ok {
			t.Fatalf("got chunk type %T, want message.ThreadUpdateChunk", got)
		}
		if update.Data.Thread.ID != want.Data.Thread.ID {
			t.Fatalf("thread ID = %q, want %q", update.Data.Thread.ID, want.Data.Thread.ID)
		}
		if len(update.Data.Thread.PromptQueue) != 1 || update.Data.Thread.PromptQueue[0].ID != "queue-1" {
			t.Fatalf("prompt queue = %#v, want queue-1", update.Data.Thread.PromptQueue)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ephemeral thread update")
	}
}

func TestWorkspaceFileWatcherEmitsWorkspaceFilesChunks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("workspace file watcher is Linux-only")
	}
	root := t.TempDir()
	chunks := make(chan message.MessageChunk, 8)
	watcher, err := startWorkspaceFileWatcher(root, func(chunk message.MessageChunk) {
		chunks <- chunk
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := watcher.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	event := waitForWorkspaceFileEvent(t, chunks, "hello.txt")
	if len(event.Changes) == 0 {
		t.Fatalf("expected file changes in event: %#v", event)
	}
}

func TestWorkspaceFileWatcherRespectsGitignore(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("workspace file watcher is Linux-only")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}

	chunks := make(chan message.MessageChunk, 8)
	watcher, err := startWorkspaceFileWatcher(root, func(chunk message.MessageChunk) {
		chunks <- chunk
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := watcher.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	if err := os.WriteFile(filepath.Join(root, "ignored", "file.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertNoWorkspaceFileEventForPath(t, chunks, "ignored/file.txt", 150*time.Millisecond)
}

func TestInitHooksDoesNotStartFileWatcherWhenHooksDisabled(t *testing.T) {
	runtime := &agentRuntime{
		cfg: &config.Config{
			HooksEnabled: false,
			AgentCwd:     t.TempDir(),
			SessionID:    "test-session",
		},
	}

	runtime.initHooks()

	if runtime.fileWatcher != nil {
		t.Fatal("file watcher started with hooks disabled")
	}
}

func waitForWorkspaceFileEvent(t *testing.T, chunks <-chan message.MessageChunk, path string) workspaceFileEvent {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case chunk := <-chunks:
			dataChunk, ok := chunk.(message.DataChunk)
			if !ok || dataChunk.DataType != workspaceFilesDataType {
				continue
			}
			var event workspaceFileEvent
			if err := json.Unmarshal(dataChunk.Data, &event); err != nil {
				t.Fatalf("unmarshal workspace file event: %v", err)
			}
			for _, change := range event.Changes {
				if change.Path == path {
					return event
				}
			}
			for _, entry := range event.Snapshot {
				if entry.Path == path {
					return event
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for workspace file event for %s", path)
		}
	}
}

func assertNoWorkspaceFileEventForPath(
	t *testing.T,
	chunks <-chan message.MessageChunk,
	path string,
	duration time.Duration,
) {
	t.Helper()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case chunk := <-chunks:
			dataChunk, ok := chunk.(message.DataChunk)
			if !ok || dataChunk.DataType != workspaceFilesDataType {
				continue
			}
			var event workspaceFileEvent
			if err := json.Unmarshal(dataChunk.Data, &event); err != nil {
				t.Fatalf("unmarshal workspace file event: %v", err)
			}
			for _, change := range event.Changes {
				if change.Path == path {
					t.Fatalf("unexpected workspace file change for %s: %#v", path, change)
				}
			}
			for _, entry := range event.Snapshot {
				if entry.Path == path {
					t.Fatalf("unexpected workspace file snapshot entry for %s: %#v", path, entry)
				}
			}
		case <-timer.C:
			return
		}
	}
}

type testAgent struct{}

func (testAgent) Prompt(context.Context, string, agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
	return func(func(message.MessageChunk, error) bool) {}
}

func (testAgent) Compact(context.Context, string, agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
	return func(func(message.MessageChunk, error) bool) {}
}

func (testAgent) Reset(_ context.Context, threadID string) (agent.ThreadInfo, error) {
	return agent.ThreadInfo{ID: threadID}, nil
}

func (testAgent) Resume(context.Context, string, agent.PromptRequest) (agent.ResumeResult, error) {
	return agent.ResumeResult{}, nil
}

func (testAgent) Cancel(string) bool { return false }

func (testAgent) Messages(string, string) ([]message.UIMessage, error) { return nil, nil }

func (testAgent) ListThreads() ([]string, error) { return nil, nil }

func (testAgent) ListThreadInfos() ([]agent.ThreadInfo, error) { return nil, nil }

func (testAgent) GetThreadInfo(threadID string) (agent.ThreadInfo, error) {
	return agent.ThreadInfo{ID: threadID}, nil
}

func (testAgent) GetThreadTokenUsageDetails(threadID string) (agent.ThreadTokenUsageDetails, error) {
	return agent.ThreadTokenUsageDetails{ThreadID: threadID}, nil
}

func (testAgent) CreateThread(context.Context, agent.CreateThreadRequest) (agent.ThreadInfo, error) {
	return agent.ThreadInfo{}, nil
}

func (testAgent) UpdateThread(context.Context, string, agent.UpdateThreadRequest) (agent.ThreadInfo, error) {
	return agent.ThreadInfo{}, nil
}

func (testAgent) DeleteThread(context.Context, string) error { return nil }

func (testAgent) HasInterruptedTurn(string) (bool, error) { return false, nil }

func (testAgent) PendingQuestion(string) (*agent.PendingQuestion, error) { return nil, nil }

func (testAgent) SubmitAnswer(string, string, api.AnswerQuestionRequest) error { return nil }

func (testAgent) FinalResponse(string) (string, error) { return "", nil }

func (testAgent) ListCommands() ([]agent.Command, error) { return nil, nil }
