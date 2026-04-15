package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSandboxSSHKeyFiles(t *testing.T) {
	srcDir := t.TempDir()
	homeDir := t.TempDir()

	privateKey := []byte("PRIVATE KEY DATA\n")
	publicKey := []byte("ecdsa-sha2-nistp256 AAAATEST discobot\n")

	if err := os.WriteFile(filepath.Join(srcDir, sandboxSSHKeyName), privateKey, 0600); err != nil {
		t.Fatalf("failed to write staged private key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, sandboxSSHKeyName+".pub"), publicKey, 0644); err != nil {
		t.Fatalf("failed to write staged public key: %v", err)
	}

	if err := installSandboxSSHKeyFiles(srcDir, homeDir, -1, -1); err != nil {
		t.Fatalf("installSandboxSSHKeyFiles failed: %v", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	privateDst := filepath.Join(sshDir, sandboxSSHKeyName)
	publicDst := filepath.Join(sshDir, sandboxSSHKeyName+".pub")

	privateData, err := os.ReadFile(privateDst)
	if err != nil {
		t.Fatalf("failed to read installed private key: %v", err)
	}
	if string(privateData) != string(privateKey) {
		t.Fatalf("unexpected private key contents: %q", string(privateData))
	}

	publicData, err := os.ReadFile(publicDst)
	if err != nil {
		t.Fatalf("failed to read installed public key: %v", err)
	}
	if string(publicData) != string(publicKey) {
		t.Fatalf("unexpected public key contents: %q", string(publicData))
	}

	sshInfo, err := os.Stat(sshDir)
	if err != nil {
		t.Fatalf("failed to stat ssh dir: %v", err)
	}
	if got := sshInfo.Mode().Perm(); got != 0700 {
		t.Fatalf("ssh dir mode = %o, want 700", got)
	}

	privateInfo, err := os.Stat(privateDst)
	if err != nil {
		t.Fatalf("failed to stat installed private key: %v", err)
	}
	if got := privateInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("private key mode = %o, want 600", got)
	}

	publicInfo, err := os.Stat(publicDst)
	if err != nil {
		t.Fatalf("failed to stat installed public key: %v", err)
	}
	if got := publicInfo.Mode().Perm(); got != 0644 {
		t.Fatalf("public key mode = %o, want 644", got)
	}

	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir to be removed, err=%v", err)
	}

	if _, err := os.Stat(filepath.Join(sshDir, "id_ecdsa")); !os.IsNotExist(err) {
		t.Fatalf("expected default ssh identity to remain absent, err=%v", err)
	}
}

func TestInstallSandboxSSHKeyFilesSkipsWhenNoKeyStaged(t *testing.T) {
	srcDir := t.TempDir()
	homeDir := t.TempDir()

	if err := installSandboxSSHKeyFiles(srcDir, homeDir, -1, -1); err != nil {
		t.Fatalf("installSandboxSSHKeyFiles returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(homeDir, ".ssh")); !os.IsNotExist(err) {
		t.Fatalf("expected .ssh dir to remain absent, err=%v", err)
	}
}

func TestEnsureBranchTracksOrigin(t *testing.T) {
	originDir := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", originDir)

	sourceDir := t.TempDir()
	runGit(t, sourceDir, "init")
	runGit(t, sourceDir, "config", "user.name", "Discobot Test")
	runGit(t, sourceDir, "config", "user.email", "discobot@example.com")
	runGit(t, sourceDir, "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("failed to write README.md: %v", err)
	}
	runGit(t, sourceDir, "add", "README.md")
	runGit(t, sourceDir, "commit", "-m", "initial commit")
	runGit(t, sourceDir, "remote", "add", "origin", originDir)
	runGit(t, sourceDir, "push", "-u", "origin", "main")
	runGit(t, originDir, "symbolic-ref", "HEAD", "refs/heads/main")

	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGit(t, "", "clone", "--single-branch", originDir, cloneDir)
	runGit(t, cloneDir, "branch", "--unset-upstream")

	branchName, err := currentBranchName(cloneDir)
	if err != nil {
		t.Fatalf("currentBranchName failed: %v", err)
	}
	if branchName != "main" {
		t.Fatalf("currentBranchName = %q, want %q", branchName, "main")
	}

	if err := ensureBranchTracksOrigin(cloneDir, branchName); err != nil {
		t.Fatalf("ensureBranchTracksOrigin failed: %v", err)
	}

	upstream := strings.TrimSpace(runGit(t, cloneDir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"))
	if upstream != "origin/main" {
		t.Fatalf("upstream = %q, want %q", upstream, "origin/main")
	}
}

func TestBranchNameFromTargetRef(t *testing.T) {
	tests := []struct {
		targetRef string
		want      string
	}{
		{targetRef: "", want: ""},
		{targetRef: "HEAD", want: ""},
		{targetRef: "main", want: "main"},
		{targetRef: "refs/heads/release", want: "release"},
		{targetRef: "origin/main", want: ""},
		{targetRef: "refs/tags/v1.0.0", want: ""},
	}

	for _, tt := range tests {
		if got := branchNameFromTargetRef(tt.targetRef); got != tt.want {
			t.Fatalf("branchNameFromTargetRef(%q) = %q, want %q", tt.targetRef, got, tt.want)
		}
	}
}

func TestBuildWorkspaceCloneArgsUsesBranchAndMirror(t *testing.T) {
	got := buildWorkspaceCloneArgs("https://example.com/repo.git", "refs/heads/main", "/cache/repo.git")
	want := []string{
		"clone",
		"--single-branch",
		"--branch",
		"main",
		"--reference-if-able",
		"/cache/repo.git",
		"https://example.com/repo.git",
		stagingDir,
	}

	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("buildWorkspaceCloneArgs() = %v, want %v", got, want)
	}
}

func TestPersistentCachePath(t *testing.T) {
	got := persistentCachePath("/home/discobot/.cache/discobot/git")
	want := filepath.Join(dataDir, "cache", "home/discobot/.cache/discobot/git")
	if got != want {
		t.Fatalf("persistentCachePath() = %q, want %q", got, want)
	}
}

func TestInstallCommitCommandVariant(t *testing.T) {
	homeDir := t.TempDir()
	commandsDir := filepath.Join(homeDir, ".discobot", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("mkdir commands dir: %v", err)
	}

	defaultBody := "default command\n"
	remoteBody := "remote command\n"
	defaultPath := filepath.Join(commandsDir, "discobot-commit.md")
	if err := os.WriteFile(defaultPath, []byte(defaultBody), 0o644); err != nil {
		t.Fatalf("write default command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "discobot-commit-remote.md"), []byte(remoteBody), 0o644); err != nil {
		t.Fatalf("write remote command: %v", err)
	}

	if err := installCommitCommandVariant(homeDir, false, nil); err != nil {
		t.Fatalf("installCommitCommandVariant(false) failed: %v", err)
	}

	got, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("read default command after local install: %v", err)
	}
	if string(got) != defaultBody {
		t.Fatalf("local installed command = %q, want %q", string(got), defaultBody)
	}

	if err := installCommitCommandVariant(homeDir, true, nil); err != nil {
		t.Fatalf("installCommitCommandVariant(true) failed: %v", err)
	}

	got, err = os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("read installed command: %v", err)
	}
	if string(got) != remoteBody {
		t.Fatalf("installed command = %q, want %q", string(got), remoteBody)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
