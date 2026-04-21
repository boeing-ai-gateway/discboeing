package handler

import (
	"context"
	"encoding/json"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/internal/hooks"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func TestStartHookFailureReprompt_SendsPromptRequest(t *testing.T) {
	reqCh := make(chan agent.PromptRequest, 1)
	ma := &streamTestAgent{
		promptFn: func(_ context.Context, threadID string, req agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
			if threadID != "thread-1" {
				t.Fatalf("threadID = %q, want %q", threadID, "thread-1")
			}
			reqCh <- req
			return func(_ func(message.MessageChunk, error) bool) {}
		},
	}
	cm := agent.NewCompletionManager(ma)
	h := New("", cm, nil, nil, nil)

	err := h.startHookFailureReprompt("thread-1", hooks.FileHookEvalResult{
		ShouldReprompt: true,
		LLMMessage:     "### Hook failed: Go Check",
		HookFailure: &hooks.HookFailureMessageMetadata{
			Kind:     "hook-failure",
			HookName: "Go Check",
			ExitCode: 1,
		},
	})
	if err != nil {
		t.Fatalf("startHookFailureReprompt() failed: %v", err)
	}

	select {
	case req := <-reqCh:
		if len(req.UserParts) != 1 {
			t.Fatalf("expected 1 user part, got %d", len(req.UserParts))
		}
		part, ok := req.UserParts[0].(message.UITextPart)
		if !ok {
			t.Fatalf("expected UITextPart, got %T", req.UserParts[0])
		}
		if part.Text != "### Hook failed: Go Check" {
			t.Fatalf("part text = %q, want %q", part.Text, "### Hook failed: Go Check")
		}

		var meta struct {
			Discobot hooks.HookFailureMessageMetadata `json:"discobot"`
		}
		if err := json.Unmarshal(req.Metadata, &meta); err != nil {
			t.Fatalf("unmarshal metadata: %v", err)
		}
		if meta.Discobot.Kind != "hook-failure" {
			t.Fatalf("kind = %q, want %q", meta.Discobot.Kind, "hook-failure")
		}
		if meta.Discobot.HookName != "Go Check" {
			t.Fatalf("hook name = %q, want %q", meta.Discobot.HookName, "Go Check")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected prompt request")
	}
}

func TestStartHookFailureReprompt_UsesResumeForInterruptedTurn(t *testing.T) {
	type resumeCall struct {
		threadID string
		req      agent.PromptRequest
	}

	store := thread.NewStore(t.TempDir())
	if err := store.CreateThread("thread-1"); err != nil {
		t.Fatal(err)
	}
	defaultAgent := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})

	resumeCh := make(chan resumeCall, 1)
	ma := &streamTestAgent{
		hasInterruptedTurnFn: func(threadID string) (bool, error) {
			if threadID != "thread-1" {
				t.Fatalf("threadID = %q, want %q", threadID, "thread-1")
			}
			return true, nil
		},
		promptFn: func(_ context.Context, _ string, _ agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
			t.Fatal("startHookFailureReprompt should resume interrupted turns")
			return func(_ func(message.MessageChunk, error) bool) {}
		},
		resumeFn: func(_ context.Context, threadID string, req agent.PromptRequest) (agent.ResumeResult, error) {
			resumeCh <- resumeCall{threadID: threadID, req: req}
			return agent.ResumeResult{Stream: func(_ func(message.MessageChunk, error) bool) {}}, nil
		},
	}
	cm := agent.NewCompletionManager(ma)
	h := New("", cm, nil, nil, defaultAgent)

	err := h.startHookFailureReprompt("thread-1", hooks.FileHookEvalResult{
		ShouldReprompt: true,
		LLMMessage:     "### Hook failed: Go Check",
		HookFailure: &hooks.HookFailureMessageMetadata{
			Kind:     "hook-failure",
			HookName: "Go Check",
			ExitCode: 1,
		},
	})
	if err != nil {
		t.Fatalf("startHookFailureReprompt() failed: %v", err)
	}

	select {
	case call := <-resumeCh:
		if call.threadID != "thread-1" {
			t.Fatalf("threadID = %q, want %q", call.threadID, "thread-1")
		}
		if len(call.req.UserParts) != 1 {
			t.Fatalf("expected 1 user part, got %d", len(call.req.UserParts))
		}
		part, ok := call.req.UserParts[0].(message.UITextPart)
		if !ok {
			t.Fatalf("expected UITextPart, got %T", call.req.UserParts[0])
		}
		if part.Text != "### Hook failed: Go Check" {
			t.Fatalf("part text = %q, want %q", part.Text, "### Hook failed: Go Check")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected hook failure resume request")
	}

	waitForCompletionDone(t, cm, "thread-1")
}

