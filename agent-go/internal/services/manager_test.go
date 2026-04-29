//go:build !windows

package services

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGetServicesIncludesBuiltInVSCodeWhenAvailable(t *testing.T) {
	workspaceRoot := t.TempDir()

	originalLookPath := lookPath
	lookPath = func(file string) (string, error) {
		if file == "code-server" {
			return "/usr/bin/code-server", nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() {
		lookPath = originalLookPath
	})

	mgr := NewManager()
	services, err := mgr.GetServices(workspaceRoot)
	if err != nil {
		t.Fatalf("GetServices() failed: %v", err)
	}

	var found *ServiceInfo
	for i := range services {
		if services[i].ID == vscodeService.ID {
			found = &services[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("GetServices() did not include %q", vscodeService.ID)
	}
	if !found.Passive {
		t.Fatalf("VS Code service passive = %v, want true", found.Passive)
	}
	if found.HTTP != vscodeService.HTTP {
		t.Fatalf("VS Code service HTTP port = %d, want %d", found.HTTP, vscodeService.HTTP)
	}
}

func TestGetServiceReturnsBuiltInVSCodeWhenAvailable(t *testing.T) {
	workspaceRoot := t.TempDir()

	originalLookPath := lookPath
	lookPath = func(file string) (string, error) {
		if file == "code-server" {
			return "/usr/bin/code-server", nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() {
		lookPath = originalLookPath
	})

	mgr := NewManager()
	svc, err := mgr.GetService(workspaceRoot, vscodeService.ID)
	if err != nil {
		t.Fatalf("GetService() failed: %v", err)
	}
	if svc == nil {
		t.Fatalf("GetService() returned nil for %q", vscodeService.ID)
	}
	if svc.ID != vscodeService.ID {
		t.Fatalf("service ID = %q, want %q", svc.ID, vscodeService.ID)
	}
	if svc.Status != "running" {
		t.Fatalf("service status = %q, want running", svc.Status)
	}
}

func TestStopServiceKillsEntireProcessGroup(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	pidDir := filepath.Join(workspaceRoot, "pids")
	t.Setenv("HOME", homeDir)

	servicesDir := filepath.Join(workspaceRoot, ServicesDir)
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	serviceID := "group-stop-test"
	scriptPath := filepath.Join(servicesDir, serviceID+".sh")
	// Use #!/bin/sh and POSIX-only constructs so the test works on macOS
	// (system bash is 3.2, which lacks $BASHPID; $PPID is POSIX-standard).
	// For the top-level script $$ is the PID.  For subshells we can't use $$
	// (it gives the parent's PID), so we spawn a child sh whose $PPID is the
	// bash subshell's PID.
	script := fmt.Sprintf(`#!/bin/sh
set -eu
pid_dir=%q

printf '%%s\n' "$$" > "$pid_dir/parent.pid"

(
  sh -c 'printf "%%s\n" "$PPID"' > "$pid_dir/child.pid"
  (
    sh -c 'printf "%%s\n" "$PPID"' > "$pid_dir/grandchild.pid"
    exec tail -f /dev/null
  ) &
  wait
) &
wait
`, pidDir)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager()
	if _, code, err := mgr.StartService(workspaceRoot, serviceID); err != nil {
		t.Fatalf("StartService() failed: code=%q err=%v", code, err)
	}

	parentPID := waitForPIDFile(t, filepath.Join(pidDir, "parent.pid"))
	childPID := waitForPIDFile(t, filepath.Join(pidDir, "child.pid"))
	grandchildPID := waitForPIDFile(t, filepath.Join(pidDir, "grandchild.pid"))

	waitForProcessesAlive(t, parentPID, childPID, grandchildPID)

	t.Cleanup(func() {
		_ = killProcessGroup(parentPID, syscall.SIGKILL)
		for _, pid := range []int{parentPID, childPID, grandchildPID} {
			if processExists(pid) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})

	_, unsubscribe, closeCh := mgr.Subscribe(serviceID)
	defer unsubscribe()
	if closeCh == nil {
		t.Fatal("Subscribe() returned nil close channel")
	}

	if code, err := mgr.StopService(serviceID); err != nil {
		t.Fatalf("StopService() failed: code=%q err=%v", code, err)
	}

	select {
	case <-closeCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for service to stop")
	}

	svc, err := mgr.GetService(workspaceRoot, serviceID)
	if err != nil {
		t.Fatalf("GetService() failed: %v", err)
	}
	if svc == nil {
		t.Fatal("GetService() returned nil service")
		return
	}
	if svc.Status != "stopped" {
		t.Fatalf("service status = %q, want stopped", svc.Status)
	}

	waitForProcessesGone(t, parentPID, childPID, grandchildPID)
}

func TestStartServiceUsesVisibleEnvSnapshotAtLaunch(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	outputPath := filepath.Join(workspaceRoot, "visible-env.txt")
	t.Setenv("HOME", homeDir)
	t.Setenv("VISIBLE_FROM_OS", "os-value")

	servicesDir := filepath.Join(workspaceRoot, ServicesDir)
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	serviceID := "visible-env-test"
	scriptPath := filepath.Join(servicesDir, serviceID+".sh")
	script := fmt.Sprintf(`#!/bin/sh
set -eu
printf 'VISIBLE_AT_LAUNCH=%%s\n' "${VISIBLE_AT_LAUNCH:-}" > %q
printf 'VISIBLE_FROM_OS=%%s\n' "${VISIBLE_FROM_OS:-}" >> %q
`, outputPath, outputPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager()
	visibleEnv := map[string]string{"VISIBLE_AT_LAUNCH": "first-value"}
	mgr.SetEnvSnapshot(func() map[string]string {
		snapshot := make(map[string]string, len(visibleEnv))
		maps.Copy(snapshot, visibleEnv)
		return snapshot
	})

	if _, code, err := mgr.StartService(workspaceRoot, serviceID); err != nil {
		t.Fatalf("StartService() failed: code=%q err=%v", code, err)
	}

	visibleEnv["VISIBLE_AT_LAUNCH"] = "second-value"

	waitForCondition(t, 5*time.Second, func() (bool, string, error) {
		data, err := os.ReadFile(outputPath)
		if os.IsNotExist(err) {
			return false, "waiting for service output file", nil
		}
		if err != nil {
			return false, "", err
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			return false, "waiting for service output contents", nil
		}
		return true, "", nil
	})

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "VISIBLE_AT_LAUNCH=first-value") {
		t.Fatalf("service did not receive request-scoped visible env at launch: %q", got)
	}
	if !strings.Contains(got, "VISIBLE_FROM_OS=os-value") {
		t.Fatalf("service did not preserve process env: %q", got)
	}
	if strings.Contains(got, "VISIBLE_AT_LAUNCH=second-value") {
		t.Fatalf("service used a later env snapshot instead of launch-time values: %q", got)
	}
	waitForServiceStopped(t, mgr, workspaceRoot, serviceID)
}

func TestStartServiceReloadsWorkspaceEnvOnEachLaunch(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	outputPath := filepath.Join(workspaceRoot, "workspace-env.txt")
	t.Setenv("HOME", homeDir)

	envDir := filepath.Join(workspaceRoot, ".discobot")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(envDir) failed: %v", err)
	}
	envPath := filepath.Join(envDir, "env")
	if err := os.WriteFile(envPath, []byte("WORKSPACE_DYNAMIC=first\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(first env) failed: %v", err)
	}

	servicesDir := filepath.Join(workspaceRoot, ServicesDir)
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(servicesDir) failed: %v", err)
	}

	serviceID := "workspace-env-test"
	scriptPath := filepath.Join(servicesDir, serviceID+".sh")
	script := fmt.Sprintf(`#!/bin/sh
set -eu
printf 'WORKSPACE_DYNAMIC=%%s\n' "${WORKSPACE_DYNAMIC:-}" > %q
`, outputPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(script) failed: %v", err)
	}

	mgr := NewManager()

	if _, code, err := mgr.StartService(workspaceRoot, serviceID); err != nil {
		t.Fatalf("StartService(first) failed: code=%q err=%v", code, err)
	}
	waitForServiceStopped(t, mgr, workspaceRoot, serviceID)

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(first output) failed: %v", err)
	}
	if got := string(data); !strings.Contains(got, "WORKSPACE_DYNAMIC=first") {
		t.Fatalf("first launch output = %q, want first workspace env value", got)
	}

	if err := os.WriteFile(envPath, []byte("WORKSPACE_DYNAMIC=second\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(second env) failed: %v", err)
	}

	if _, code, err := mgr.StartService(workspaceRoot, serviceID); err != nil {
		t.Fatalf("StartService(second) failed: code=%q err=%v", code, err)
	}
	waitForServiceStopped(t, mgr, workspaceRoot, serviceID)

	data, err = os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(second output) failed: %v", err)
	}
	if got := string(data); !strings.Contains(got, "WORKSPACE_DYNAMIC=second") {
		t.Fatalf("second launch output = %q, want updated workspace env value", got)
	}

	if err := os.WriteFile(envPath, []byte("WORKSPACE_OTHER=present\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(third env) failed: %v", err)
	}

	if _, code, err := mgr.StartService(workspaceRoot, serviceID); err != nil {
		t.Fatalf("StartService(third) failed: code=%q err=%v", code, err)
	}
	waitForServiceStopped(t, mgr, workspaceRoot, serviceID)

	data, err = os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(third output) failed: %v", err)
	}
	if got := string(data); !strings.Contains(got, "WORKSPACE_DYNAMIC=") || strings.Contains(got, "WORKSPACE_DYNAMIC=second") {
		t.Fatalf("third launch output = %q, want removed workspace env key to be unset", got)
	}
}

