package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasUsableProxyCertificateReturnsTrueForValidPair(t *testing.T) {
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "ca.crt")
	keyPath := filepath.Join(certDir, "ca.key")

	if err := generateCACertificate(certPath, keyPath); err != nil {
		t.Fatalf("generateCACertificate failed: %v", err)
	}

	usable, err := hasUsableProxyCertificate(certPath, keyPath)
	if err != nil {
		t.Fatalf("hasUsableProxyCertificate returned error: %v", err)
	}
	if !usable {
		t.Fatal("hasUsableProxyCertificate = false, want true")
	}
}

func TestHasUsableProxyCertificateReturnsFalseWhenKeyMissing(t *testing.T) {
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "ca.crt")
	keyPath := filepath.Join(certDir, "ca.key")

	if err := generateCACertificate(certPath, keyPath); err != nil {
		t.Fatalf("generateCACertificate failed: %v", err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("failed to remove key: %v", err)
	}

	usable, err := hasUsableProxyCertificate(certPath, keyPath)
	if err != nil {
		t.Fatalf("hasUsableProxyCertificate returned error: %v", err)
	}
	if usable {
		t.Fatal("hasUsableProxyCertificate = true, want false")
	}
}

func TestPEMFileContainsCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	otherCertPath := filepath.Join(dir, "other.crt")
	otherKeyPath := filepath.Join(dir, "other.key")
	bundlePath := filepath.Join(dir, "bundle.pem")

	if err := generateCACertificate(certPath, keyPath); err != nil {
		t.Fatalf("generateCACertificate(certPath) failed: %v", err)
	}
	if err := generateCACertificate(otherCertPath, otherKeyPath); err != nil {
		t.Fatalf("generateCACertificate(otherCertPath) failed: %v", err)
	}

	certData, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read certPath: %v", err)
	}
	otherCertData, err := os.ReadFile(otherCertPath)
	if err != nil {
		t.Fatalf("read otherCertPath: %v", err)
	}

	if err := os.WriteFile(bundlePath, append(otherCertData, certData...), 0o644); err != nil {
		t.Fatalf("write bundlePath: %v", err)
	}

	contains, err := pemFileContainsCertificate(bundlePath, certPath)
	if err != nil {
		t.Fatalf("pemFileContainsCertificate returned error: %v", err)
	}
	if !contains {
		t.Fatal("pemFileContainsCertificate = false, want true")
	}

	contains, err = pemFileContainsCertificate(bundlePath, otherCertPath)
	if err != nil {
		t.Fatalf("pemFileContainsCertificate(other) returned error: %v", err)
	}
	if !contains {
		t.Fatal("pemFileContainsCertificate(other) = false, want true")
	}
}

