package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/obot-platform/discobot/agent-go/internal/config"
)

func TestSetupConfiguredWorkspaceClonesWorkspace(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	originDir := filepath.Join(t.TempDir(), "origin")
	runWorkspaceTestGit(t, "", "init", originDir)
	runWorkspaceTestGit(t, originDir, "checkout", "-b", "main")
	runWorkspaceTestGit(t, originDir, "config", "user.email", "test@example.com")
	runWorkspaceTestGit(t, originDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(originDir, "hello.txt"), []byte("hello\n"), 0600); err != nil {
		t.Fatalf("write origin file: %v", err)
	}
	runWorkspaceTestGit(t, originDir, "add", "hello.txt")
	runWorkspaceTestGit(t, originDir, "commit", "-m", "initial")

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("create empty workspace dir: %v", err)
	}
	var progress []string
	err := setupConfiguredWorkspace(context.Background(), &config.Config{
		AgentCwd:        workspaceDir,
		WorkspaceSource: originDir,
		WorkspaceRef:    "main",
	}, runtimeInitialCredentials{}, func(message string) {
		progress = append(progress, message)
	})
	if err != nil {
		t.Fatalf("setupConfiguredWorkspace: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workspaceDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read cloned file: %v", err)
	}
	if string(content) != "hello\n" {
		t.Fatalf("cloned file = %q, want hello", string(content))
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, ".git")); err != nil {
		t.Fatalf("workspace .git missing: %v", err)
	}
	wantProgress := []string{
		"preparing workspace clone",
		"cloning workspace",
		"finalizing workspace",
		"workspace ready",
	}
	if !reflect.DeepEqual(progress, wantProgress) {
		t.Fatalf("progress = %#v, want %#v", progress, wantProgress)
	}
}

func TestSetupConfiguredWorkspaceCreatesEmptyWorkspace(t *testing.T) {
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	var progress []string
	err := setupConfiguredWorkspace(context.Background(), &config.Config{AgentCwd: workspaceDir}, runtimeInitialCredentials{}, func(message string) {
		progress = append(progress, message)
	})
	if err != nil {
		t.Fatalf("setupConfiguredWorkspace: %v", err)
	}
	if info, err := os.Stat(workspaceDir); err != nil || !info.IsDir() {
		t.Fatalf("workspace dir not created: info=%v err=%v", info, err)
	}
	if !reflect.DeepEqual(progress, []string{"creating empty workspace"}) {
		t.Fatalf("progress = %#v", progress)
	}
}

func runWorkspaceTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