func waitForServiceStopped(t *testing.T, mgr *Manager, workspaceRoot, serviceID string) {
	t.Helper()

	waitForCondition(t, 5*time.Second, func() (bool, string, error) {
		svc, err := mgr.GetService(workspaceRoot, serviceID)
		if err != nil {
			return false, "", err
		}
		if svc == nil {
			return false, "waiting for service state", nil
		}
		if svc.Status != "stopped" {
			return false, "waiting for service to stop", nil
		}
		return true, "", nil
	})
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()

	var pid int
	waitForCondition(t, 5*time.Second, func() (bool, string, error) {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return false, "waiting for pid file", nil
		}
		if err != nil {
			return false, "", err
		}

		text := strings.TrimSpace(string(data))
		if text == "" {
			return false, "waiting for pid file contents", nil
		}

		parsed, err := strconv.Atoi(text)
		if err != nil {
			return false, "", err
		}
		if parsed <= 0 {
			return false, fmt.Sprintf("pid %d is not positive", parsed), nil
		}

		pid = parsed
		return true, "", nil
	})

	return pid
}

func waitForProcessesAlive(t *testing.T, pids ...int) {
	t.Helper()

	waitForCondition(t, 5*time.Second, func() (bool, string, error) {
		var dead []string
		for _, pid := range pids {
			if !processExists(pid) {
				dead = append(dead, strconv.Itoa(pid))
			}
		}
		if len(dead) > 0 {
			return false, fmt.Sprintf("processes not alive yet: %s", strings.Join(dead, ", ")), nil
		}
		return true, "", nil
	})
}

func waitForProcessesGone(t *testing.T, pids ...int) {
	t.Helper()

	waitForCondition(t, 5*time.Second, func() (bool, string, error) {
		var alive []string
		for _, pid := range pids {
			if processExists(pid) {
				alive = append(alive, strconv.Itoa(pid))
			}
		}
		if len(alive) > 0 {
			return false, fmt.Sprintf("processes still alive: %s", strings.Join(alive, ", ")), nil
		}
		return true, "", nil
	})
}

func waitForCondition(t *testing.T, timeout time.Duration, check func() (bool, string, error)) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastReason string
	for time.Now().Before(deadline) {
		ok, reason, err := check()
		if err != nil {
			t.Fatalf("condition check failed: %v", err)
		}
		if ok {
			return
		}
		if reason != "" {
			lastReason = reason
		}
		time.Sleep(25 * time.Millisecond)
	}

	if lastReason == "" {
		lastReason = "condition was not satisfied"
	}
	t.Fatal(lastReason)
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