func TestPEMFileContainsCertificateReturnsFalseWhenMissing(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	otherCertPath := filepath.Join(dir, "other.crt")
	otherKeyPath := filepath.Join(dir, "other.key")
	bundlePath := filepath.Join(dir, "bundle.pem")

	if err := generateCACertificate(certPath, keyPath); err != nil {
		t.Fatalf("generateCACertificate(certPath) failed: %v", err)
	}
	if err := generateCACertificate(otherCertPath, otherKeyPath); err != nil {
		t.Fatalf("generateCACertificate(otherCertPath) failed: %v", err)
	}

	otherCertData, err := os.ReadFile(otherCertPath)
	if err != nil {
		t.Fatalf("read otherCertPath: %v", err)
	}
	if err := os.WriteFile(bundlePath, otherCertData, 0o644); err != nil {
		t.Fatalf("write bundlePath: %v", err)
	}

	contains, err := pemFileContainsCertificate(bundlePath, certPath)
	if err != nil {
		t.Fatalf("pemFileContainsCertificate returned error: %v", err)
	}
	if contains {
		t.Fatal("pemFileContainsCertificate = true, want false")
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

func TestBuildChildEnvUsesDiscobotSessionID(t *testing.T) {
	legacySessionEnv := "SESSION" + "_ID"
	t.Setenv(legacySessionEnv, "legacy-session")
	t.Setenv("DISCOBOT_SESSION_ID", "discobot-session")

	env := buildChildEnv(&userInfo{
		username: "discobot",
		homeDir:  "/home/discobot",
	}, false)

	values := map[string]string{}
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	if values[legacySessionEnv] != "" {
		t.Fatalf("%s = %q, want unset", legacySessionEnv, values[legacySessionEnv])
	}
	if values["DISCOBOT_SESSION_ID"] != "discobot-session" {
		t.Fatalf("DISCOBOT_SESSION_ID = %q, want discobot-session", values["DISCOBOT_SESSION_ID"])
	}
}

func TestSyncNewFilesPreservesExistingWorkspaceAndAddsMissingFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	workspaceSrc := filepath.Join(srcDir, "workspace")
	workspaceDst := filepath.Join(dstDir, "workspace")
	if err := os.MkdirAll(workspaceSrc, 0o755); err != nil {
		t.Fatalf("mkdir workspace src: %v", err)
	}
	if err := os.MkdirAll(workspaceDst, 0o755); err != nil {
		t.Fatalf("mkdir workspace dst: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceSrc, "README.md"), []byte("template\n"), 0o644); err != nil {
		t.Fatalf("write workspace src README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDst, "README.md"), []byte("persisted workspace\n"), 0o644); err != nil {
		t.Fatalf("write workspace dst README: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, ".bashrc"), []byte("template bashrc\n"), 0o644); err != nil {
		t.Fatalf("write src bashrc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".gitconfig"), []byte("[user]\n	name = Discobot\n"), 0o644); err != nil {
		t.Fatalf("write src gitconfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, ".bashrc"), []byte("custom bashrc\n"), 0o644); err != nil {
		t.Fatalf("write dst bashrc: %v", err)
	}

	u := &userInfo{uid: os.Getuid(), gid: os.Getgid()}
	if err := syncNewFiles(srcDir, dstDir, u); err != nil {
		t.Fatalf("syncNewFiles failed: %v", err)
	}

	workspaceReadme, err := os.ReadFile(filepath.Join(workspaceDst, "README.md"))
	if err != nil {
		t.Fatalf("read workspace dst README: %v", err)
	}
	if string(workspaceReadme) != "persisted workspace\n" {
		t.Fatalf("workspace README = %q, want persisted contents", string(workspaceReadme))
	}

	bashrc, err := os.ReadFile(filepath.Join(dstDir, ".bashrc"))
	if err != nil {
		t.Fatalf("read dst bashrc: %v", err)
	}
	if string(bashrc) != "custom bashrc\n" {
		t.Fatalf("bashrc = %q, want existing contents preserved", string(bashrc))
	}

	gitconfig, err := os.ReadFile(filepath.Join(dstDir, ".gitconfig"))
	if err != nil {
		t.Fatalf("read dst gitconfig: %v", err)
	}
	if string(gitconfig) != "[user]\n	name = Discobot\n" {
		t.Fatalf("gitconfig = %q, want new file copied from source", string(gitconfig))
	}
}

func TestRemoveObsoleteBundledHomeConfig(t *testing.T) {
	homeDir := t.TempDir()
	scriptsDir := filepath.Join(homeDir, ".discobot", "scripts")
	commandsDir := filepath.Join(homeDir, ".discobot", "commands")
	skillsDir := filepath.Join(homeDir, ".discobot", "skills", "browser-harness")
	legacyCommandsDir := filepath.Join(homeDir, ".claude", "commands")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("mkdir commands dir: %v", err)
	}
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}
	if err := os.MkdirAll(legacyCommandsDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy commands dir: %v", err)
	}

	for _, name := range []string{"discobot-commit", "discobot-commit-remote", "discobot-rebase"} {
		if err := os.WriteFile(filepath.Join(scriptsDir, name), []byte("legacy\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for _, name := range []string{"discobot-commit.md", "discobot-commit-remote.md", "discobot-rebase.md"} {
		if err := os.WriteFile(filepath.Join(commandsDir, name), []byte("legacy\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("legacy\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyCommandsDir, "custom.md"), []byte("legacy\n"), 0o644); err != nil {
		t.Fatalf("write legacy command: %v", err)
	}

	if err := removeObsoleteBundledHomeConfig(homeDir); err != nil {
		t.Fatalf("removeObsoleteBundledHomeConfig failed: %v", err)
	}

	for _, name := range []string{"discobot-commit", "discobot-commit-remote", "discobot-rebase"} {
		if _, err := os.Stat(filepath.Join(scriptsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", name, err)
		}
	}
	for _, name := range []string{"discobot-commit.md", "discobot-commit-remote.md", "discobot-rebase.md"} {
		if _, err := os.Stat(filepath.Join(commandsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("expected browser-harness skill to be removed, got err=%v", err)
	}
	if _, err := os.Stat(legacyCommandsDir); !os.IsNotExist(err) {
		t.Fatalf("expected legacy commands dir removed, got err=%v", err)
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
