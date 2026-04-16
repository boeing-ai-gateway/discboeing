package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
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

func TestGetRequestCommitPullPreview_UsesRequestedDirectoryBaseAndCommit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)
	session := env.createTestSession(t, project.ID, workspace.ID, initialCommit)

	const (
		threadID         = "thread-1"
		questionID       = "approval-1"
		requestedDir     = "/tmp/discobot-commit-worktree"
		requestedBase    = "3526056ae5f926d742c49a686531fb0a33315853"
		requestedHead    = "5078ce9fa81e548c99f9682d26a66aff83876608"
		requestedSubject = "fix(ui): avoid duplicate credential keys"
	)

	var gotQuery map[string]string
	env.mockSandbox.HTTPHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/chat/question/"+questionID) && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&sandboxapi.PendingQuestionResponse{
				Status: "pending",
				Question: &sandboxapi.PendingQuestion{
					Context: requestCommitPullPreviewContext,
					Metadata: mustMarshalJSON(t, requestCommitPullQuestionMetadata{
						Directory:  requestedDir,
						BaseCommit: requestedBase,
						CommitHash: requestedHead,
					}),
				},
			})
		case r.URL.Path == "/commits" && r.Method == http.MethodGet:
			gotQuery = map[string]string{
				"target": r.URL.Query().Get("target"),
				"head":   r.URL.Query().Get("head"),
				"cwd":    r.URL.Query().Get("cwd"),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(&sandboxapi.CommitsResponse{
				Patches: strings.TrimSpace(`
From 5078ce9fa81e548c99f9682d26a66aff83876608 Mon Sep 17 00:00:00 2001
From: Example Author <author@example.com>
Date: Wed, 8 Apr 2026 17:54:25 +0000
Subject: [PATCH] fix(ui): avoid duplicate credential keys

---
 ui/src/lib/example.ts | 1 +
 1 file changed, 1 insertion(+)

diff --git a/ui/src/lib/example.ts b/ui/src/lib/example.ts
index e69de29..587be6b 100644
--- a/ui/src/lib/example.ts
+++ b/ui/src/lib/example.ts
@@ -0,0 +1 @@
+export const ready = true;
`),
				CommitCount: 1,
				HeadCommit:  requestedHead,
			})
		default:
			http.NotFound(w, r)
		}
	})

	_, err := env.mockSandbox.Create(context.Background(), session.ID, sandbox.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	if err := env.mockSandbox.Start(context.Background(), session.ID); err != nil {
		t.Fatalf("Failed to start sandbox: %v", err)
	}

	sandboxSvc := NewSandboxService(env.store, env.mockSandbox, &config.Config{}, nil, env.eventBroker, nil, nil)
	sandboxSvc.SetSessionInitializer(&testSessionInitializer{})
	chatSvc := &ChatService{
		store:          env.store,
		sandboxService: sandboxSvc,
	}

	preview, err := chatSvc.GetRequestCommitPullPreview(context.Background(), project.ID, session.ID, threadID, questionID)
	if err != nil {
		t.Fatalf("GetRequestCommitPullPreview returned error: %v", err)
	}
	if gotQuery["target"] != requestedBase || gotQuery["head"] != requestedHead || gotQuery["cwd"] != requestedDir {
		t.Fatalf("unexpected commits query: %#v", gotQuery)
	}
	if preview.HeadCommit != requestedHead {
		t.Fatalf("HeadCommit = %q, want %q", preview.HeadCommit, requestedHead)
	}
	if preview.CommitCount != 1 {
		t.Fatalf("CommitCount = %d, want 1", preview.CommitCount)
	}
	if len(preview.Commits) != 1 || preview.Commits[0].Subject != requestedSubject {
		t.Fatalf("unexpected preview commits: %#v", preview.Commits)
	}
}

func mustMarshalJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return data
}
