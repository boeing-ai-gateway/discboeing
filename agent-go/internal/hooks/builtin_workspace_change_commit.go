package hooks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/workspaceenv"
)

const builtinWorkspaceChangeCommitHookID = "discboeing-workspace-change-commit"

func builtinWorkspaceChangeCommitHook() Hook {
	return Hook{
		ID:        builtinWorkspaceChangeCommitHookID,
		Name:      "Workspace change commit",
		Type:      HookTypeFile,
		Engine:    HookEngineBuiltin,
		Pattern:   "**/*",
		RunAs:     "user",
		NotifyLLM: false,
	}
}

func runBuiltinHook(hook Hook, opts ExecuteOptions) HookResult {
	start := time.Now()
	output, err := createWorkspaceChangeCommit(opts.Cwd, opts.Env, opts.ChangedFiles, opts.SessionID)
	exitCode := 0
	if err != nil {
		exitCode = 1
		if output != "" {
			output += "\n"
		}
		output += err.Error()
	}
	result := HookResult{
		Success:    exitCode == 0,
		ExitCode:   exitCode,
		Output:     output,
		Hook:       hook,
		DurationMs: time.Since(start).Milliseconds(),
	}
	writeOutputFile(opts.OutputPath, output)
	return result
}

func createWorkspaceChangeCommit(workspaceRoot string, env map[string]string, changedFiles []string, sessionID string) (string, error) {
	if workspaceRoot == "" {
		workspaceRoot = "."
	}
	if _, err := workspaceChangeGitOutput(workspaceRoot, nil, nil, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "No git work tree found; skipping workspace change commit.", nil
	}

	base, err := workspaceChangeGitOutput(workspaceRoot, nil, nil, "rev-parse", "HEAD")
	if err != nil {
		return "No git HEAD found; skipping workspace change commit.", nil
	}
	baseTree, err := workspaceChangeGitOutput(workspaceRoot, nil, nil, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return "", fmt.Errorf("read HEAD tree: %w", err)
	}

	tmpIndex, err := os.CreateTemp("", "discboeing-workspace-change-index-*")
	if err != nil {
		return "", fmt.Errorf("create temporary git index: %w", err)
	}
	tmpIndexPath := tmpIndex.Name()
	_ = tmpIndex.Close()
	defer os.Remove(tmpIndexPath)

	gitEnv := map[string]string{"GIT_INDEX_FILE": tmpIndexPath}
	if _, err := workspaceChangeGitOutput(workspaceRoot, gitEnv, nil, "read-tree", "HEAD"); err != nil {
		return "", fmt.Errorf("read HEAD into temporary index: %w", err)
	}
	if _, err := workspaceChangeGitOutput(workspaceRoot, gitEnv, nil, "add", "-u"); err != nil {
		return "", fmt.Errorf("add tracked workspace changes to temporary index: %w", err)
	}
	snapshotTree, err := workspaceChangeGitOutput(workspaceRoot, gitEnv, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write workspace change tree: %w", err)
	}
	if snapshotTree == baseTree {
		return "No workspace changes to commit.", nil
	}

	rawID := fmt.Sprintf("%s-%d", time.Now().UTC().Format("20060102T150405.000000000Z"), os.Getpid())
	safeID := sanitizeWorkspaceChangeRefPart(rawID)
	if sessionID == "" {
		sessionID = env["DISCBOEING_SESSION_ID"]
	}
	if sessionID == "" {
		sessionID = "session"
	}
	safeSession := sanitizeWorkspaceChangeRefPart(sessionID)
	ref := fmt.Sprintf("refs/discboeing/workspace-change-commits/%s/%s", safeSession, safeID)

	commitEnv := map[string]string{
		"GIT_AUTHOR_NAME":     envWithDefault(env, "GIT_AUTHOR_NAME", "Discboeing Workspace Change"),
		"GIT_AUTHOR_EMAIL":    envWithDefault(env, "GIT_AUTHOR_EMAIL", "discboeing-workspace-change@localhost"),
		"GIT_COMMITTER_NAME":  envWithDefault(env, "GIT_COMMITTER_NAME", "Discboeing Workspace Change"),
		"GIT_COMMITTER_EMAIL": envWithDefault(env, "GIT_COMMITTER_EMAIL", "discboeing-workspace-change@localhost"),
	}
	changedFileList := strings.Join(changedFiles, " ")
	if changedFileList == "" {
		changedFileList = env["DISCBOEING_CHANGED_FILES"]
	}
	message := fmt.Sprintf("Discboeing workspace change commit %s\n\nSession: %s\nBase: %s\nChanged files: %s\n", safeID, sessionID, base, changedFileList)
	commit, err := workspaceChangeGitOutput(workspaceRoot, commitEnv, []byte(message), "commit-tree", snapshotTree, "-p", base, "-F", "-")
	if err != nil {
		return "", fmt.Errorf("create workspace change commit: %w", err)
	}
	if _, err := workspaceChangeGitOutput(workspaceRoot, nil, nil, "update-ref", ref, commit); err != nil {
		return "", fmt.Errorf("update workspace change commit ref: %w", err)
	}
	return fmt.Sprintf("Created workspace change commit %s at %s", commit, ref), nil
}

func workspaceChangeGitOutput(dir string, extraEnv map[string]string, stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	env := workspaceenv.MergeProcessSnapshot(extraEnv)
	cmd.Env = workspaceenv.List(env)
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		if trimmed == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, trimmed)
	}
	return trimmed, nil
}

func envWithDefault(env map[string]string, key, fallback string) string {
	if value := strings.TrimSpace(env[key]); value != "" {
		return value
	}
	return fallback
}

func sanitizeWorkspaceChangeRefPart(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}
