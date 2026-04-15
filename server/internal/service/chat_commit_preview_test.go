package service

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/server/internal/git"
	"github.com/obot-platform/discobot/server/internal/model"
)

func TestParseCommitPullPreview(t *testing.T) {
	rawPatch := strings.TrimSpace(`
From 1111111111111111111111111111111111111111 Mon Sep 17 00:00:00 2001
From: Example Author <author@example.com>
Date: Wed, 8 Apr 2026 17:54:25 +0000
Subject: [PATCH 1/2] Add greeting

Create the greeting file.

Signed-off-by: Example Author <author@example.com>
---
 hello.txt | 2 ++
 1 file changed, 2 insertions(+)
 create mode 100644 hello.txt

diff --git a/hello.txt b/hello.txt
new file mode 100644
index 0000000..ce01362
--- /dev/null
+++ b/hello.txt
@@ -0,0 +1,2 @@
+hello
+world

From 2222222222222222222222222222222222222222 Mon Sep 17 00:00:00 2001
From: Example Author <author@example.com>
Date: Wed, 8 Apr 2026 17:55:25 +0000
Subject: [PATCH 2/2] Rename greeting

Move the file into the app folder.
---
 hello.txt => app/hello.txt | 0
 1 file changed, 0 insertions(+), 0 deletions(-)
 rename hello.txt => app/hello.txt (100%)

diff --git a/hello.txt b/app/hello.txt
similarity index 100%
rename from hello.txt
rename to app/hello.txt
`)

	preview, err := parseCommitPullPreview(rawPatch, "2222222222222222222222222222222222222222")
	if err != nil {
		t.Fatalf("parseCommitPullPreview returned error: %v", err)
	}

	if preview.CommitCount != 2 {
		t.Fatalf("expected 2 commits, got %d", preview.CommitCount)
	}
	if preview.Stats.FilesChanged != 2 {
		t.Fatalf("expected 2 files changed, got %d", preview.Stats.FilesChanged)
	}
	if preview.Stats.Additions != 2 || preview.Stats.Deletions != 0 {
		t.Fatalf("unexpected aggregate stats: %+v", preview.Stats)
	}

	first := preview.Commits[0]
	if first.Subject != "Add greeting" {
		t.Fatalf("expected stripped subject, got %q", first.Subject)
	}
	if first.AuthorName != "Example Author" || first.AuthorEmail != "author@example.com" {
		t.Fatalf("unexpected author: %+v", first)
	}
	if len(first.SignedOffBy) != 1 || first.SignedOffBy[0] != "Example Author <author@example.com>" {
		t.Fatalf("unexpected signed-off-by trailers: %#v", first.SignedOffBy)
	}
	if len(first.Files) != 1 {
		t.Fatalf("expected first commit to have 1 file, got %d", len(first.Files))
	}
	if first.Files[0].Status != "added" || first.Files[0].Path != "hello.txt" {
		t.Fatalf("unexpected first file: %+v", first.Files[0])
	}

	second := preview.Commits[1]
	if len(second.Files) != 1 {
		t.Fatalf("expected second commit to have 1 file, got %d", len(second.Files))
	}
	if second.Files[0].Status != "renamed" {
		t.Fatalf("expected renamed file status, got %+v", second.Files[0])
	}
	if second.Files[0].OldPath != "hello.txt" || second.Files[0].Path != "app/hello.txt" {
		t.Fatalf("unexpected renamed paths: %+v", second.Files[0])
	}
}

func TestResolveCommitPullPreviewTargetCommit(t *testing.T) {
	ctx := context.Background()
	repoDir := initPreviewTargetRepo(t)

	provider, err := git.NewLocalProvider(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalProvider: %v", err)
	}
	_, headCommit, err := provider.EnsureWorkspace(ctx, "project-1", "workspace-1", repoDir, "")
	if err != nil {
		t.Fatalf("EnsureWorkspace: %v", err)
	}

	chatService := &ChatService{gitService: NewGitService(nil, provider)}

	t.Run("resolves default HEAD target ref", func(t *testing.T) {
		got, err := chatService.resolveCommitPullPreviewTargetCommit(ctx, &model.Session{WorkspaceID: "workspace-1"})
		if err != nil {
			t.Fatalf("resolveCommitPullPreviewTargetCommit returned error: %v", err)
		}
		if got != headCommit {
			t.Fatalf("expected target commit %q, got %q", headCommit, got)
		}
	})

	t.Run("rejects unsupported target refs", func(t *testing.T) {
		targetRef := "origin/main"
		_, err := chatService.resolveCommitPullPreviewTargetCommit(ctx, &model.Session{
			WorkspaceID: "workspace-1",
			TargetRef:   &targetRef,
		})
		if err == nil {
			t.Fatal("expected error for unsupported target ref")
		}
		if !strings.Contains(err.Error(), "unsupported target ref") {
			t.Fatalf("expected unsupported target ref error, got %v", err)
		}
	})
}

func initPreviewTargetRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Preview Tester")
	runGit("config", "user.email", "preview@example.com")
	if err := os.WriteFile(repoDir+"/hello.txt", []byte("hello\n"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit("add", "hello.txt")
	runGit("commit", "-m", "initial commit")

	return repoDir
}
