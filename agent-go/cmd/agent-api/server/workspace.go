package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/config"
	"github.com/obot-platform/discobot/agent-go/internal/credentials"
)

const agentDataDir = "/.data"

type workspaceSetupProgress func(message string)

var (
	gitBasicAuthURLPattern   = regexp.MustCompile(`(?i)(https?://)([^/\s@]+)@`)
	gitSensitiveQueryPattern = regexp.MustCompile(`(?i)([?&](?:token|access_token|api_key|password|secret)=)[^&\s]+`)
)

// setupConfiguredWorkspace prepares the agent working tree after dynamic
// configuration arrives. The init process may have created an empty workspace
// directory already, but it no longer receives enough env to clone.
func setupConfiguredWorkspace(ctx context.Context, cfg *config.Config, initialCreds runtimeInitialCredentials, progress workspaceSetupProgress) error {
	emit := func(message string) {
		if progress != nil {
			progress(message)
		}
	}

	workspaceDir := strings.TrimSpace(cfg.AgentCwd)
	if workspaceDir == "" {
		return fmt.Errorf("agent workspace directory is empty")
	}

	cloneSource := configuredWorkspaceCloneSource(cfg)
	if cloneSource == "" {
		emit("creating empty workspace")
		return os.MkdirAll(workspaceDir, 0755)
	}

	if workspaceGitDirExists(workspaceDir) {
		emit("workspace already configured")
		return nil
	}
	// The init agent creates an empty workspace before overlayfs is mounted. If
	// something else wrote files here, avoid deleting it implicitly.
	if ok, err := workspaceDirectoryReadyForClone(workspaceDir); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("workspace directory %s already exists and is not empty", workspaceDir)
	}

	emit("preparing workspace clone")
	stagingDir := workspaceDir + ".staging"
	if err := os.RemoveAll(stagingDir); err != nil {
		return fmt.Errorf("failed to remove workspace staging directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(workspaceDir), 0755); err != nil {
		return fmt.Errorf("failed to create workspace parent directory: %w", err)
	}

	mirrorDir := ""
	if isGitURL(cloneSource) {
		var err error
		emit("preparing git cache")
		mirrorDir, err = ensureGitMirrorCache(ctx, cloneSource, initialCreds.Credentials)
		if err != nil {
			return err
		}
	}

	emit("cloning workspace")
	cloneArgs := buildWorkspaceCloneArgs(cloneSource, cfg.WorkspaceRef, mirrorDir, stagingDir)
	if err := runGit(ctx, "", initialCreds.Credentials, cloneArgs...); err != nil {
		if cleanupErr := os.RemoveAll(stagingDir); cleanupErr != nil {
			return fmt.Errorf("git clone failed: %w; failed to remove workspace staging directory: %v", err, cleanupErr)
		}
		return fmt.Errorf("git clone failed: %w", err)
	}

	branchName, err := currentBranchName(ctx, stagingDir)
	if err != nil {
		return err
	}

	if strings.TrimSpace(cfg.WorkspaceCommit) != "" {
		emit("checking out workspace commit")
		if err := runGit(ctx, stagingDir, initialCreds.Credentials, "reset", "--hard", cfg.WorkspaceCommit); err != nil {
			return fmt.Errorf("git reset --hard %s failed: %w", cfg.WorkspaceCommit, err)
		}
	} else if shouldResetWorkspaceToTargetRef(cfg.WorkspaceRef) {
		emit("checking out workspace ref")
		if err := runGit(ctx, stagingDir, initialCreds.Credentials, "reset", "--hard", cfg.WorkspaceRef); err != nil {
			return fmt.Errorf("git reset --hard %s failed: %w", cfg.WorkspaceRef, err)
		}
	}

	if err := ensureBranchTracksOrigin(ctx, stagingDir, branchName); err != nil {
		return err
	}

	emit("finalizing workspace")
	if err := moveWorkspaceContents(stagingDir, workspaceDir); err != nil {
		return err
	}

	emit("workspace ready")
	return nil
}

func moveWorkspaceContents(stagingDir, workspaceDir string) error {
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return fmt.Errorf("failed to read workspace staging directory: %w", err)
	}
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	for _, entry := range entries {
		from := filepath.Join(stagingDir, entry.Name())
		to := filepath.Join(workspaceDir, entry.Name())
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("failed to move workspace entry %s into place: %w", entry.Name(), err)
		}
	}
	if err := os.Remove(stagingDir); err != nil {
		return fmt.Errorf("failed to remove workspace staging directory: %w", err)
	}
	return nil
}

func configuredWorkspaceCloneSource(cfg *config.Config) string {
	workspaceSource := strings.TrimSpace(cfg.WorkspaceSource)
	if isGitURL(workspaceSource) {
		return workspaceSource
	}
	// Local workspaces are mounted at WorkspaceOrigin. WorkspaceSource is the
	// host path, which may not exist inside the sandbox.
	if workspaceOrigin := strings.TrimSpace(cfg.WorkspaceOrigin); workspaceOrigin != "" {
		return workspaceOrigin
	}
	return workspaceSource
}

func workspaceGitDirExists(workspaceDir string) bool {
	_, err := os.Stat(filepath.Join(workspaceDir, ".git"))
	return err == nil
}

func workspaceDirectoryReadyForClone(workspaceDir string) (bool, error) {
	entries, err := os.ReadDir(workspaceDir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read workspace directory: %w", err)
	}
	return len(entries) == 0, nil
}

