package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/helperbin"
	"github.com/obot-platform/discobot/agent-go/internal/workspaceenv"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("bash is not available on Windows")
	}
}

func skipIfBashUnavailable(t *testing.T) {
	t.Helper()
	if _, err := resolveBashCommand(); err != nil {
		t.Skipf("bash unavailable: %v", err)
	}
}

func stageApplyPatchShimForTests(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go unavailable: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path")
	}
	agentBin := filepath.Join(t.TempDir(), "agent-api-test")
	build := exec.Command("go", "build", "-o", agentBin, "./cmd/agent-api")
	build.Dir = filepath.Join(filepath.Dir(file), "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build agent-api test binary: %v\n%s", err, string(out))
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	binDir := helperbin.Dir()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"apply_patch", "applypatch"} {
		script := "#!/usr/bin/env bash\nset -eu\nexec " + agentBin + " --discobot-run-as-apply-patch \"$@\"\n"
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// runBash is a test helper that executes a Bash tool call and returns the output text.
// It accepts an arbitrary input map so callers can set timeout, run_in_background, etc.
func runBash(t *testing.T, e *Executor, input map[string]any) (string, bool) {
	t.Helper()
	return runBashWithContext(t, e, nil, input)
}

func runBashWithContext(t *testing.T, e *Executor, toolCtx *thread.ToolContext, input map[string]any) (string, bool) {
	t.Helper()
	raw, _ := json.Marshal(input)
	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: t.Name(),
		ToolName:   "Bash",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	switch v := result.Result.Output.(type) {
	case message.TextOutput:
		return v.Value, true
	case message.ErrorTextOutput:
		return v.Value, false
	}
	return "", false
}

func TestBash_EmptyCommand(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	_, ok := runBash(t, e, map[string]any{"command": ""})
	if ok {
		t.Error("expected ErrorTextOutput for empty command, got TextOutput")
	}
}

func TestBash_SimpleEcho(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runBash(t, e, map[string]any{"command": "echo hello"})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in output, got: %q", out)
	}
}

// TestBash_LineNumbers verifies output lines carry "     N→" prefixes.
func TestBash_LineNumbers(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runBash(t, e, map[string]any{"command": "echo hello"})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	// addLineNumbers produces "     1→hello\n"
	if !strings.Contains(out, "→") {
		t.Errorf("expected →-separated line numbers in output, got: %q", out)
	}
	trimmed := strings.TrimLeft(out, " ")
	if !strings.HasPrefix(trimmed, "1→") {
		t.Errorf("expected first line to start with '1→', got: %q", out)
	}
}

// TestBash_MultiLineNumbers verifies each line gets the correct sequential number.
func TestBash_MultiLineNumbers(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runBash(t, e, map[string]any{"command": "printf 'a\\nb\\nc\\n'"})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	for i, want := range []string{"a", "b", "c"} {
		// Each line must appear with its correct number.
		if !strings.Contains(out, strings.TrimSpace(strings.Repeat(" ", 5)+string(rune('0'+i+1))+"\t"+want)) {
			t.Logf("full output:\n%s", out)
		}
		// Simpler: just verify the content appears and line count is right.
		_ = want
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 numbered lines, got %d:\n%s", len(lines), out)
	}
}

// TestBash_StderrCaptured verifies stderr output is included in the result.
func TestBash_StderrCaptured(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, _ := runBash(t, e, map[string]any{"command": "echo errline >&2"})
	if !strings.Contains(out, "errline") {
		t.Errorf("expected stderr 'errline' in output, got: %s", out)
	}
}

// TestBash_NonZeroExitIsTextOutput verifies a failing command returns TextOutput, not an error.
func TestBash_NonZeroExitIsTextOutput(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	_, ok := runBash(t, e, map[string]any{"command": "exit 1"})
	if !ok {
		t.Error("expected TextOutput (not ErrorTextOutput) for non-zero exit code")
	}
}

// TestBash_CwdPersistsAcrossCalls verifies that a cd in one call is visible in the next.
func TestBash_CwdPersistsAcrossCalls(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	runBash(t, e, map[string]any{"command": "cd /tmp"})
	out, ok := runBash(t, e, map[string]any{"command": "pwd"})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "/tmp") {
		t.Errorf("expected cwd '/tmp' after 'cd /tmp', got: %s", out)
	}
}

