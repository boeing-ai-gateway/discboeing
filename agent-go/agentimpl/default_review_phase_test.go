package agentimpl

import (
	"os"
	"path/filepath"
	"testing"

	agenthooks "github.com/obot-platform/discobot/agent-go/internal/hooks"
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

func writeHook(t *testing.T, hooksDir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) failed: %v", name, err)
	}
}
