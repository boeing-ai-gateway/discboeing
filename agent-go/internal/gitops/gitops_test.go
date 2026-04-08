package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetCommitPatchesPreservesFileModes(t *testing.T) {
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")

	scriptPath := filepath.Join(repoDir, "script.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	runGitCommand(t, repoDir, "add", "script.sh")
	runGitCommand(t, repoDir, "commit", "-m", "Add script")
	base := strings.TrimSpace(runGitCommand(t, repoDir, "rev-parse", "HEAD"))

	if err := os.Chmod(scriptPath, 0755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}
	runGitCommand(t, repoDir, "add", "script.sh")
	if strings.TrimSpace(runGitCommand(t, repoDir, "status", "--short", "--", "script.sh")) == "" {
		t.Skip("git does not track executable-bit-only changes on this platform")
	}
	runGitCommand(t, repoDir, "commit", "-m", "Make script executable")

	result, commitsErr := GetCommitPatches(repoDir, base)
	if commitsErr != nil {
		t.Fatalf("GetCommitPatches: %v", commitsErr)
	}
	if result.CommitCount != 1 {
		t.Fatalf("expected 1 commit, got %d", result.CommitCount)
	}
	if !strings.Contains(result.Patches, "old mode 100644") {
		t.Fatalf("expected old mode in patch, got %q", result.Patches)
	}
	if !strings.Contains(result.Patches, "new mode 100755") {
		t.Fatalf("expected new mode in patch, got %q", result.Patches)
	}
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, output)
	}
	return string(output)
}
