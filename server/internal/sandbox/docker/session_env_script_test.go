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

func TestDiscobotSessionEnvScriptLoadsValidEntries(t *testing.T) {
	tempDir := t.TempDir()
	agentEnv := filepath.Join(tempDir, "agent.env")
	workspaceEnv := filepath.Join(tempDir, "workspace.env")

	if err := os.WriteFile(agentEnv, []byte("FROM_AGENT=agent\nQUOTED=\"hello from agent\"\n"), 0644); err != nil {
		t.Fatalf("WriteFile(agent): %v", err)
	}
	if err := os.WriteFile(workspaceEnv, []byte("FROM_WORKSPACE=workspace\nexport QUOTED='hello from workspace'\n"), 0644); err != nil {
		t.Fatalf("WriteFile(workspace): %v", err)
	}

	shell := sessionEnvShell()
	stdout, stderr, err := runSessionEnvScript(t, agentEnv, workspaceEnv,
		shell, "-c", `printf 'FROM_AGENT=%s\n' "$FROM_AGENT"; printf 'FROM_WORKSPACE=%s\n' "$FROM_WORKSPACE"; printf 'QUOTED=%s\n' "$QUOTED"`)
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

func TestDiscobotSessionEnvScriptIgnoresInvalidAndMaliciousLines(t *testing.T) {
	tempDir := t.TempDir()
	agentEnv := filepath.Join(tempDir, "agent.env")
	workspaceEnv := filepath.Join(tempDir, "workspace.env")
	markerFile := filepath.Join(tempDir, "marker")
	secret := "TOP_SECRET_SHOULD_NOT_APPEAR"

	if err := os.WriteFile(agentEnv, []byte("FROM_AGENT=agent\n"), 0644); err != nil {
		t.Fatalf("WriteFile(agent): %v", err)
	}
	content := strings.Join([]string{
		"SAFE=ok",
		"MALICIOUS_VALUE=$(touch " + markerFile + ")",
		"not an assignment " + secret,
		`BROKEN_QUOTE="` + secret,
		"$(touch " + markerFile + ")",
	}, "\n") + "\n"
	if err := os.WriteFile(workspaceEnv, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(workspace): %v", err)
	}

	shell := sessionEnvShell()
	stdout, stderr, err := runSessionEnvScript(t, agentEnv, workspaceEnv,
		shell, "-c", `printf 'SAFE=%s\n' "$SAFE"; printf 'MALICIOUS_VALUE=%s\n' "$MALICIOUS_VALUE"; echo READY`)
	if err != nil {
		t.Fatalf("runSessionEnvScript failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	if !strings.Contains(stdout, "SAFE=ok\n") {
		t.Fatalf("stdout missing SAFE value:\n%s", stdout)
	}
	if !strings.Contains(stdout, "MALICIOUS_VALUE=$(touch "+markerFile+")\n") {
		t.Fatalf("stdout missing MALICIOUS_VALUE literal:\n%s", stdout)
	}
	if !strings.Contains(stdout, "READY\n") {
		t.Fatalf("stdout missing READY marker:\n%s", stdout)
	}
	if _, err := os.Stat(markerFile); !os.IsNotExist(err) {
		t.Fatalf("marker file should not exist, stat err=%v", err)
	}

	if got := strings.Count(stderr, "ignoring invalid env line"); got != 3 {
		t.Fatalf("warning count = %d, want 3\nstderr:\n%s", got, stderr)
	}
	if !strings.Contains(stderr, workspaceEnv) {
		t.Fatalf("stderr missing workspace env path:\n%s", stderr)
	}
	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked invalid env content:\n%s", stderr)
	}
}

func runSessionEnvScript(t *testing.T, agentEnvPath, workspaceEnvPath string, args ...string) (string, string, error) {
	t.Helper()

	cmdArgs := append([]string{sessionEnvScriptPath(t)}, args...)
	cmd := exec.Command(sessionEnvShell(), cmdArgs...)
	cmd.Env = append(os.Environ(),
		"DISCOBOT_AGENT_ENV_FILE="+agentEnvPath,
		"DISCOBOT_WORKSPACE_ENV_FILE="+workspaceEnvPath,
	)
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
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../container-assets/discobot-session-env.sh"))
}
