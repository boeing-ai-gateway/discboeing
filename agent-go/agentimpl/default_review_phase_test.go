package agentimpl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	agenthooks "github.com/boeing-ai-gateway/discboeing/agent-go/internal/hooks"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func TestHasReviewPhaseHooks(t *testing.T) {
	workspace := t.TempDir()
	hooksDir := filepath.Join(workspace, agenthooks.HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	if hasReviewPhaseHooks(workspace) {
		t.Fatal("hasReviewPhaseHooks() = true before adding a review hook")
	}

	writeHook(t, hooksDir, "regular.md", `---
name: Regular Hook
type: file
engine: ai
pattern: "*.go"
---
Review Go changes.
`)
	if hasReviewPhaseHooks(workspace) {
		t.Fatal("hasReviewPhaseHooks() = true for a hook without phase: review")
	}

	writeHook(t, hooksDir, "review.md", `---
name: Review Hook
type: file
engine: ai
pattern: "*.go"
phase: review
---
Review Go changes.
`)
	if !hasReviewPhaseHooks(workspace) {
		t.Fatal("hasReviewPhaseHooks() = false after adding a review hook")
	}
}

func TestPrompt_ReadyForReviewOnlyExposedForTopLevelThreads(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, agenthooks.HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHook(t, hooksDir, "review.md", `---
name: Review Hook
type: file
engine: ai
pattern: "*.go"
phase: review
---
Review Go changes.
`)

	for _, tc := range []struct {
		name        string
		threadID    string
		setupThread func(*testing.T, *thread.Store, string)
		req         agent.PromptRequest
		want        bool
	}{
		{
			name:     "top level thread gets tool",
			threadID: "thread-top",
			req: agent.PromptRequest{
				UserParts: []message.UIPart{message.UITextPart{Text: "hello"}},
			},
			want: true,
		},
		{
			name:     "parent task request hides tool",
			threadID: "thread-parent-task",
			req: agent.PromptRequest{
				UserParts:    []message.UIPart{message.UITextPart{Text: "hello"}},
				ParentTaskID: "task-1",
			},
			want: false,
		},
		{
			name:     "nested depth request hides tool",
			threadID: "thread-depth",
			req: agent.PromptRequest{
				UserParts:     []message.UIPart{message.UITextPart{Text: "hello"}},
				SubagentDepth: 1,
			},
			want: false,
		},
		{
			name:     "persisted child thread metadata hides tool",
			threadID: "thread-child",
			setupThread: func(t *testing.T, store *thread.Store, threadID string) {
				t.Helper()
				if _, err := store.CreateThreadInfo(root, thread.CreateThreadRequest{
					ID: threadID,
					Metadata: thread.ConfigMetadata{
						ParentThreadID: "thread-parent",
					}.RawMessage(),
				}); err != nil {
					t.Fatal(err)
				}
			},
			req: agent.PromptRequest{
				UserParts: []message.UIPart{message.UITextPart{Text: "hello"}},
			},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := thread.NewStore(t.TempDir())
			if tc.setupThread != nil {
				tc.setupThread(t, store, tc.threadID)
			}
			registry := providers.NewProviderRegistry(nil)
			provider := &toolCaptureProvider{}
			registry.Add(provider)

			agentImpl := NewDefaultAgent(store, registry, nil, root, MCPConfig{})
			for _, err := range agentImpl.Prompt(context.Background(), tc.threadID, tc.req) {
				if err != nil {
					t.Fatal(err)
				}
			}

			got := hasToolNamed(provider.lastRequest.Tools, "ReadyForReview")
			if got != tc.want {
				t.Fatalf("ReadyForReview present = %v, want %v", got, tc.want)
			}
		})
	}
}

func writeHook(t *testing.T, hooksDir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) failed: %v", name, err)
	}
}
