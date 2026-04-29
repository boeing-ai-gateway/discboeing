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

func TestRefreshBundledScriptsOverwritesBundledFilesOnly(t *testing.T) {
	srcHome := t.TempDir()
	dstHome := t.TempDir()
	srcScripts := filepath.Join(srcHome, ".discobot", "scripts")
	dstScripts := filepath.Join(dstHome, ".discobot", "scripts")
	if err := os.MkdirAll(srcScripts, 0o755); err != nil {
		t.Fatalf("mkdir src scripts: %v", err)
	}
	if err := os.MkdirAll(dstScripts, 0o755); err != nil {
		t.Fatalf("mkdir dst scripts: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcScripts, "discobot-commit"), []byte("new script\n"), 0o755); err != nil {
		t.Fatalf("write src script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstScripts, "discobot-commit"), []byte("old script\n"), 0o755); err != nil {
		t.Fatalf("write dst script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstHome, ".bashrc"), []byte("custom bashrc\n"), 0o644); err != nil {
		t.Fatalf("write dst bashrc: %v", err)
	}

	u := &userInfo{uid: os.Getuid(), gid: os.Getgid()}
	if err := refreshBundledScripts(srcHome, dstHome, u); err != nil {
		t.Fatalf("refreshBundledScripts failed: %v", err)
	}

	commandBody, err := os.ReadFile(filepath.Join(dstScripts, "discobot-commit"))
	if err != nil {
		t.Fatalf("read refreshed script: %v", err)
	}
	if string(commandBody) != "new script\n" {
		t.Fatalf("script body = %q, want refreshed contents", string(commandBody))
	}

	bashrc, err := os.ReadFile(filepath.Join(dstHome, ".bashrc"))
	if err != nil {
		t.Fatalf("read dst bashrc: %v", err)
	}
	if string(bashrc) != "custom bashrc\n" {
		t.Fatalf("bashrc = %q, want unrelated file unchanged", string(bashrc))
	}
}

func TestRefreshBundledCommandsCreatesCommandsWhenThreadsDirExists(t *testing.T) {
	srcHome := t.TempDir()
	dstHome := t.TempDir()
	srcCommands := filepath.Join(srcHome, ".discobot", "commands")
	threadsDir := filepath.Join(dstHome, ".discobot", "threads", "thread-1")
	if err := os.MkdirAll(srcCommands, 0o755); err != nil {
		t.Fatalf("mkdir src commands: %v", err)
	}
	if err := os.MkdirAll(threadsDir, 0o755); err != nil {
		t.Fatalf("mkdir threads dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcCommands, "discobot-commit.md"), []byte("new command\n"), 0o644); err != nil {
		t.Fatalf("write src command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(threadsDir, "state.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write thread state: %v", err)
	}

	u := &userInfo{uid: os.Getuid(), gid: os.Getgid()}
	if err := refreshBundledCommands(srcHome, dstHome, u); err != nil {
		t.Fatalf("refreshBundledCommands failed: %v", err)
	}

	commandBody, err := os.ReadFile(filepath.Join(dstHome, ".discobot", "commands", "discobot-commit.md"))
	if err != nil {
		t.Fatalf("read refreshed command: %v", err)
	}
	if string(commandBody) != "new command\n" {
		t.Fatalf("command body = %q, want refreshed contents", string(commandBody))
	}

	threadState, err := os.ReadFile(filepath.Join(threadsDir, "state.json"))
	if err != nil {
		t.Fatalf("read thread state: %v", err)
	}
	if string(threadState) != "{}\n" {
		t.Fatalf("thread state = %q, want existing contents preserved", string(threadState))
	}
}

func TestRefreshBundledCommandsRemovesLegacyClaudeCommands(t *testing.T) {
	srcHome := t.TempDir()
	dstHome := t.TempDir()
	srcCommands := filepath.Join(srcHome, ".discobot", "commands")
	legacyCommands := filepath.Join(dstHome, ".claude", "commands")
	if err := os.MkdirAll(srcCommands, 0o755); err != nil {
		t.Fatalf("mkdir src commands: %v", err)
	}
	if err := os.MkdirAll(legacyCommands, 0o755); err != nil {
		t.Fatalf("mkdir legacy commands: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcCommands, "discobot-commit.md"), []byte("new command\n"), 0o644); err != nil {
		t.Fatalf("write src default command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyCommands, "discobot-commit.md"), []byte("old command\n"), 0o644); err != nil {
		t.Fatalf("write legacy command: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyCommands, "custom.md"), []byte("custom command\n"), 0o644); err != nil {
		t.Fatalf("write legacy custom command: %v", err)
	}

	u := &userInfo{uid: os.Getuid(), gid: os.Getgid()}
	if err := refreshBundledCommands(srcHome, dstHome, u); err != nil {
		t.Fatalf("refreshBundledCommands failed: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(dstHome, ".discobot", "commands", "discobot-commit.md"))
	if err != nil {
		t.Fatalf("read discobot command: %v", err)
	}
	if string(body) != "new command\n" {
		t.Fatalf("discobot command = %q, want refreshed contents", string(body))
	}

	if _, err := os.Stat(legacyCommands); !os.IsNotExist(err) {
		t.Fatalf("expected legacy commands dir removed, err=%v", err)
	}
}

func TestInstallCommitCommandVariant(t *testing.T) {
	homeDir := t.TempDir()
	scriptsDir := filepath.Join(homeDir, ".discobot", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}

	defaultBody := "default script\n"
	remoteBody := "remote script\n"
	defaultPath := filepath.Join(scriptsDir, "discobot-commit")
	remotePath := filepath.Join(scriptsDir, "discobot-commit-remote")
	if err := os.WriteFile(defaultPath, []byte(defaultBody), 0o755); err != nil {
		t.Fatalf("write default script: %v", err)
	}
	if err := os.WriteFile(remotePath, []byte(remoteBody), 0o755); err != nil {
		t.Fatalf("write remote script: %v", err)
	}

	if err := installCommitCommandVariant(homeDir, false, nil); err != nil {
		t.Fatalf("installCommitCommandVariant(false) failed: %v", err)
	}

	got, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("read default script after local install: %v", err)
	}
	if string(got) != defaultBody {
		t.Fatalf("local installed script = %q, want %q", string(got), defaultBody)
	}
	if _, err := os.Stat(remotePath); !os.IsNotExist(err) {
		t.Fatalf("expected remote variant to be removed after install, err=%v", err)
	}

	if err := os.WriteFile(remotePath, []byte(remoteBody), 0o755); err != nil {
		t.Fatalf("rewrite remote script: %v", err)
	}
	if err := installCommitCommandVariant(homeDir, true, nil); err != nil {
		t.Fatalf("installCommitCommandVariant(true) failed: %v", err)
	}

	got, err = os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("read installed script: %v", err)
	}
	if string(got) != remoteBody {
		t.Fatalf("installed script = %q, want %q", string(got), remoteBody)
	}
	if _, err := os.Stat(remotePath); !os.IsNotExist(err) {
		t.Fatalf("expected remote variant to be removed after remote install, err=%v", err)
	}
}

func TestRemoveLegacyBundledCommands(t *testing.T) {
	homeDir := t.TempDir()
	commandsDir := filepath.Join(homeDir, ".discobot", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("mkdir commands dir: %v", err)
	}

	for _, name := range []string{"discobot-commit.md", "discobot-commit-remote.md", "discobot-rebase.md"} {
		if err := os.WriteFile(filepath.Join(commandsDir, name), []byte("legacy\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if err := removeLegacyBundledCommands(homeDir); err != nil {
		t.Fatalf("removeLegacyBundledCommands failed: %v", err)
	}

	for _, name := range []string{"discobot-commit.md", "discobot-commit-remote.md", "discobot-rebase.md"} {
		if _, err := os.Stat(filepath.Join(commandsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", name, err)
		}
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
