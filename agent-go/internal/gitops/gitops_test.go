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

func TestGetCommitPatches_DefaultTargetUsesLocalMergeBase(t *testing.T) {
	originDir := filepath.Join(t.TempDir(), "origin.git")
	runGitCommand(t, "", "init", "--bare", originDir)

	seedRepo := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "init", "-b", "main", seedRepo)
	runGitCommand(t, seedRepo, "config", "user.email", "test@example.com")
	runGitCommand(t, seedRepo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedRepo, "hello.txt"), []byte("base\n"), 0600); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitCommand(t, seedRepo, "add", "hello.txt")
	runGitCommand(t, seedRepo, "commit", "-m", "Initial")
	runGitCommand(t, seedRepo, "remote", "add", "origin", originDir)
	runGitCommand(t, seedRepo, "push", "-u", "origin", "main")

	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGitCommand(t, "", "clone", "--branch", "main", originDir, cloneDir)
	runGitCommand(t, cloneDir, "config", "user.email", "test@example.com")
	runGitCommand(t, cloneDir, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(cloneDir, "hello.txt"), []byte("base\nlocal-1\n"), 0600); err != nil {
		t.Fatalf("write local-1 file: %v", err)
	}
	runGitCommand(t, cloneDir, "add", "hello.txt")
	runGitCommand(t, cloneDir, "commit", "-m", "Local one")
	if err := os.WriteFile(filepath.Join(cloneDir, "hello.txt"), []byte("base\nlocal-1\nlocal-2\n"), 0600); err != nil {
		t.Fatalf("write local-2 file: %v", err)
	}
	runGitCommand(t, cloneDir, "add", "hello.txt")
	runGitCommand(t, cloneDir, "commit", "-m", "Local two")

	if err := os.WriteFile(filepath.Join(seedRepo, "hello.txt"), []byte("base\nremote\n"), 0600); err != nil {
		t.Fatalf("write remote file: %v", err)
	}
	runGitCommand(t, seedRepo, "add", "hello.txt")
	runGitCommand(t, seedRepo, "commit", "-m", "Remote change")
	runGitCommand(t, seedRepo, "push", "origin", "main")
	runGitCommand(t, cloneDir, "fetch", "origin")

	result, commitsErr := GetCommitPatches(cloneDir, "")
	if commitsErr != nil {
		t.Fatalf("GetCommitPatches: %v", commitsErr)
	}
	if result.CommitCount != 2 {
		t.Fatalf("expected 2 commits, got %d", result.CommitCount)
	}
	if !strings.Contains(result.Patches, "Subject: [PATCH 1/2] Local one") {
		t.Fatalf("expected first local commit in patch bundle, got %q", result.Patches)
	}
	if !strings.Contains(result.Patches, "Subject: [PATCH 2/2] Local two") {
		t.Fatalf("expected second local commit in patch bundle, got %q", result.Patches)
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
