package docker

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscboeingSessionEnvScriptLoadsValidEntries(t *testing.T) {
	tempDir := t.TempDir()
	agentEnv := filepath.Join(tempDir, "agent.env")
	workspaceEnv := filepath.Join(tempDir, "workspace.env")

	if err := os.WriteFile(agentEnv, []byte("FROM_AGENT=agent\nQUOTED=\"hello from agent\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile(agent): %v", err)
	}
	if err := os.WriteFile(workspaceEnv, []byte("FROM_WORKSPACE=workspace\nexport QUOTED='hello from workspace'\n"), 0644); err != nil {
		t.Fatalf("WriteFile(workspace): %v", err)
	}

	stdout, stderr, err := runSessionEnvScript(t, agentEnv, workspaceEnv, "env")
	if err != nil {
		t.Fatalf("runSessionEnvScript failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr:\n%s", stderr)
	}

	if !strings.Contains(stdout, "FROM_AGENT=agent\n") {
		t.Fatalf("stdout missing FROM_AGENT value:\n%s", stdout)
	}
	if !strings.Contains(stdout, "FROM_WORKSPACE=workspace\n") {
		t.Fatalf("stdout missing FROM_WORKSPACE value:\n%s", stdout)
	}
	if !strings.Contains(stdout, "QUOTED=hello from workspace\n") {
		t.Fatalf("stdout missing QUOTED value:\n%s", stdout)
	}
}

func TestDiscboeingSessionEnvScriptIgnoresInvalidAndMaliciousLines(t *testing.T) {
	tempDir := t.TempDir()
	agentEnv := filepath.Join(tempDir, "agent.env")
	workspaceEnv := filepath.Join(tempDir, "workspace.env")
	markerFile := filepath.Join(tempDir, "marker")
	shellMarkerFile := sessionEnvShellPath(t, markerFile)
	secret := "TOP_SECRET_SHOULD_NOT_APPEAR"

	if err := os.WriteFile(agentEnv, []byte("FROM_AGENT=agent\n"), 0644); err != nil {
		t.Fatalf("WriteFile(agent): %v", err)
	}
	content := strings.Join([]string{
		"SAFE=ok",
		"MALICIOUS_VALUE=$(touch " + shellMarkerFile + ")",
		"not an assignment " + secret,
		`BROKEN_QUOTE="` + secret,
		"$(touch " + shellMarkerFile + ")",
	}, "\n") + "\n"
	if err := os.WriteFile(workspaceEnv, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(workspace): %v", err)
	}

	stdout, stderr, err := runSessionEnvScript(t, agentEnv, workspaceEnv, "env")
	if err != nil {
		t.Fatalf("runSessionEnvScript failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "SAFE=ok\n") {
		t.Fatalf("stdout missing SAFE value:\n%s", stdout)
	}
	if !strings.Contains(stdout, "MALICIOUS_VALUE=$(touch "+shellMarkerFile+")\n") {
		t.Fatalf("stdout missing MALICIOUS_VALUE literal:\n%s", stdout)
	}
	if _, err := os.Stat(markerFile); !os.IsNotExist(err) {
		t.Fatalf("marker file should not exist, stat err=%v", err)
	}

	if got := strings.Count(stderr, "ignoring invalid env line"); got != 3 {
		t.Fatalf("warning count = %d, want 3\nstderr:\n%s", got, stderr)
	}
	if !strings.Contains(stderr, sessionEnvShellPath(t, workspaceEnv)) {
		t.Fatalf("stderr missing workspace env path:\n%s", stderr)
	}
	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked invalid env content:\n%s", stderr)
	}
}

func runSessionEnvScript(t *testing.T, agentEnvPath, workspaceEnvPath string, args ...string) (string, string, error) {
	t.Helper()

	cmdArgs := append([]string{sessionEnvShellPath(t, sessionEnvScriptPath(t))}, args...)
	cmd := exec.Command(sessionEnvShell(), cmdArgs...)
	cmd.Env = append(os.Environ(),
		"DISCBOEING_AGENT_ENV_FILE="+sessionEnvShellPath(t, agentEnvPath),
		"DISCBOEING_WORKSPACE_ENV_FILE="+sessionEnvShellPath(t, workspaceEnvPath),
	)
	if runtime.GOOS == "windows" {
		cmd.Env = append(cmd.Env, "WSLENV="+appendWSLEnv(os.Getenv("WSLENV"),
			"DISCBOEING_AGENT_ENV_FILE/u",
			"DISCBOEING_WORKSPACE_ENV_FILE/u",
		))
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func sessionEnvShell() string {
	if runtime.GOOS == "windows" {
		return "bash"
	}
	return "/bin/sh"
}

func sessionEnvScriptPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../container-assets/discboeing-session-env.sh"))
}

func sessionEnvShellPath(t *testing.T, filePath string) string {
	t.Helper()

	if runtime.GOOS != "windows" {
		return filePath
	}

	cleanPath := filepath.Clean(filePath)
	if len(cleanPath) < 3 || cleanPath[1] != ':' {
		return filepath.ToSlash(cleanPath)
	}

	drive := strings.ToLower(cleanPath[:1])
	remainder := strings.TrimPrefix(filepath.ToSlash(cleanPath[2:]), "/")
	if shellPathExists("/mnt/" + drive) {
		if remainder == "" {
			return "/mnt/" + drive
		}
		return "/mnt/" + drive + "/" + remainder
	}
	if shellPathExists("/" + drive) {
		if remainder == "" {
			return "/" + drive
		}
		return "/" + drive + "/" + remainder
	}
	return filepath.ToSlash(cleanPath)
}

func shellPathExists(path string) bool {
	quotedPath := strings.ReplaceAll(path, "'", `'\''`)
	command := "test -e '" + quotedPath + "'"
	return exec.Command(sessionEnvShell(), "-lc", command).Run() == nil
}

func appendWSLEnv(current string, entries ...string) string {
	if current == "" {
		return strings.Join(entries, ":")
	}
	return current + ":" + strings.Join(entries, ":")
}
