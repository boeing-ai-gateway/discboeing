package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type bashInput struct {
	Command         string                 `json:"command"`
	Description     string                 `json:"description"`
	Timeout         int                    `json:"timeout"` // milliseconds; 0 = default 120s
	RunInBackground bool                   `json:"run_in_background"`
	CredentialUses  []CredentialUseBinding `json:"credentialUses,omitempty"`
}

func (e *Executor) executeBash(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input bashInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.Command == "" {
		return errResult(call, "command is required"), nil
	}
	currentProviderID := ""
	if toolCtx != nil {
		currentProviderID = toolCtx.ProviderID
	}
	if err := e.authorizeCredentialUses(ctx, currentProviderID, call.ToolCallID, input.Command, input.Description, input.CredentialUses); err != nil {
		return errResult(call, err.Error()), nil
	}

	timeout := 120 * time.Second
	if input.Timeout > 0 {
		ms := min(input.Timeout, 600_000)
		timeout = time.Duration(ms) * time.Millisecond
	}

	if input.RunInBackground {
		return e.startBashBackground(toolCtx, call, input.Command)
	}
	return e.runBashSync(ctx, toolCtx, call, input.Command, timeout)
}

// bashLogPath returns the path for the log file for a bash call.
// All bash output (foreground and background) is persisted here so the LLM
// can reference or tail the file later.
//
// Path: {threadsDir}/{threadID}/bash/{toolCallID}.log
func (e *Executor) bashLogPath(toolCtx *thread.ToolContext, toolCallID string) string {
	return filepath.Join(e.threadDataDir(toolCtx), "bash", toolCallID+".log")
}

// runBashSync runs a bash command synchronously, returns the combined output,
// and saves it to a log file in {threadsDir}/{threadID}/bash/.
func (e *Executor) runBashSync(ctx context.Context, toolCtx *thread.ToolContext, call message.ToolCallPart, command string, timeout time.Duration) (thread.ToolExecuteResult, error) {
	cwd := e.getCwd(toolCtx)
	logPath := e.bashLogPath(toolCtx, call.ToolCallID)
	cwdPath := logPath + ".cwd"
	shellPath, err := resolveBashCommand()
	if err != nil {
		return errResult(call, err.Error()), nil
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return errResult(call, fmt.Sprintf("failed to create log directory: %v", err)), nil
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return errResult(call, fmt.Sprintf("failed to create log file: %v", err)), nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	wrapped := wrapShellCommandForOS(runtime.GOOS, command)
	cmd := exec.CommandContext(cmdCtx, shellPath, shellCommandArgsForOS(runtime.GOOS, wrapped)...)
	cmd.Dir = cwd
	cmd.Env = append(e.bashEnv(), "DISCOBOT_BASH_CWD_PATH="+cwdPath)
	// Put bash in its own process group so that killing it also kills any
	// child processes it spawned (e.g. sleep, subshells).
	setSysProcAttr(cmd)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Kill the entire process group (negative PID = pgid).
		return killProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
	}

	// Write stdout/stderr directly to a real file instead of a pipe-backed
	// writer. That way, a shell-backgrounded descendant inheriting stdout or
	// stderr cannot keep cmd.Run waiting for pipe EOF after the shell exits.
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	runErr := cmd.Run()

	if cmdCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
		fmt.Fprintf(logFile, "[Command timed out after %s and was killed]\n", timeout)
	}
	if closeErr := logFile.Close(); closeErr != nil {
		return errResult(call, fmt.Sprintf("failed to close log file: %v", closeErr)), nil
	}

	newCwdBytes, err := os.ReadFile(cwdPath)
	newCwd := ""
	if err == nil {
		newCwd = normalizeBashWorkingDir(strings.TrimSpace(string(newCwdBytes)))
	}

	outputBytes, err := os.ReadFile(logPath)
	if err != nil {
		return errResult(call, fmt.Sprintf("failed to read log file: %v", err)), nil
	}
	output := string(outputBytes)

	if newCwd != "" && newCwd != cwd {
		if err := e.setCwd(toolCtx, newCwd); err != nil {
			return errResult(call, fmt.Sprintf("failed to persist working directory: %v", err)), nil
		}
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			msg := fmt.Sprintf("failed to run command: %v", runErr)
			if trimmed := strings.TrimSpace(output); trimmed != "" {
				msg = trimmed + "\n" + msg
			}
			return errResult(call, msg), nil
		}
	}

	return textResult(call, addLineNumbers(output, 1)), nil
}

