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

func TestGetCommitPatchesAtHead_UsesExplicitBaseAndHead(t *testing.T) {
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init", "-b", "main")
	runGitCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	runGitCommand(t, repoDir, "commit", "-m", "Base")
	baseCommit := strings.TrimSpace(runGitCommand(t, repoDir, "rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("base\none\n"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	runGitCommand(t, repoDir, "commit", "-m", "Commit one")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("base\none\ntwo\n"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	runGitCommand(t, repoDir, "commit", "-m", "Commit two")
	headCommit := strings.TrimSpace(runGitCommand(t, repoDir, "rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(repoDir, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	result, commitsErr := GetCommitPatchesAtHead(repoDir, baseCommit, headCommit)
	if commitsErr != nil {
		t.Fatalf("GetCommitPatchesAtHead: %v", commitsErr)
	}
	if result.HeadCommit != headCommit {
		t.Fatalf("HeadCommit = %q, want %q", result.HeadCommit, headCommit)
	}
	if result.CommitCount != 2 {
		t.Fatalf("expected 2 commits, got %d", result.CommitCount)
	}
	if !strings.Contains(result.Patches, "Subject: [PATCH 1/2] Commit one") {
		t.Fatalf("expected first commit in patch bundle, got %q", result.Patches)
	}
	if !strings.Contains(result.Patches, "Subject: [PATCH 2/2] Commit two") {
		t.Fatalf("expected second commit in patch bundle, got %q", result.Patches)
	}
}

func TestGetCommitPatchesAtHead_EmptyTargetDerivesBaseForDetachedWorktree(t *testing.T) {
	originDir := filepath.Join(t.TempDir(), "origin.git")
	runGitCommand(t, "", "init", "--bare", originDir)

	seedRepo := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, "", "init", "-b", "main", seedRepo)
	runGitCommand(t, seedRepo, "config", "user.email", "test@example.com")
	runGitCommand(t, seedRepo, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(seedRepo, "file.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitCommand(t, seedRepo, "add", "file.txt")
	runGitCommand(t, seedRepo, "commit", "-m", "Base")
	runGitCommand(t, seedRepo, "remote", "add", "origin", originDir)
	runGitCommand(t, seedRepo, "push", "origin", "main")
	baseCommit := strings.TrimSpace(runGitCommand(t, seedRepo, "rev-parse", "HEAD"))

	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGitCommand(t, "", "clone", "--branch", "main", originDir, cloneDir)
	runGitCommand(t, cloneDir, "config", "user.email", "test@example.com")
	runGitCommand(t, cloneDir, "config", "user.name", "Test User")

	worktreeDir := filepath.Join(t.TempDir(), "prepared-worktree")
	runGitCommand(t, cloneDir, "worktree", "add", "--detach", worktreeDir, baseCommit)
	if err := os.WriteFile(filepath.Join(worktreeDir, "file.txt"), []byte("base\nprepared\n"), 0o644); err != nil {
		t.Fatalf("write prepared file: %v", err)
	}
	runGitCommand(t, worktreeDir, "add", "file.txt")
	runGitCommand(t, worktreeDir, "commit", "-m", "Prepared change")
	headCommit := strings.TrimSpace(runGitCommand(t, worktreeDir, "rev-parse", "HEAD"))

	result, commitsErr := GetCommitPatchesAtHead(worktreeDir, "", headCommit)
	if commitsErr != nil {
		t.Fatalf("GetCommitPatchesAtHead: %v", commitsErr)
	}
	if result.HeadCommit != headCommit {
		t.Fatalf("HeadCommit = %q, want %q", result.HeadCommit, headCommit)
	}
	if result.CommitCount != 1 {
		t.Fatalf("expected 1 commit, got %d", result.CommitCount)
	}
	if !strings.Contains(result.Patches, "Subject: [PATCH] Prepared change") {
		t.Fatalf("expected prepared commit in patch bundle, got %q", result.Patches)
	}
}

func TestListWorkspaceChangeCommitsReturnsDiffStat(t *testing.T) {
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	runGitCommand(t, repoDir, "commit", "-m", "base")
	base := strings.TrimSpace(runGitCommand(t, repoDir, "rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("base\none\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	tree := strings.TrimSpace(runGitCommand(t, repoDir, "write-tree"))
	commit := strings.TrimSpace(runGitCommand(t, repoDir, "commit-tree", tree, "-p", base, "-m", "workspace change"))
	runGitCommand(t, repoDir, "update-ref", "refs/discobot/workspace-change-commits/session-123/snapshot-1", commit)

	result, err := ListWorkspaceChangeCommits(repoDir, "session-123")
	if err != nil {
		t.Fatalf("ListWorkspaceChangeCommits: %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("commit count = %d, want 1", len(result.Commits))
	}
	got := result.Commits[0]
	if got.Hash != commit {
		t.Fatalf("hash = %q, want %q", got.Hash, commit)
	}
	if got.CreatedAt == "" {
		t.Fatal("expected createdAt to be populated")
	}
	if got.DiffStat.FilesChanged != 1 || got.DiffStat.Additions != 1 || got.DiffStat.Deletions != 0 {
		t.Fatalf("diffstat = %#v, want 1 file, 1 addition, 0 deletions", got.DiffStat)
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

func TestGetCommitPatchesAtHeadRejectsWorkspaceChangeCommits(t *testing.T) {
	repoDir := t.TempDir()
	runGitCommand(t, repoDir, "init")
	runGitCommand(t, repoDir, "config", "user.email", "test@example.com")
	runGitCommand(t, repoDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	runGitCommand(t, repoDir, "commit", "-m", "base")
	base := strings.TrimSpace(runGitCommand(t, repoDir, "rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("base\nsecret\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}
	runGitCommand(t, repoDir, "add", "file.txt")
	tree := strings.TrimSpace(runGitCommand(t, repoDir, "write-tree"))
	commit := strings.TrimSpace(runGitCommand(t, repoDir, "commit-tree", tree, "-p", base, "-m", "workspace change"))
	runGitCommand(t, repoDir, "update-ref", workspaceChangeCommitRefPrefix+"/session-123/change-1", commit)

	if result, commitsErr := GetCommitPatchesAtHead(repoDir, base, commit); commitsErr == nil {
		t.Fatalf("expected workspace change commit export to be rejected, got result %#v", result)
	} else if commitsErr.Code != "invalid_target" {
		t.Fatalf("error code = %q, want invalid_target", commitsErr.Code)
	}

	if result, commitsErr := GetCommitPatchesAtHead(repoDir, commit, base); commitsErr == nil {
		t.Fatalf("expected workspace change target export to be rejected, got result %#v", result)
	} else if commitsErr.Code != "invalid_target" {
		t.Fatalf("target error code = %q, want invalid_target", commitsErr.Code)
	}
}
