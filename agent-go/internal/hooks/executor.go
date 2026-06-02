package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/processes"
	"github.com/obot-platform/discobot/agent-go/internal/workspaceenv"
)

// DefaultTimeout is the default hook execution timeout (15 minutes).
const DefaultTimeout = 15 * time.Minute

// HookResult is the result of executing a hook.
type HookResult struct {
	Success    bool
	NotifyLLM  *bool
	ExitCode   int
	Output     string
	Hook       Hook
	DurationMs int64
}

// ExecuteOptions configures how a hook is executed.
type ExecuteOptions struct {
	Cwd          string
	Env          map[string]string
	Timeout      time.Duration
	ChangedFiles []string
	SessionID    string
	OutputPath   string
	Processes    *processes.Manager
}

// ExecuteHook runs a hook script and returns the result.
func ExecuteHook(hook Hook, opts ExecuteOptions) HookResult {
	if opts.Processes != nil {
		return executeHookProcess(hook, opts)
	}

	start := time.Now()

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command, args := buildHookCommandForRunAs(hook)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = opts.Cwd

	// Build environment
	env := workspaceenv.MergeProcessSnapshot(opts.Env)
	env["DISCOBOT_HOOK_TYPE"] = string(hook.Type)
	if opts.SessionID != "" {
		env["DISCOBOT_SESSION_ID"] = opts.SessionID
	}
	if opts.Cwd != "" {
		env["DISCOBOT_WORKSPACE"] = opts.Cwd
	}
	if len(opts.ChangedFiles) > 0 {
		env["DISCOBOT_CHANGED_FILES"] = strings.Join(opts.ChangedFiles, " ")
	}
	cmd.Env = workspaceenv.List(env)

	// Set process group so we can kill the entire group on timeout
	setSysProcAttr(cmd)

	// Capture stdout and stderr
	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Start()
	if err != nil {
		result := HookResult{
			Success:    false,
			ExitCode:   127,
			Output:     err.Error(),
			Hook:       hook,
			DurationMs: time.Since(start).Milliseconds(),
		}
		writeOutputFile(opts.OutputPath, result.Output)
		return result
	}

	err = cmd.Wait()
	durationMs := time.Since(start).Milliseconds()
	output := outputBuf.String()

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Kill process group on timeout
			if cmd.Process != nil {
				_ = killProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
			}
			exitCode = 124
			output += fmt.Sprintf("\n[Hook timed out after %ds and was killed]\n", int(timeout.Seconds()))
		} else {
			exitErr := new(exec.ExitError)
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 126
			}
		}
	}

	result := HookResult{
		Success:    exitCode == 0,
		ExitCode:   exitCode,
		Output:     output,
		Hook:       hook,
		DurationMs: durationMs,
	}

	writeOutputFile(opts.OutputPath, output)

	return result
}

func executeHookProcess(hook Hook, opts ExecuteOptions) HookResult {
	start := time.Now()
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	command, args := buildHookCommandForRunAs(hook)
	env := hookEnv(hook, opts)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	writeOutputFile(opts.OutputPath, "")

	session, err := opts.Processes.Start(ctx, processes.CreateRequest{
		Kind:    processes.KindHook,
		Name:    hook.Name,
		Cmd:     append([]string{command}, args...),
		WorkDir: opts.Cwd,
		Env:     env,
		LogDir:  processSidecarDir(opts.OutputPath),
		LogPath: opts.OutputPath,
		Metadata: map[string]string{
			"hookId": hook.ID,
			"type":   string(hook.Type),
		},
	})
	if err != nil {
		result := HookResult{
			Success:    false,
			ExitCode:   127,
			Output:     err.Error(),
			Hook:       hook,
			DurationMs: time.Since(start).Milliseconds(),
		}
		writeOutputFile(opts.OutputPath, result.Output)
		return result
	}

	_, unsubscribe, done, err := opts.Processes.Subscribe(session.ID)
	if err != nil {
		_ = opts.Processes.Kill(session.ID)
		result := HookResult{
			Success:    false,
			ExitCode:   126,
			Output:     err.Error(),
			Hook:       hook,
			DurationMs: time.Since(start).Milliseconds(),
		}
		writeOutputFile(opts.OutputPath, result.Output)
		return result
	}
	defer unsubscribe()

	timedOut := false
	timer := time.NewTimer(timeout)
	select {
	case <-done:
	case <-timer.C:
		timedOut = true
		_ = opts.Processes.Kill(session.ID)
		<-done
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	output := readOutputFile(opts.OutputPath)
	exitCode := 0
	if timedOut {
		exitCode = 124
		timeoutMessage := fmt.Sprintf("\n[Hook timed out after %ds and was killed]\n", int(timeout.Seconds()))
		output += timeoutMessage
		appendOutputFile(opts.OutputPath, timeoutMessage)
	} else if session, err := opts.Processes.Get(session.ID); err == nil && session.ExitCode != nil {
		exitCode = *session.ExitCode
	}

	return HookResult{
		Success:    exitCode == 0,
		ExitCode:   exitCode,
		Output:     output,
		Hook:       hook,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

func hookEnv(hook Hook, opts ExecuteOptions) map[string]string {
	env := workspaceenv.MergeProcessSnapshot(opts.Env)
	env["DISCOBOT_HOOK_TYPE"] = string(hook.Type)
	if opts.SessionID != "" {
		env["DISCOBOT_SESSION_ID"] = opts.SessionID
	}
	if opts.Cwd != "" {
		env["DISCOBOT_WORKSPACE"] = opts.Cwd
	}
	if len(opts.ChangedFiles) > 0 {
		env["DISCOBOT_CHANGED_FILES"] = strings.Join(opts.ChangedFiles, " ")
	}
	return env
}

func buildHookCommandForRunAs(hook Hook) (string, []string) {
	command, args := buildHookCommand(hook.Path)
	if hook.RunAs != "root" || runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return command, args
	}
	return "sudo", append([]string{command}, args...)
}

// GetHookOutputPath returns the path to a hook's output log file.
func GetHookOutputPath(hooksDataDir, hookID string) string {
	return filepath.Join(hooksDataDir, "output", hookID+".log")
}

// writeOutputFile writes output to a file, creating parent directories.
func writeOutputFile(path, output string) {
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(output), 0o644)
}

func appendOutputFile(path, output string) {
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = io.WriteString(f, output)
}

func readOutputFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func processSidecarDir(outputPath string) string {
	if outputPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(outputPath), "."+filepath.Base(outputPath)+".process")
}

func buildHookCommand(path string) (string, []string) {
	if runtime.GOOS != "windows" {
		return path, nil
	}

	interpreter, args := parseScriptShebang(path)
	switch interpreter {
	case "bash", "sh", "zsh":
		return "bash", append(args, path)
	case "":
		return path, nil
	default:
		return interpreter, append(args, path)
	}
}

func parseScriptShebang(path string) (string, []string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}

	line, _, _ := strings.Cut(string(content), "\n")
	if !strings.HasPrefix(line, "#!") {
		return "", nil
	}

	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "#!")))
	if len(fields) == 0 {
		return "", nil
	}

	interpreter := filepath.Base(fields[0])
	args := fields[1:]
	if interpreter == "env" {
		for len(args) > 0 && args[0] == "-S" {
			args = args[1:]
		}
		if len(args) == 0 {
			return "", nil
		}
		interpreter = filepath.Base(args[0])
		args = args[1:]
	}

	return strings.ToLower(interpreter), args
}