// startBashBackground launches a bash command in the background. It returns
// immediately with the process PID and log path so the LLM can tail or read
// the output at any time. Output is streamed directly to the log file.
func (e *Executor) startBashBackground(toolCtx *thread.ToolContext, call message.ToolCallPart, command string) (thread.ToolExecuteResult, error) {
	cwd := e.getCwd(toolCtx)
	logPath := e.bashLogPath(toolCtx, call.ToolCallID)
	shellPath, err := resolveBashCommand()
	if err != nil {
		return errResult(call, err.Error()), nil
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return errResult(call, fmt.Sprintf("failed to create log directory: %v", err)), nil
	}

	logFile, err := os.Create(logPath)
	if err != nil {
		return errResult(call, fmt.Sprintf("failed to create log file: %v", err)), nil
	}

	cmd := exec.Command(shellPath, shellCommandArgsForOS(runtime.GOOS, command)...) //nolint:gosec
	cmd.Dir = cwd
	cmd.Env = e.bashEnv()
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return errResult(call, fmt.Sprintf("failed to start background command: %v", err)), nil
	}

	pid := cmd.Process.Pid

	// Let the process run independently; close the log file when it exits.
	go func() {
		_ = cmd.Wait()
		logFile.Close()
	}()

	followCommand, stopCommand := backgroundShellHintsForOS(runtime.GOOS, logPath, pid)
	return textResult(call, fmt.Sprintf(
		"Background process started.\nPID: %d\nOutput: %s\n\nUse `%s` to follow the output, or `%s` to stop it.",
		pid, logPath, followCommand, stopCommand,
	)), nil
}

func resolveBashCommand() (string, error) {
	return resolveBashCommandForOS(runtime.GOOS, filepath.SplitList(os.Getenv("PATH")), os.Getenv("PATHEXT"))
}

func resolveBashCommandForOS(goos string, pathDirs []string, pathExt string) (string, error) {
	if goos != "windows" {
		for _, dir := range pathDirs {
			dir = strings.TrimSpace(strings.Trim(dir, `"`))
			if dir == "" {
				continue
			}
			candidate := filepath.Join(dir, "bash")
			if isExecutableFile(candidate) {
				return candidate, nil
			}
		}
		for _, candidate := range []string{"/bin/bash", "/usr/bin/bash", "/usr/local/bin/bash", "/opt/homebrew/bin/bash"} {
			if isExecutableFile(candidate) {
				return candidate, nil
			}
		}
		return "", fmt.Errorf("bash is not available: no bash executable was found")
	}

	for _, base := range []string{"powershell", "powershell.exe", "pwsh", "pwsh.exe"} {
		for _, dir := range pathDirs {
			dir = strings.TrimSpace(strings.Trim(dir, `"`))
			if dir == "" {
				continue
			}
			for _, ext := range windowsExecutableExtensions(pathExt) {
				candidate := filepath.Join(dir, strings.TrimSuffix(base, ext)+ext)
				if isExecutableFile(candidate) {
					return candidate, nil
				}
			}
			candidate := filepath.Join(dir, base)
			if isExecutableFile(candidate) {
				return candidate, nil
			}
		}
	}

	for _, candidate := range []string{
		filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "PowerShell", "7", "pwsh.exe"),
	} {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("PowerShell is not available: no powershell.exe or pwsh.exe executable was found")
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func windowsExecutableExtensions(pathExt string) []string {
	exts := []string{".exe"}
	seen := map[string]struct{}{
		".exe": {},
	}

	for raw := range strings.SplitSeq(pathExt, ";") {
		ext := strings.TrimSpace(strings.ToLower(raw))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if ext == ".cmd" || ext == ".bat" {
			continue
		}
		if _, ok := seen[ext]; ok {
			continue
		}
		seen[ext] = struct{}{}
		exts = append(exts, ext)
	}

	return exts
}

func wrapShellCommandForOS(goos, command string) string {
	if goos != "windows" {
		return fmt.Sprintf("%s\n__exit=$?\n%s > \"$DISCOBOT_BASH_CWD_PATH\"\nexit $__exit", command, bashPwdCaptureCommand())
	}
	return strings.Join([]string{
		`$ErrorActionPreference = "Continue"`,
		command,
		`$__discobot_exit = if ($null -ne $LASTEXITCODE) { $LASTEXITCODE } elseif ($?) { 0 } else { 1 }`,
		`(Get-Location).Path | Set-Content -LiteralPath $env:DISCOBOT_BASH_CWD_PATH -NoNewline`,
		`exit $__discobot_exit`,
	}, "\n")
}

func shellCommandArgsForOS(goos string, command string) []string {
	if goos != "windows" {
		return []string{"-c", command}
	}
	return []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command}
}

