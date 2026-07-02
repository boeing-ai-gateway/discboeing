package scriptexec

import (
	"bufio"
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

	"github.com/boeing-ai-gateway/discboeing/agent-go/sessionconfig"
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
	var args []string
	if rawArgs != "" {
		args = []string{rawArgs}
	}

	cmd, err := commandForScript(ctx, script.Path, args)
	if err != nil {
		return Execution{}, fmt.Errorf("run script %s: %w", script.Name, err)
	}
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

func commandForScript(ctx context.Context, scriptPath string, args []string) (*exec.Cmd, error) {
	if runtime.GOOS != "windows" {
		return exec.CommandContext(ctx, scriptPath, args...), nil
	}

	switch strings.ToLower(filepath.Ext(scriptPath)) {
	case ".bat", ".cmd", ".com", ".exe":
		return exec.CommandContext(ctx, scriptPath, args...), nil
	case ".ps1":
		return exec.CommandContext(ctx, "powershell", append([]string{"-File", scriptPath}, args...)...), nil
	}

	interpreter, interpreterArgs, err := interpreterForScript(scriptPath)
	if err != nil {
		return nil, err
	}
	if interpreter == "" {
		return exec.CommandContext(ctx, scriptPath, args...), nil
	}

	return exec.CommandContext(
		ctx,
		interpreter,
		append(append(interpreterArgs, scriptPath), args...)...,
	), nil
}

func interpreterForScript(scriptPath string) (string, []string, error) {
	file, err := os.Open(scriptPath)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	line, err := bufio.NewReader(file).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", nil, err
	}
	if !strings.HasPrefix(line, "#!") {
		return "", nil, nil
	}

	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "#!")))
	if len(fields) == 0 {
		return "", nil, nil
	}
	if filepath.Base(fields[0]) == "env" {
		if len(fields) == 1 {
			return "", nil, nil
		}
		return fields[1], fields[2:], nil
	}
	if strings.HasPrefix(fields[0], "/") {
		fields[0] = filepath.Base(fields[0])
	}
	return fields[0], fields[1:], nil
}