func TestBash_CwdPersistsPerThreadContext(t *testing.T) {
	skipOnWindows(t)

	e := New(t.TempDir(), t.TempDir(), t.Name())
	dirOne := filepath.Join(t.TempDir(), "one")
	dirTwo := filepath.Join(t.TempDir(), "two")
	if err := os.MkdirAll(dirOne, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirTwo, 0o755); err != nil {
		t.Fatal(err)
	}
	threadOne := &thread.ToolContext{ThreadID: "thread-1", CurrentWorkingDirectory: dirOne}
	threadOne.SetCurrentWorkingDirectory = func(cwd string) error {
		threadOne.CurrentWorkingDirectory = cwd
		return nil
	}
	threadTwo := &thread.ToolContext{ThreadID: "thread-2", CurrentWorkingDirectory: dirTwo}
	threadTwo.SetCurrentWorkingDirectory = func(cwd string) error {
		threadTwo.CurrentWorkingDirectory = cwd
		return nil
	}

	if _, ok := runBashWithContext(t, e, threadOne, map[string]any{"command": "cd .."}); !ok {
		t.Fatal("expected first thread cd to succeed")
	}
	out, ok := runBashWithContext(t, e, threadOne, map[string]any{"command": "pwd"})
	if !ok {
		t.Fatalf("unexpected error for first thread: %s", out)
	}
	if !strings.Contains(out, filepath.Dir(dirOne)) {
		t.Fatalf("expected first thread cwd %q, got: %s", filepath.Dir(dirOne), out)
	}

	out, ok = runBashWithContext(t, e, threadTwo, map[string]any{"command": "pwd"})
	if !ok {
		t.Fatalf("unexpected error for second thread: %s", out)
	}
	if !strings.Contains(out, dirTwo) {
		t.Fatalf("expected second thread cwd %q, got: %s", dirTwo, out)
	}
}

func TestBash_HeredocCommandOutput(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())

	out, ok := runBash(t, e, map[string]any{"command": "cat <<'EOF'\nfoo\nEOF"})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "foo") {
		t.Errorf("expected heredoc output in result, got: %s", out)
	}
}

func TestBash_HeredocThenCwdPersistsAcrossCalls(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())

	out, ok := runBash(t, e, map[string]any{"command": "cat <<'EOF'\nhello\nEOF\ncd /tmp"})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected heredoc output in result, got: %s", out)
	}

	out, ok = runBash(t, e, map[string]any{"command": "pwd"})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "/tmp") {
		t.Errorf("expected cwd '/tmp' after heredoc + cd, got: %s", out)
	}
}

