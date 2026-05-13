package tools

import (
	"slices"
	"testing"

	"github.com/obot-platform/discobot/agent-go/thread"
)

func TestBashEnvForToolUsesBrowserEnvCallback(t *testing.T) {
	t.Parallel()

	exec := New(t.TempDir(), t.TempDir(), "")
	exec.SetEnvForThread(func(threadID string) map[string]string {
		if threadID != "thread-1" {
			t.Fatalf("expected thread-1, got %q", threadID)
		}
		return map[string]string{
			"DISCOBOT_BROWSER_CDP_URL":   "ws://127.0.0.1/browser?threadId=thread-1",
			"BU_CDP_WS":                  "ws://127.0.0.1/browser?threadId=thread-1",
			"DISCOBOT_BROWSER_THREAD_ID": "thread-1",
		}
	})

	env := exec.bashEnvForTool(&thread.ToolContext{ThreadID: "thread-1"})
	joined := ""
	for _, item := range env {
		joined += item + "\n"
	}
	if !containsLine(env, "DISCOBOT_BROWSER_CDP_URL=ws://127.0.0.1/browser?threadId=thread-1") {
		t.Fatalf("expected browser cdp url in env, got %s", joined)
	}
	if !containsLine(env, "DISCOBOT_BROWSER_THREAD_ID=thread-1") {
		t.Fatalf("expected browser thread id in env, got %s", joined)
	}
	if !containsLine(env, "BU_CDP_WS=ws://127.0.0.1/browser?threadId=thread-1") {
		t.Fatalf("expected browser-harness cdp url in env, got %s", joined)
	}
}

func containsLine(lines []string, want string) bool {
	return slices.Contains(lines, want)
}
