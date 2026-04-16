package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
)

func TestExecuteRequestCommitPull(t *testing.T) {
	repo := initRequestCommitPullRepo(t)
	e := New(repo, t.TempDir(), t.Name())
	e.setCwd(filepath.Join(repo, "subdir"))

	result, err := e.executeRequestCommitPull(message.ToolCallPart{
		ToolCallID: "tc1",
		ToolName:   "RequestCommitPull",
		Input:      `{"notes":"ready to apply"}`,
	})
	if err != nil {
		t.Fatalf("executeRequestCommitPull returned error: %v", err)
	}
	if result.Approval == nil {
		t.Fatal("expected approval request")
	}
	if result.Approval.Context != requestCommitPullApprovalContext {
		t.Fatalf("approval context = %q", result.Approval.Context)
	}

	var questions []api.AskUserQuestion
	if err := json.Unmarshal(result.Approval.Questions, &questions); err != nil {
		t.Fatalf("unmarshal questions: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}

	var metadata requestCommitPullMetadata
	if err := json.Unmarshal(result.Approval.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	wantDir := filepath.ToSlash(filepath.Join(repo, "subdir"))
	if metadata.Directory != wantDir {
		t.Fatalf("directory = %q, want %q", metadata.Directory, wantDir)
	}
	if metadata.CommitHash == "" {
		t.Fatal("expected commit hash")
	}
	if metadata.BaseCommit == "" {
		t.Fatal("expected base commit")
	}
	if metadata.CommitTitle != "initial" {
		t.Fatalf("commit title = %q", metadata.CommitTitle)
	}
	if metadata.CommitBody != "commit body" {
		t.Fatalf("commit body = %q", metadata.CommitBody)
	}
	wantNotes := "ready to apply"
	if questions[0].Notes != wantNotes {
		t.Fatalf("notes = %q, want %q", questions[0].Notes, wantNotes)
	}
}

func TestExecuteRequestCommitPull_UsesExplicitBaseCommit(t *testing.T) {
	repo := initRequestCommitPullRepo(t)
	e := New(repo, t.TempDir(), t.Name())

	baseCommit := strings.TrimSpace(gitOutputForTest(t, repo, "rev-parse", "HEAD"))
	result, err := e.executeRequestCommitPull(message.ToolCallPart{
		ToolCallID: "tc1",
		ToolName:   "RequestCommitPull",
		Input:      `{"baseCommit":"` + baseCommit + `"}`,
	})
	if err != nil {
		t.Fatalf("executeRequestCommitPull returned error: %v", err)
	}

	var metadata requestCommitPullMetadata
	if err := json.Unmarshal(result.Approval.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata.BaseCommit != baseCommit {
		t.Fatalf("baseCommit = %q, want %q", metadata.BaseCommit, baseCommit)
	}
}

func TestResolveRequestCommitPull(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	t.Run("success result", func(t *testing.T) {
		result, err := e.resolveRequestCommitPull(message.ToolCallPart{
			ToolCallID: "tc1",
			ToolName:   "RequestCommitPull",
		}, api.AnswerQuestionRequest{
			Answers: map[string]string{
				requestCommitPullSucceededKey: "true",
				requestCommitPullResultKey:    "Applied workspace commit: abc123",
			},
		})
		if err != nil {
			t.Fatalf("resolveRequestCommitPull returned error: %v", err)
		}
		text, ok := result.Output.(message.TextOutput)
		if !ok {
			t.Fatalf("output type = %T", result.Output)
		}
		if text.Value != "Applied workspace commit: abc123" {
			t.Fatalf("text = %q", text.Value)
		}
	})

	t.Run("failure result", func(t *testing.T) {
		result, err := e.resolveRequestCommitPull(message.ToolCallPart{
			ToolCallID: "tc1",
			ToolName:   "RequestCommitPull",
		}, api.AnswerQuestionRequest{
			Answers: map[string]string{
				requestCommitPullFailedKey: "true",
				requestCommitPullResultKey: "Commit hash did not match sandbox head",
			},
		})
		if err != nil {
			t.Fatalf("resolveRequestCommitPull returned error: %v", err)
		}
		text, ok := result.Output.(message.TextOutput)
		if !ok {
			t.Fatalf("output type = %T", result.Output)
		}
		if text.Value != "Commit hash did not match sandbox head" {
			t.Fatalf("text = %q", text.Value)
		}
	})
}

func initRequestCommitPullRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")
	if err := os.Mkdir(filepath.Join(repo, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "subdir", "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial", "-m", "commit body")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func gitOutputForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}
