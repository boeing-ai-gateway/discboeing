package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGetDiff_TargetCommitMatchesCurrentTree(t *testing.T) {
	repo := initDiffTestRepo(t)
	writeRepoFile(t, repo, "index.html", "<style>\n\n    body {}\n</style>\n")
	commitAll(t, repo, "Adjust spacing")
	target := headCommit(t, repo)

	result, err := GetDiff(repo, "index.html", target)
	if err != nil {
		t.Fatalf("GetDiff() error = %v", err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("expected no diff files, got %#v", result.Files)
	}
}

func TestGetDiff_TargetCommitShowsWorkingTreeChanges(t *testing.T) {
	repo := initDiffTestRepo(t)
	writeRepoFile(t, repo, "index.html", "<style>\nbody {}\n</style>\n")
	base := commitAll(t, repo, "Initial style")

	writeRepoFile(t, repo, "index.html", "<style>\n\n    body {}\n</style>\n")

	result, err := GetDiff(repo, "index.html", base)
	if err != nil {
		t.Fatalf("GetDiff() error = %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 diff file, got %d", len(result.Files))
	}
	if result.Files[0].Path != "index.html" {
		t.Fatalf("expected diff for index.html, got %q", result.Files[0].Path)
	}
	if result.Files[0].Status != "modified" {
		t.Fatalf("expected modified status, got %q", result.Files[0].Status)
	}
	if result.Files[0].Additions == 0 {
		t.Fatalf("expected additions in patch, got %#v", result.Files[0])
	}
}

func TestGetDiff_InvalidTarget(t *testing.T) {
	repo := initDiffTestRepo(t)
	writeRepoFile(t, repo, "index.html", "hello\n")
	commitAll(t, repo, "Initial")

	_, err := GetDiff(repo, "index.html", "does-not-exist")
	if err == nil {
		t.Fatal("expected invalid target error")
	}
}

func TestGetDiff_DefaultTargetUsesLocalMergeBaseWithoutFetch(t *testing.T) {
	originDir := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", originDir)

	seedRepo := initDiffTestRepo(t)
	writeRepoFile(t, seedRepo, "index.html", "base\n")
	commitAll(t, seedRepo, "Initial")
	runGit(t, seedRepo, "remote", "add", "origin", originDir)
	runGit(t, seedRepo, "push", "-u", "origin", "main")

	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGit(t, "", "clone", "--branch", "main", originDir, cloneDir)

	writeRepoFile(t, seedRepo, "index.html", "base\nremote\n")
	commitAll(t, seedRepo, "Remote change")
	runGit(t, seedRepo, "push", "origin", "main")

	writeRepoFile(t, cloneDir, "index.html", "base\nlocal\n")

	result, err := GetDiff(cloneDir, "index.html", "")
	if err != nil {
		t.Fatalf("GetDiff() error = %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 diff file, got %d", len(result.Files))
	}
	if got := result.Files[0].Patch; !containsLine(got, "+local") {
		t.Fatalf("expected patch to include local change, got:\n%s", got)
	}
	if got := result.Files[0].Patch; containsLine(got, "-remote") {
		t.Fatalf("expected patch to ignore unfetched remote change, got:\n%s", got)
	}
}

func TestGetDiff_DefaultTargetFallsBackToHeadWithoutUpstream(t *testing.T) {
	repo := initDiffTestRepo(t)
	writeRepoFile(t, repo, "index.html", "base\n")
	commitAll(t, repo, "Initial")
	writeRepoFile(t, repo, "index.html", "base\nlocal\n")

	result, err := GetDiff(repo, "index.html", "")
	if err != nil {
		t.Fatalf("GetDiff() error = %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 diff file, got %d", len(result.Files))
	}
	if got := result.Files[0].Patch; !containsLine(got, "+local") {
		t.Fatalf("expected patch to include local change, got:\n%s", got)
	}
}

func initDiffTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	return repo
}

func writeRepoFile(t *testing.T, repo, relPath, content string) {
	t.Helper()
	path := filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func commitAll(t *testing.T, repo, message string) string {
	t.Helper()
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", message)
	return headCommit(t, repo)
}

func headCommit(t *testing.T, repo string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD error = %v", err)
	}
	return string(out[:len(out)-1])
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, string(out))
	}
}

func containsLine(text, line string) bool {
	return slices.Contains(strings.Split(text, "\n"), line)
}