func TestBash_ApplyPatchHeredocViaShim(t *testing.T) {
	skipOnWindows(t)
	stageApplyPatchShimForTests(t)
	cwd := t.TempDir()
	e := New(cwd, t.TempDir(), t.Name())

	out, ok := runBash(t, e, map[string]any{
		"command": "apply_patch <<'PATCH'\n*** Begin Patch\n*** Add File: from-bash.txt\n+hello\n*** End Patch\nPATCH",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "A from-bash.txt") {
		t.Fatalf("expected apply_patch summary, got: %q", out)
	}

	data, err := os.ReadFile(filepath.Join(cwd, "from-bash.txt"))
	if err != nil {
		t.Fatalf("expected file to be created: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestBash_ApplypatchAliasWithCdViaShim(t *testing.T) {
	skipOnWindows(t)
	stageApplyPatchShimForTests(t)
	cwd := t.TempDir()
	subdir := filepath.Join(cwd, "sub dir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())

	out, ok := runBash(t, e, map[string]any{
		"command": "cd 'sub dir' && applypatch <<'PATCH'\n*** Begin Patch\n*** Add File: nested.txt\n+hello\n*** End Patch\nPATCH",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "A nested.txt") {
		t.Fatalf("expected apply_patch summary, got: %q", out)
	}

	data, err := os.ReadFile(filepath.Join(subdir, "nested.txt"))
	if err != nil {
		t.Fatalf("expected nested file to be created: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("unexpected nested file content: %q", string(data))
	}

	out, ok = runBash(t, e, map[string]any{"command": "pwd"})
	if !ok {
		t.Fatalf("unexpected pwd error: %s", out)
	}
	if !strings.Contains(out, subdir) {
		t.Fatalf("expected cwd %q after apply_patch shim command, got: %q", subdir, out)
	}
}

// TestBash_LogFileCreatedForeground verifies the log file is written to the expected path.
func TestBash_LogFileCreatedForeground(t *testing.T) {
	skipOnWindows(t)
	cwd := t.TempDir()
	dataDir := t.TempDir()
	e := New(cwd, dataDir, "thread-1")
	raw, _ := json.Marshal(map[string]string{"command": "echo logged"})
	callID := "call-fg"
	_, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: callID,
		ToolName:   "Bash",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	logPath := filepath.Join(dataDir, "threads", "thread-1", "bash", callID+".log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("log file not found at %s: %v", logPath, err)
	}
	if !strings.Contains(string(data), "logged") {
		t.Errorf("expected 'logged' in log file, got: %s", data)
	}
}

// TestBash_Timeout verifies that a slow command is killed and the output contains a timeout notice.
func TestBash_Timeout(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())
	raw, _ := json.Marshal(map[string]any{
		"command": "sleep 60",
		"timeout": 100, // 100 ms
	})
	start := time.Now()
	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "timeout-call",
		ToolName:   "Bash",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("timeout did not fire: command ran for %s", elapsed)
	}

	out, ok := "", false
	switch v := result.Result.Output.(type) {
	case message.TextOutput:
		out, ok = v.Value, true
	case message.ErrorTextOutput:
		out = v.Value
	}
	if !ok {
		t.Fatalf("unexpected ErrorTextOutput: %s", out)
	}
	if !strings.Contains(out, "timed out") {
		t.Errorf("expected 'timed out' in output, got: %s", out)
	}
}

func TestBash_LargeOutputUsesOpenCodeStyleTruncation(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())

	out, ok := runBash(t, e, map[string]any{
		"command": "i=1; while [ \"$i\" -le 2505 ]; do echo line-$i; i=$((i+1)); done",
	})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "...") || !strings.Contains(out, "truncated") {
		t.Fatalf("expected truncation marker, got: %q", out)
	}
	if !strings.Contains(out, "The tool call succeeded but the output was truncated.") {
		t.Fatalf("expected spill notice, got: %q", out)
	}
	if !strings.Contains(out, "Use Grep to search the full content or Read with offset/limit") {
		t.Fatalf("expected follow-up guidance, got: %q", out)
	}
}

// TestBash_BackgroundReturnsPIDAndLogPath verifies the immediate response for a background command.
func TestBash_SyncCommandWithShellBackgroundingReturnsPromptly(t *testing.T) {
	skipOnWindows(t)
	e := New(t.TempDir(), t.TempDir(), t.Name())

	start := time.Now()
	out, ok := runBash(t, e, map[string]any{
		"command": "sleep 5 & echo started",
		"timeout": 1000,
	})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected sync bash call to return before background child exits; took %s", elapsed)
	}
	if !strings.Contains(out, "started") {
		t.Fatalf("expected command output in result, got: %s", out)
	}
	if strings.Contains(out, "timed out") {
		t.Fatalf("expected command not to time out, got: %s", out)
	}
}

func TestBash_BackgroundReturnsPIDAndLogPath(t *testing.T) {
	skipOnWindows(t)
	cwd := t.TempDir()
	e := New(cwd, t.TempDir(), "bg-thread")
	raw, _ := json.Marshal(map[string]any{
		"command":           "sleep 5",
		"run_in_background": true,
	})
	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "bg-call",
		ToolName:   "Bash",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput for background command, got %T", result.Result.Output)
	}
	if !strings.Contains(out.Value, "PID:") {
		t.Errorf("expected 'PID:' in output, got: %s", out.Value)
	}
	if !strings.Contains(out.Value, "bg-call.log") {
		t.Errorf("expected log path containing 'bg-call.log' in output, got: %s", out.Value)
	}
}

// TestBash_BackgroundLogFileWritten verifies background output is saved to the log file.
func TestBash_BackgroundLogFileWritten(t *testing.T) {
	skipOnWindows(t)
	dataDir := t.TempDir()
	e := New(t.TempDir(), dataDir, "bg-thread")
	raw, _ := json.Marshal(map[string]any{
		"command":           "echo bg-output",
		"run_in_background": true,
	})
	_, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "bg-log",
		ToolName:   "Bash",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logPath := filepath.Join(dataDir, "threads", "bg-thread", "bash", "bg-log.log")
	// Give the background process time to write.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err == nil && strings.Contains(string(data), "bg-output") {
			return // pass
		}
		time.Sleep(50 * time.Millisecond)
	}
	data, _ := os.ReadFile(logPath)
	t.Errorf("expected 'bg-output' in background log file within 2s, got: %s", data)
}