func buildWorkspaceCloneArgs(cloneSource, workspaceTargetRef, mirrorDir, destination string) []string {
	args := []string{"clone"}
	if safeDirectory := gitSafeDirectoryForCloneSource(cloneSource); safeDirectory != "" {
		args = []string{"-c", "safe.directory=" + safeDirectory, "clone"}
	}
	if branch := branchNameFromTargetRef(workspaceTargetRef); branch != "" {
		args = append(args, "--single-branch", "--branch", branch)
	} else if strings.TrimSpace(workspaceTargetRef) == "" || strings.TrimSpace(workspaceTargetRef) == "HEAD" {
		args = append(args, "--single-branch")
	}
	if mirrorDir != "" {
		args = append(args, "--reference-if-able", mirrorDir)
	}
	return append(args, cloneSource, destination)
}

func gitSafeDirectoryForCloneSource(cloneSource string) string {
	if isGitURL(cloneSource) {
		return ""
	}
	if strings.HasPrefix(cloneSource, "/") {
		return path.Clean(cloneSource)
	}
	if filepath.IsAbs(cloneSource) {
		return filepath.Clean(cloneSource)
	}
	return ""
}

func shouldResetWorkspaceToTargetRef(targetRef string) bool {
	targetRef = strings.TrimSpace(targetRef)
	if targetRef == "" || targetRef == "HEAD" {
		return false
	}
	return branchNameFromTargetRef(targetRef) == ""
}

func branchNameFromTargetRef(targetRef string) string {
	targetRef = strings.TrimSpace(targetRef)
	if targetRef == "" || targetRef == "HEAD" {
		return ""
	}
	if after, ok := strings.CutPrefix(targetRef, "refs/heads/"); ok {
		return after
	}
	if strings.HasPrefix(targetRef, "refs/") || strings.Contains(targetRef, "/") {
		return ""
	}
	return targetRef
}

func isGitURL(source string) bool {
	source = strings.TrimSpace(source)
	if source == "" {
		return false
	}
	return strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "ssh://") ||
		strings.HasPrefix(source, "git@")
}

func ensureGitMirrorCache(ctx context.Context, cloneSource string, creds []credentials.EnvVar) (string, error) {
	cacheBase := persistentCachePath("/home/discobot/.cache/discobot/git")
	if err := os.MkdirAll(cacheBase, 0777); err != nil {
		return "", fmt.Errorf("failed to create git cache directory: %w", err)
	}

	mirrorDir := filepath.Join(cacheBase, hashWorkspaceSource(cloneSource)+".git")
	if _, err := os.Stat(mirrorDir); os.IsNotExist(err) {
		if err := runGit(ctx, "", creds, "clone", "--mirror", cloneSource, mirrorDir); err != nil {
			return "", fmt.Errorf("git clone --mirror failed: %w", err)
		}
		return mirrorDir, nil
	} else if err != nil {
		return "", fmt.Errorf("failed to stat git mirror cache: %w", err)
	}

	if err := runGit(ctx, mirrorDir, creds, "remote", "update", "--prune"); err != nil {
		return "", fmt.Errorf("git remote update failed for mirror cache: %w", err)
	}
	return mirrorDir, nil
}

func persistentCachePath(runtimePath string) string {
	runtimePath = filepath.Clean(runtimePath)
	if runtimePath == "/" || runtimePath == "." {
		return filepath.Join(agentDataDir, "cache")
	}
	return filepath.Join(agentDataDir, "cache", strings.TrimPrefix(runtimePath, "/"))
}

func hashWorkspaceSource(source string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(source)))
	return hex.EncodeToString(sum[:])
}

func currentBranchName(ctx context.Context, repoDir string) (string, error) {
	output, err := gitOutput(ctx, repoDir, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("failed to determine current branch: %w", err)
	}
	branchName := strings.TrimSpace(string(output))
	if branchName == "" {
		return "", fmt.Errorf("cloned workspace has no current branch")
	}
	return branchName, nil
}

func ensureBranchTracksOrigin(ctx context.Context, repoDir, branchName string) error {
	upstreamRef := "origin/" + branchName
	// Local workspace clones may not have a matching remote branch. That is not
	// fatal; it only means later git operations cannot rely on an upstream.
	if err := runGitQuiet(ctx, repoDir, "rev-parse", "--verify", upstreamRef); err != nil {
		return nil
	}
	if err := runGit(ctx, repoDir, nil, "branch", "--set-upstream-to", upstreamRef, branchName); err != nil {
		return fmt.Errorf("failed to set upstream %s for branch %s: %w", upstreamRef, branchName, err)
	}
	return nil
}

func runGit(ctx context.Context, dir string, creds []credentials.EnvVar, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitCommandEnv(creds)
	var output bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &output)
	cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(output.String())
		if detail != "" {
			return fmt.Errorf("%w: %s", err, trimGitErrorDetail(redactGitErrorDetail(detail)))
		}
		return err
	}
	return nil
}

func runGitQuiet(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitCommandEnv(nil)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitCommandEnv(nil)
	return cmd.Output()
}

func gitCommandEnv(creds []credentials.EnvVar) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	// Clone runs before request middleware exists, so seed git with the same
	// agent-visible credentials that will later populate the runtime manager.
	for _, cred := range creds {
		if !cred.AgentVisible || cred.EnvVar == "" {
			continue
		}
		env = append(env, cred.EnvVar+"="+cred.Value)
	}
	return env
}

func trimGitErrorDetail(detail string) string {
	const maxLen = 4096
	if len(detail) <= maxLen {
		return detail
	}
	return detail[:maxLen] + "...[truncated]"
}

func redactGitErrorDetail(detail string) string {
	detail = gitBasicAuthURLPattern.ReplaceAllString(detail, "${1}[redacted]@")
	return gitSensitiveQueryPattern.ReplaceAllString(detail, "${1}[redacted]")
}
