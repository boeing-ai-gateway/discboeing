package hooks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/workspaceenv"
)

const builtinShadowSnapshotHookID = "discobot-shadow-snapshot"

func builtinShadowSnapshotHook() Hook {
	return Hook{
		ID:        builtinShadowSnapshotHookID,
		Name:      "Shadow workspace snapshot",
		Type:      HookTypeFile,
		Engine:    HookEngineBuiltin,
		Pattern:   "**/*",
		RunAs:     "user",
		NotifyLLM: false,
	}
}

func runBuiltinHook(hook Hook, opts ExecuteOptions) HookResult {
	start := time.Now()
	output, err := createShadowSnapshot(opts.Cwd, opts.Env, opts.ChangedFiles, opts.SessionID)
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

func createShadowSnapshot(workspaceRoot string, env map[string]string, changedFiles []string, sessionID string) (string, error) {
	if workspaceRoot == "" {
		workspaceRoot = "."
	}
	if _, err := shadowGitOutput(workspaceRoot, nil, nil, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "No git work tree found; skipping shadow snapshot.", nil
	}

	base, err := shadowGitOutput(workspaceRoot, nil, nil, "rev-parse", "HEAD")
	if err != nil {
		return "No git HEAD found; skipping shadow snapshot.", nil
	}
	baseTree, err := shadowGitOutput(workspaceRoot, nil, nil, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return "", fmt.Errorf("read HEAD tree: %w", err)
	}

	tmpIndex, err := os.CreateTemp("", "discobot-shadow-index-*")
	if err != nil {
		return "", fmt.Errorf("create temporary git index: %w", err)
	}
	tmpIndexPath := tmpIndex.Name()
	_ = tmpIndex.Close()
	defer os.Remove(tmpIndexPath)

	gitEnv := map[string]string{"GIT_INDEX_FILE": tmpIndexPath}
	if _, err := shadowGitOutput(workspaceRoot, gitEnv, nil, "read-tree", "HEAD"); err != nil {
		return "", fmt.Errorf("read HEAD into temporary index: %w", err)
	}
	if _, err := shadowGitOutput(workspaceRoot, gitEnv, nil, "add", "-A"); err != nil {
		return "", fmt.Errorf("add workspace to temporary index: %w", err)
	}
	snapshotTree, err := shadowGitOutput(workspaceRoot, gitEnv, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write snapshot tree: %w", err)
	}
	if snapshotTree == baseTree {
		return "No workspace changes to snapshot.", nil
	}

	rawID := fmt.Sprintf("%s-%d", time.Now().UTC().Format("20060102T150405.000000000Z"), os.Getpid())
	safeID := sanitizeSnapshotRefPart(rawID)
	if sessionID == "" {
		sessionID = env["DISCOBOT_SESSION_ID"]
	}
	if sessionID == "" {
		sessionID = "session"
	}
	safeSession := sanitizeSnapshotRefPart(sessionID)
	ref := fmt.Sprintf("refs/discobot/snapshots/%s/%s", safeSession, safeID)

	commitEnv := map[string]string{
		"GIT_AUTHOR_NAME":     envWithDefault(env, "GIT_AUTHOR_NAME", "Discobot Snapshot"),
		"GIT_AUTHOR_EMAIL":    envWithDefault(env, "GIT_AUTHOR_EMAIL", "discobot-snapshot@localhost"),
		"GIT_COMMITTER_NAME":  envWithDefault(env, "GIT_COMMITTER_NAME", "Discobot Snapshot"),
		"GIT_COMMITTER_EMAIL": envWithDefault(env, "GIT_COMMITTER_EMAIL", "discobot-snapshot@localhost"),
	}
	changedFileList := strings.Join(changedFiles, " ")
	if changedFileList == "" {
		changedFileList = env["DISCOBOT_CHANGED_FILES"]
	}
	message := fmt.Sprintf("Discobot shadow snapshot %s\n\nSession: %s\nBase: %s\nChanged files: %s\n", safeID, sessionID, base, changedFileList)
	commit, err := shadowGitOutput(workspaceRoot, commitEnv, []byte(message), "commit-tree", snapshotTree, "-p", base, "-F", "-")
	if err != nil {
		return "", fmt.Errorf("create snapshot commit: %w", err)
	}
	if _, err := shadowGitOutput(workspaceRoot, nil, nil, "update-ref", ref, commit); err != nil {
		return "", fmt.Errorf("update snapshot ref: %w", err)
	}
	return fmt.Sprintf("Created shadow snapshot %s at %s", commit, ref), nil
}

func shadowGitOutput(dir string, extraEnv map[string]string, stdin []byte, args ...string) (string, error) {
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
		return "", fmt.Errorf("%v: %s", err, trimmed)
	}
	return trimmed, nil
}

func envWithDefault(env map[string]string, key, fallback string) string {
	if value := strings.TrimSpace(env[key]); value != "" {
		return value
	}
	return fallback
}

func sanitizeSnapshotRefPart(value string) string {
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