func TestEnqueueHookFailureReprompt_PrependsQueuedPromptWithMetadata(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	if err := store.CreateThread("thread-1"); err != nil {
		t.Fatal(err)
	}
	defaultAgent := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	h := New("", agent.NewCompletionManager(&streamTestAgent{}), nil, nil, defaultAgent)

	if _, _, err := store.AppendQueuedPrompt("thread-1", thread.QueuedPrompt{
		Message: message.UIMessage{
			ID:    "user-queued",
			Role:  "user",
			Parts: []message.UIPart{message.UITextPart{Text: "normal follow-up"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result := hooks.FileHookEvalResult{
		ShouldReprompt: true,
		LLMMessage:     "### Hook failed: Go Check",
		HookFailure: &hooks.HookFailureMessageMetadata{
			Kind:     "hook-failure",
			HookName: "Go Check",
			ExitCode: 1,
		},
	}
	if err := h.enqueueHookFailureReprompt("thread-1", result); err != nil {
		t.Fatalf("enqueueHookFailureReprompt() failed: %v", err)
	}

	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PromptQueue) != 2 {
		t.Fatalf("prompt queue length = %d, want 2", len(cfg.PromptQueue))
	}

	first := cfg.PromptQueue[0]
	part, ok := first.Message.Parts[0].(message.UITextPart)
	if !ok {
		t.Fatalf("expected UITextPart, got %T", first.Message.Parts[0])
	}
	if part.Text != "### Hook failed: Go Check" {
		t.Fatalf("part text = %q, want %q", part.Text, "### Hook failed: Go Check")
	}
	if len(first.Message.Metadata) == 0 {
		t.Fatal("expected metadata on queued hook re-prompt")
	}

	var meta struct {
		Discobot hooks.HookFailureMessageMetadata `json:"discobot"`
	}
	if err := json.Unmarshal(first.Message.Metadata, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.Discobot.HookName != "Go Check" {
		t.Fatalf("hook name = %q, want %q", meta.Discobot.HookName, "Go Check")
	}

	second := cfg.PromptQueue[1]
	secondPart, ok := second.Message.Parts[0].(message.UITextPart)
	if !ok {
		t.Fatalf("expected second UITextPart, got %T", second.Message.Parts[0])
	}
	if secondPart.Text != "normal follow-up" {
		t.Fatalf("second part text = %q, want %q", secondPart.Text, "normal follow-up")
	}
}

func TestScheduleHookEvaluation_SticksNotificationToFirstThread(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, hooks.HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDir, "go-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Go Check
# type: file
# pattern: "*.go"
#---
echo "lint failed"
exit 1
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(workspaceRoot, "main.go")
	if err := os.WriteFile(mainPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hookManager := hooks.NewManager(workspaceRoot, "session-123")
	if err := hookManager.Init(); err != nil {
		t.Fatal(err)
	}

	reqThreadCh := make(chan string, 4)
	ma := &streamTestAgent{
		promptFn: func(_ context.Context, threadID string, _ agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
			reqThreadCh <- threadID
			return func(_ func(message.MessageChunk, error) bool) {}
		},
	}
	h := New("", agent.NewCompletionManager(ma), hookManager, nil, nil)

	h.scheduleHookEvaluation("thread-1")
	select {
	case got := <-reqThreadCh:
		if got != "thread-1" {
			t.Fatalf("first hook notification thread = %q, want %q", got, "thread-1")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first hook notification")
	}

	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(mainPath, []byte("package main\n\nvar _ = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h.scheduleHookEvaluation("thread-2")
	select {
	case got := <-reqThreadCh:
		t.Fatalf("unexpected hook notification for %q; wanted original thread only", got)
	case <-time.After(400 * time.Millisecond):
	}
}

func TestScheduleHookEvaluation_QueuesHookRepromptWhenThreadBusy(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, hooks.HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDir, "go-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Go Check
# type: file
# pattern: "*.go"
#---
echo "lint failed"
exit 1
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hookManager := hooks.NewManager(workspaceRoot, "session-123")
	if err := hookManager.Init(); err != nil {
		t.Fatal(err)
	}

	store := thread.NewStore(t.TempDir())
	if err := store.CreateThread("thread-1"); err != nil {
		t.Fatal(err)
	}
	defaultAgent := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})

	release := make(chan struct{})
	reqCh := make(chan agent.PromptRequest, 2)
	ma := &streamTestAgent{
		promptFn: func(ctx context.Context, _ string, req agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
			reqCh <- req
			return func(yield func(message.MessageChunk, error) bool) {
				if !yield(message.StartChunk{MessageID: "assistant-1"}, nil) {
					return
				}
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
		},
	}
	cm := agent.NewCompletionManager(ma)
	h := New("", cm, hookManager, nil, defaultAgent)

	if _, err := cm.Chat("thread-1", agent.PromptRequest{
		UserParts: []message.UIPart{message.UITextPart{Text: "active turn"}},
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reqCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for active turn")
	}

	h.scheduleHookEvaluation("thread-1")

	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PromptQueue) != 1 {
		t.Fatalf("prompt queue length = %d, want 1", len(cfg.PromptQueue))
	}
	part, ok := cfg.PromptQueue[0].Message.Parts[0].(message.UITextPart)
	if !ok {
		t.Fatalf("expected queued UITextPart, got %T", cfg.PromptQueue[0].Message.Parts[0])
	}
	if !strings.Contains(part.Text, "### Hook failed: Go Check") {
		t.Fatalf("queued prompt text = %q, want hook failure prompt", part.Text)
	}

	close(release)
	select {
	case queuedReq := <-reqCh:
		part, ok := queuedReq.UserParts[0].(message.UITextPart)
		if !ok {
			t.Fatalf("expected queued UITextPart, got %T", queuedReq.UserParts[0])
		}
		if !strings.Contains(part.Text, "### Hook failed: Go Check") {
			t.Fatalf("queued prompt text = %q, want hook failure prompt", part.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued hook re-prompt to start")
	}
	waitForCompletionDone(t, cm, "thread-1")
}