func backgroundShellHintsForOS(goos, logPath string, pid int) (follow, stop string) {
	if goos != "windows" {
		return fmt.Sprintf("tail -f %s", logPath), fmt.Sprintf("kill %d", pid)
	}
	return fmt.Sprintf("Get-Content -Wait %s", logPath), fmt.Sprintf("Stop-Process -Id %d", pid)
}

// extractCwdFromOutput splits the sentinel + pwd from the end of the output.
// Returns (userOutput, newCwd).
func extractCwdFromOutput(raw, sentinel string) (string, string) {
	idx := strings.LastIndex(raw, sentinel)
	if idx < 0 {
		return raw, ""
	}
	userOutput := raw[:idx]
	after := strings.TrimPrefix(raw[idx:], sentinel)
	after = strings.TrimPrefix(after, "\n")
	lines := strings.SplitN(after, "\n", 2)
	newCwd := strings.TrimSpace(lines[0])
	return userOutput, newCwd
}

func bashPwdCaptureCommand() string {
	if runtime.GOOS != "windows" {
		return "pwd"
	}
	return `(pwd -W 2>/dev/null || cygpath -w "$PWD" 2>/dev/null || pwd)`
}

func normalizeBashWorkingDir(cwd string) string {
	return normalizeBashWorkingDirForOS(runtime.GOOS, cwd)
}

func normalizeBashWorkingDirForOS(goos, cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" || goos != "windows" {
		return cwd
	}
	if isWindowsAbsolutePath(cwd) {
		return filepath.Clean(strings.ReplaceAll(cwd, "/", `\`))
	}
	if converted, ok := convertWindowsBashPwd(cwd); ok {
		return converted
	}
	return cwd
}

func isWindowsAbsolutePath(path string) bool {
	if len(path) >= 3 && isASCIIAlpha(path[0]) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`)
}

func convertWindowsBashPwd(cwd string) (string, bool) {
	if drive, rest, ok := splitBashDrivePath(cwd, "/"); ok {
		return buildWindowsDrivePath(drive, rest), true
	}
	if drive, rest, ok := splitBashDrivePath(cwd, "/mnt/"); ok {
		return buildWindowsDrivePath(drive, rest), true
	}
	return "", false
}

func isASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func splitBashDrivePath(cwd, prefix string) (byte, string, bool) {
	if !strings.HasPrefix(cwd, prefix) {
		return 0, "", false
	}
	idx := len(prefix)
	if len(cwd) < idx+1 {
		return 0, "", false
	}
	drive := cwd[idx]
	if !isASCIIAlpha(drive) {
		return 0, "", false
	}
	if len(cwd) > idx+1 && cwd[idx+1] != '/' {
		return 0, "", false
	}
	rest := ""
	if len(cwd) > idx+2 {
		rest = cwd[idx+2:]
	}
	return drive, rest, true
}

func buildWindowsDrivePath(drive byte, rest string) string {
	base := strings.ToUpper(string(drive)) + `:\`
	if rest == "" {
		return filepath.Clean(base)
	}
	return filepath.Clean(base + strings.ReplaceAll(rest, "/", `\`))
}

// getCwd returns the current working directory for this tool execution.
func (e *Executor) getCwd(toolCtx *thread.ToolContext) string {
	if toolCtx != nil && strings.TrimSpace(toolCtx.CurrentWorkingDirectory) != "" {
		return toolCtx.CurrentWorkingDirectory
	}
	e.cwdMu.Lock()
	defer e.cwdMu.Unlock()
	return e.currentCwd
}

// setCwd updates the current working directory for this tool execution.
func (e *Executor) setCwd(toolCtx *thread.ToolContext, cwd string) error {
	if toolCtx != nil {
		if toolCtx.SetCurrentWorkingDirectory != nil {
			return toolCtx.SetCurrentWorkingDirectory(cwd)
		}
		toolCtx.CurrentWorkingDirectory = cwd
		return nil
	}
	e.cwdMu.Lock()
	defer e.cwdMu.Unlock()
	e.currentCwd = cwd
	return nil
}
