package scriptexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/obot-platform/discobot/agent-go/sessionconfig"
)

type Execution struct {
	Script   sessionconfig.ScriptConfig
	Stdout   string
	Stderr   string
	ExitCode int
	Success  bool
}

func (e Execution) TrimmedStdout() string {
	return strings.TrimSpace(e.Stdout)
}

func (e Execution) HasVisibleStdout() bool {
	return e.TrimmedStdout() != ""
}

func (e Execution) FormatForLLM() string {
	if e.Success {
		return e.TrimmedStdout()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "<script_execution name=%q exit_code=%d success=%q>\n", e.Script.Name, e.ExitCode, "false")
	if stdout := strings.TrimSpace(e.Stdout); stdout != "" {
		b.WriteString("<stdout>\n")
		b.WriteString(stdout)
		b.WriteString("\n</stdout>\n")
	}
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		b.WriteString("<stderr>\n")
		b.WriteString(stderr)
		b.WriteString("\n</stderr>\n")
	}
	b.WriteString("</script_execution>")
	return b.String()
}

func RunNamed(ctx context.Context, projectRoot, workDir string, env []string, scriptName, rawArgs string, visibleOnly bool) (Execution, error) {
	script, found, err := sessionconfig.LookupScript(projectRoot, scriptName, visibleOnly)
	if err != nil {
		return Execution{}, err
	}
	if !found {
		if visibleOnly {
			return Execution{}, fmt.Errorf("script %q not found in configured script directories", scriptName)
		}
		return Execution{}, fmt.Errorf("script %q not found", scriptName)
	}
	return RunDiscovered(ctx, script, workDir, env, rawArgs)
}

func RunDiscovered(ctx context.Context, script sessionconfig.ScriptConfig, workDir string, env []string, rawArgs string) (Execution, error) {
	args, err := sessionconfig.SplitScriptArgs(rawArgs)
	if err != nil {
		return Execution{}, err
	}

	cmd := exec.CommandContext(ctx, script.Path, args...)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}
	if len(env) > 0 {
		cmd.Env = env
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	execErr := cmd.Run()
	result := Execution{
		Script:   script,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Success:  execErr == nil,
		ExitCode: 0,
	}
	if execErr == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(execErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return Execution{}, fmt.Errorf("run script %s: %w", script.Name, execErr)
}
