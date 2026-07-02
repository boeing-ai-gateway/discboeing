package tools

import (
	"slices"
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func TestBashEnvForToolUsesBrowserEnvCallback(t *testing.T) {
	t.Parallel()

	exec := New(t.TempDir(), t.TempDir(), "")
	exec.SetEnvForThread(func(threadID string) map[string]string {
		if threadID != "thread-1" {
			t.Fatalf("expected thread-1, got %q", threadID)
		}
		return map[string]string{
			"DISCBOEING_BROWSER_CDP_URL":   "ws://127.0.0.1/browser?threadId=thread-1",
			"BU_CDP_WS":                  "ws://127.0.0.1/browser?threadId=thread-1",
			"DISCBOEING_BROWSER_THREAD_ID": "thread-1",
		}
	})

	env := exec.bashEnvForTool(&thread.ToolContext{ThreadID: "thread-1"})
	var joined strings.Builder
	for _, item := range env {
		joined.WriteString(item + "\n")
	}
	if !containsLine(env, "DISCBOEING_BROWSER_CDP_URL=ws://127.0.0.1/browser?threadId=thread-1") {
		t.Fatalf("expected browser cdp url in env, got %s", joined.String())
	}
	if !containsLine(env, "DISCBOEING_BROWSER_THREAD_ID=thread-1") {
		t.Fatalf("expected browser thread id in env, got %s", joined.String())
	}
	if !containsLine(env, "BU_CDP_WS=ws://127.0.0.1/browser?threadId=thread-1") {
		t.Fatalf("expected browser-harness cdp url in env, got %s", joined.String())
	}
}

func containsLine(lines []string, want string) bool {
	return slices.Contains(lines, want)
}