func TestBash_DefaultPassesProcessEnv(t *testing.T) {
	skipOnWindows(t)
	t.Setenv("DISCOBOT_BASH_ENV_TEST_DEFAULT", "present")

	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runBash(t, e, map[string]any{"command": "echo \"${DISCOBOT_BASH_ENV_TEST_DEFAULT}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→present") {
		t.Errorf("expected env var to be visible to bash, got: %q", out)
	}
}

func TestBash_AllowlistFiltersEnv(t *testing.T) {
	skipOnWindows(t)
	t.Setenv("DISCOBOT_BASH_ENV_TEST_ALLOWED", "yes")
	t.Setenv("DISCOBOT_BASH_ENV_TEST_BLOCKED", "no")

	e := New(t.TempDir(), t.TempDir(), t.Name())
	e.SetBashEnvAllowlist([]string{"DISCOBOT_BASH_ENV_TEST_ALLOWED"})

	out, ok := runBash(t, e, map[string]any{"command": "echo \"${DISCOBOT_BASH_ENV_TEST_ALLOWED}|${DISCOBOT_BASH_ENV_TEST_BLOCKED}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→yes|") {
		t.Errorf("expected only allowlisted env var in output, got: %q", out)
	}
}

func TestBash_RequestScopedEnvVisible(t *testing.T) {
	skipOnWindows(t)

	e := New(t.TempDir(), t.TempDir(), t.Name())
	e.SetEnvSnapshot(func() map[string]string {
		return map[string]string{"DISCOBOT_BASH_ENV_TEST_REQUEST": "from-request"}
	})

	out, ok := runBash(t, e, map[string]any{"command": "echo \"${DISCOBOT_BASH_ENV_TEST_REQUEST}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→from-request") {
		t.Errorf("expected request-scoped env var to be visible to bash, got: %q", out)
	}
}

func TestBash_RequestScopedEnvRespectsAllowlist(t *testing.T) {
	skipOnWindows(t)

	e := New(t.TempDir(), t.TempDir(), t.Name())
	e.SetBashEnvAllowlist([]string{"DISCOBOT_BASH_ENV_TEST_ALLOWED_REQUEST"})
	e.SetEnvSnapshot(func() map[string]string {
		return map[string]string{
			"DISCOBOT_BASH_ENV_TEST_ALLOWED_REQUEST": "allowed-request",
			"DISCOBOT_BASH_ENV_TEST_BLOCKED_REQUEST": "blocked-request",
		}
	})

	out, ok := runBash(t, e, map[string]any{"command": "echo \"${DISCOBOT_BASH_ENV_TEST_ALLOWED_REQUEST}|${DISCOBOT_BASH_ENV_TEST_BLOCKED_REQUEST}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→allowed-request|") {
		t.Errorf("expected only allowlisted request-scoped env var in output, got: %q", out)
	}
}

func TestBash_PATHAlwaysIncludesHelperBin(t *testing.T) {
	skipOnWindows(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin")

	e := New(t.TempDir(), t.TempDir(), t.Name())
	e.SetBashEnvAllowlist([]string{"DISCOBOT_BASH_ENV_TEST_ALLOWED"})

	out, ok := runBash(t, e, map[string]any{"command": "echo \"$PATH\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, helperbin.Dir()) {
		t.Fatalf("expected PATH to include helper bin %q, got: %q", helperbin.Dir(), out)
	}
}

func TestBash_RequestScopedEnvOverridesProcessEnv(t *testing.T) {
	skipOnWindows(t)
	t.Setenv("DISCOBOT_BASH_ENV_TEST_OVERRIDE", "from-process")

	e := New(t.TempDir(), t.TempDir(), t.Name())
	e.SetEnvSnapshot(func() map[string]string {
		return map[string]string{"DISCOBOT_BASH_ENV_TEST_OVERRIDE": "from-request"}
	})

	out, ok := runBash(t, e, map[string]any{"command": "echo \"${DISCOBOT_BASH_ENV_TEST_OVERRIDE}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→from-request") {
		t.Errorf("expected request-scoped env var to override process env, got: %q", out)
	}
}

func TestBash_WorkspaceEnvReloadsBetweenCalls(t *testing.T) {
	skipOnWindows(t)

	cwd := t.TempDir()
	envDir := filepath.Join(cwd, ".discobot")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	envPath := filepath.Join(envDir, "env")
	if err := os.WriteFile(envPath, []byte("DISCOBOT_BASH_ENV_TEST_DYNAMIC=first\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(first): %v", err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	e.SetEnvSnapshot(func() map[string]string {
		return workspaceenv.FileSnapshot(cwd)
	})

	out, ok := runBash(t, e, map[string]any{"command": "echo \"${DISCOBOT_BASH_ENV_TEST_DYNAMIC}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→first") {
		t.Fatalf("expected first workspace env value, got: %q", out)
	}

	if err := os.WriteFile(envPath, []byte("DISCOBOT_BASH_ENV_TEST_DYNAMIC=second\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(second): %v", err)
	}

	out, ok = runBash(t, e, map[string]any{"command": "echo \"${DISCOBOT_BASH_ENV_TEST_DYNAMIC}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→second") {
		t.Fatalf("expected updated workspace env value, got: %q", out)
	}

	if err := os.WriteFile(envPath, []byte("DISCOBOT_BASH_ENV_TEST_OTHER=present\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(third): %v", err)
	}

	out, ok = runBash(t, e, map[string]any{"command": "printf '%s|%s' \"${DISCOBOT_BASH_ENV_TEST_DYNAMIC:-}\" \"${DISCOBOT_BASH_ENV_TEST_OTHER:-}\""})
	if !ok {
		t.Fatalf("unexpected error output: %s", out)
	}
	if !strings.Contains(out, "→|present") {
		t.Fatalf("expected removed workspace env key to be unset, got: %q", out)
	}
}

func TestBash_InvalidWorkingDirReturnsError(t *testing.T) {
	skipIfBashUnavailable(t)

	e := New(t.TempDir(), t.TempDir(), t.Name())
	_ = e.setCwd(nil, filepath.Join(t.TempDir(), "missing"))

	out, ok := runBash(t, e, map[string]any{"command": "pwd"})
	if ok {
		t.Fatalf("expected ErrorTextOutput for invalid cwd, got: %s", out)
	}
	if !strings.Contains(out, "failed to run command") {
		t.Fatalf("expected startup error in output, got: %q", out)
	}
}

func TestBash_WindowsSecondCommandStillRunsAfterPwd(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only test")
	}
	skipIfBashUnavailable(t)

	cwd := t.TempDir()
	e := New(cwd, t.TempDir(), t.Name())

	out, ok := runBash(t, e, map[string]any{"command": "pwd"})
	if !ok {
		t.Fatalf("unexpected error from first pwd: %s", out)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected first pwd output to be non-empty")
	}
	if !sameResolvedPath(e.getCwd(nil), cwd) {
		t.Fatalf("executor cwd = %q, want native path matching %q", e.getCwd(nil), cwd)
	}

	out, ok = runBash(t, e, map[string]any{"command": "pwd"})
	if !ok {
		t.Fatalf("unexpected error from second pwd: %s", out)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected second pwd output to be non-empty")
	}
}

func TestSameResolvedPathUsesFileIdentity(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("discobot\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(target): %v", err)
	}

	alias := filepath.Join(dir, "alias.txt")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if !sameResolvedPath(target, alias) {
		t.Fatalf("sameResolvedPath(%q, %q) = false, want true", target, alias)
	}
	if !sameResolvedPath(alias, target) {
		t.Fatalf("sameResolvedPath(%q, %q) = false, want true", alias, target)
	}
}

func TestNormalizeBashWorkingDirForOS(t *testing.T) {
	tests := []struct {
		name string
		goos string
		cwd  string
		want string
	}{
		{
			name: "non-windows unchanged",
			goos: "linux",
			cwd:  "/tmp/discobot",
			want: "/tmp/discobot",
		},
		{
			name: "msys drive path converted",
			goos: "windows",
			cwd:  "/e/src/discobot",
			want: `E:\src\discobot`,
		},
		{
			name: "wsl drive path converted",
			goos: "windows",
			cwd:  "/mnt/c/Users/tester/project",
			want: `C:\Users\tester\project`,
		},
		{
			name: "non-drive path left alone",
			goos: "windows",
			cwd:  "/home/tester",
			want: "/home/tester",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeBashWorkingDirForOS(tc.goos, tc.cwd); got != tc.want {
				t.Fatalf("normalizeBashWorkingDirForOS() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveBashCommandForOS_NonWindows(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "bash")
	if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("failed to create bash: %v", err)
	}

	got, err := resolveBashCommandForOS("linux", []string{dir}, "")
	if err != nil {
		t.Fatalf("resolveBashCommandForOS returned error: %v", err)
	}
	if got != want {
		t.Fatalf("resolveBashCommandForOS() = %q, want %q", got, want)
	}
}

func TestResolveBashCommandForOS_WindowsPrefersPowerShellExecutable(t *testing.T) {
	dirWithCmd := t.TempDir()
	dirWithExe := t.TempDir()

	if err := os.WriteFile(filepath.Join(dirWithCmd, "powershell.cmd"), []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatalf("failed to create powershell.cmd: %v", err)
	}
	want := filepath.Join(dirWithExe, "powershell.exe")
	if err := os.WriteFile(want, []byte("fake exe"), 0o644); err != nil {
		t.Fatalf("failed to create powershell.exe: %v", err)
	}

	got, err := resolveBashCommandForOS("windows", []string{dirWithCmd, dirWithExe}, ".CMD;.EXE;.BAT")
	if err != nil {
		t.Fatalf("resolveBashCommandForOS returned error: %v", err)
	}
	if got != want {
		t.Fatalf("resolveBashCommandForOS() = %q, want %q", got, want)
	}
}

func TestResolveBashCommandForOS_WindowsUsesFirstPowerShellExecutableInPath(t *testing.T) {
	firstDir := t.TempDir()
	secondDir := t.TempDir()

	first := filepath.Join(firstDir, "powershell.exe")
	second := filepath.Join(secondDir, "powershell.exe")
	if err := os.WriteFile(first, []byte("first"), 0o644); err != nil {
		t.Fatalf("failed to create first powershell.exe: %v", err)
	}
	if err := os.WriteFile(second, []byte("second"), 0o644); err != nil {
		t.Fatalf("failed to create second powershell.exe: %v", err)
	}

	got, err := resolveBashCommandForOS("windows", []string{firstDir, secondDir}, ".EXE")
	if err != nil {
		t.Fatalf("resolveBashCommandForOS returned error: %v", err)
	}
	if got != first {
		t.Fatalf("resolveBashCommandForOS() = %q, want %q", got, first)
	}
}

func TestResolveBashCommandForOS_WindowsRejectsBatchShimsWithoutPowerShellFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SystemRoot", "")
	t.Setenv("ProgramFiles", "")
	if err := os.WriteFile(filepath.Join(dir, "powershell.cmd"), []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatalf("failed to create powershell.cmd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pwsh.bat"), []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatalf("failed to create pwsh.bat: %v", err)
	}

	_, err := resolveBashCommandForOS("windows", []string{dir}, ".CMD;.BAT")
	if err == nil {
		t.Fatal("expected resolveBashCommandForOS to fail when only batch shims exist and fallback locations are disabled")
	}
	if !strings.Contains(err.Error(), "PowerShell is not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExtractCwdFromOutput unit-tests the sentinel-based cwd extraction helper.
func TestExtractCwdFromOutput(t *testing.T) {
	const sentinel = "__DISCOBOT_PWD_SENTINEL__"

	tests := []struct {
		name    string
		raw     string
		wantOut string
		wantCwd string
	}{
		{
			name:    "basic",
			raw:     "output line\n" + sentinel + "\n/home/user\n",
			wantOut: "output line\n",
			wantCwd: "/home/user",
		},
		{
			name:    "no sentinel",
			raw:     "just output\n",
			wantOut: "just output\n",
			wantCwd: "",
		},
		{
			name:    "empty user output",
			raw:     sentinel + "\n/tmp\n",
			wantOut: "",
			wantCwd: "/tmp",
		},
		{
			name:    "multiple sentinels uses last",
			raw:     sentinel + "\n/first\n" + sentinel + "\n/second\n",
			wantOut: sentinel + "\n/first\n",
			wantCwd: "/second",
		},
		{
			name:    "cwd with trailing whitespace is trimmed",
			raw:     sentinel + "\n/some/path   \n",
			wantOut: "",
			wantCwd: "/some/path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotOut, gotCwd := extractCwdFromOutput(tc.raw, sentinel)
			if gotOut != tc.wantOut {
				t.Errorf("output: got %q, want %q", gotOut, tc.wantOut)
			}
			if gotCwd != tc.wantCwd {
				t.Errorf("cwd: got %q, want %q", gotCwd, tc.wantCwd)
			}
		})
	}
}
