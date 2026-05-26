//go:build windows

package wsl

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/obot-platform/discobot/server/internal/config"
)

func TestDecodeCommandOutputUTF16LE(t *testing.T) {
	input := encodeUTF16ForTest("  NAME                   STATE           VERSION\r\n* Ubuntu-24.04           Running         2\r\n  discobot               Stopped         2\r\n", binary.LittleEndian)

	got := decodeCommandOutput(input)

	if !strings.Contains(got, "discobot") {
		t.Fatalf("decodeCommandOutput() missing distro name: %q", got)
	}
	if strings.ContainsRune(got, '\x00') {
		t.Fatalf("decodeCommandOutput() still contains NUL bytes: %q", got)
	}
}

func TestWaitForCommandSuccessWithFallbackTimeoutUsesProvidedTimeout(t *testing.T) {
	t.Parallel()

	start := time.Now()
	err := (&Manager{}).waitForCommandSuccessWithFallbackTimeout(context.Background(), "test wait", 20*time.Millisecond, func(context.Context) error {
		return fmt.Errorf("not ready")
	})
	if err == nil {
		t.Fatal("waitForCommandSuccessWithFallbackTimeout() error = nil, want timeout")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("waitForCommandSuccessWithFallbackTimeout() error = %v, want deadline exceeded", err)
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatalf("waitForCommandSuccessWithFallbackTimeout() took too long: %v", time.Since(start))
	}
}

func TestWaitForCommandSuccessUntilCanceledWaitsForCallerContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := (&Manager{}).waitForCommandSuccessUntilCanceled(ctx, "test wait", func(context.Context) error {
		return fmt.Errorf("not ready")
	})
	if err == nil {
		t.Fatal("waitForCommandSuccessUntilCanceled() error = nil, want context cancellation")
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("waitForCommandSuccessUntilCanceled() error = %v, want canceled", err)
	}
	if time.Since(start) > 250*time.Millisecond {
		t.Fatalf("waitForCommandSuccessUntilCanceled() took too long: %v", time.Since(start))
	}
}

func TestWaitForSystemdReadyInDistroAcceptsDegradedState(t *testing.T) {
	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
	})

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	runCommandOutput = func(_ context.Context, name string, args ...string) (string, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if !slices.Equal(args, []string{"-d", "discobot", "--", "systemctl", "is-system-running"}) {
			t.Fatalf("unexpected command args: %v", args)
		}
		return "degraded\n", errors.New("exit status 1")
	}

	if err := manager.waitForSystemdReadyInDistro(context.Background(), "discobot"); err != nil {
		t.Fatalf("waitForSystemdReadyInDistro() error = %v", err)
	}
}

func TestWaitForNamedDistroRunnableStateWaitsForInstalling(t *testing.T) {
	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
	})

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) (string, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if !slices.Equal(args, []string{"--list", "--verbose"}) {
			t.Fatalf("unexpected command args: %v", args)
		}
		callIndex++
		if callIndex == 1 {
			return distroListForTest("Installing"), nil
		}
		return distroListForTest("Stopped"), nil
	}

	distro, err := manager.waitForNamedDistroRunnableState(context.Background(), "discobot")
	if err != nil {
		t.Fatalf("waitForNamedDistroRunnableState() error = %v", err)
	}
	if distro.State != "Stopped" {
		t.Fatalf("waitForNamedDistroRunnableState() state = %q, want %q", distro.State, "Stopped")
	}
	if callIndex != 2 {
		t.Fatalf("waitForNamedDistroRunnableState() calls = %d, want 2", callIndex)
	}
}

func TestShouldRecoverBrokenDistroRecognizesUnexpectedStop(t *testing.T) {
	if !shouldRecoverBrokenDistro(errors.New("managed WSL distro \"discobot\" stopped while waiting for docker.service readiness")) {
		t.Fatal("shouldRecoverBrokenDistro() = false, want true for stopped distro error")
	}
	if !shouldRecoverBrokenDistro(errors.New("managed WSL distro \"discobot\" disappeared while waiting for /var readiness")) {
		t.Fatal("shouldRecoverBrokenDistro() = false, want true for disappeared distro error")
	}
}

func TestVarDiskLabelSanitizesDistroName(t *testing.T) {
	t.Parallel()

	manager := NewManager(&config.Config{
		WSLDistroName: "Discobot Data",
		WSLStateDir:   t.TempDir(),
	})

	if got := manager.varDiskLabel(); got != "discobot-data-va" {
		t.Fatalf("varDiskLabel() = %q, want %q", got, "discobot-data-va")
	}
}

func TestIsStaleVarDiskUnmountError(t *testing.T) {
	t.Parallel()

	for _, message := range []string{
		"failed to detach disk",
		"invalid argument",
		"not mounted",
		"not attached",
		"The system cannot find the path specified",
	} {
		if !isStaleVarDiskUnmountError(message) {
			t.Fatalf("isStaleVarDiskUnmountError(%q) = false, want true", message)
		}
	}
	if isStaleVarDiskUnmountError("access is denied") {
		t.Fatal("isStaleVarDiskUnmountError(access is denied) = true, want false")
	}
}

func TestBuildDiscobotWSLEnvFileQuotesVarDiskLabel(t *testing.T) {
	t.Parallel()

	manager := &Manager{cfg: &config.Config{
		WSLDistroName: "Discobot Data",
		WSLStateDir:   t.TempDir(),
	}}

	got := manager.buildDiscobotWSLEnvFile()

	if !strings.Contains(got, "DISCOBOT_GUEST_PLATFORM='wsl'") {
		t.Fatalf("buildDiscobotWSLEnvFile() missing guest platform: %q", got)
	}
	if !strings.Contains(got, "DISCOBOT_VAR_DISK_LABEL='discobot-data-va'") {
		t.Fatalf("buildDiscobotWSLEnvFile() missing quoted disk label: %q", got)
	}
}

func TestWaitForVarReadyInDistroUsesMountpointOnly(t *testing.T) {
	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
	})

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	mountpointCalls := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) (string, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if slices.Equal(args, []string{"-d", "discobot", "--", "mountpoint", "-q", "/var"}) {
			mountpointCalls++
			return "", nil
		}
		t.Fatalf("unexpected command args: %v", args)
		return "", nil
	}

	err := manager.waitForVarReadyInDistro(context.Background(), "discobot")
	if err != nil {
		t.Fatalf("waitForVarReadyInDistro() error = %v", err)
	}
	if mountpointCalls != 1 {
		t.Fatalf("waitForVarReadyInDistro() mountpoint calls = %d, want 1", mountpointCalls)
	}
}

func TestEnsureMainDistroReadyReportsBootstrapRequiredForBrokenRuntimeDistro(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "discobot")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "marker.txt"), []byte("stale"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLInstallDir: installDir,
		WSLStateDir:   filepath.Join(root, "state"),
	})

	type expectedCommand struct {
		name   string
		args   []string
		output string
		err    error
	}
	sequence := []expectedCommand{
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "systemctl", "is-system-running"}, output: "Catastrophic failure\r\nError code: Wsl/Service/E_UNEXPECTED\r\n", err: errors.New("exit status 1")},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) (string, error) {
		if callIndex >= len(sequence) {
			t.Fatalf("unexpected extra command: %s %v", name, args)
		}
		expected := sequence[callIndex]
		callIndex++
		if name != expected.name {
			t.Fatalf("command %d name = %q, want %q", callIndex, name, expected.name)
		}
		if !slices.Equal(args, expected.args) {
			t.Fatalf("command %d args = %v, want %v", callIndex, args, expected.args)
		}
		return expected.output, expected.err
	}

	_, err := manager.ensureMainDistroReady(context.Background(), progressReporter{})
	if err == nil {
		t.Fatal("ensureMainDistroReady() error = nil, want bootstrap-required error")
	}
	var bootstrapErr *wslBootstrapRequiredError
	if !errors.As(err, &bootstrapErr) {
		t.Fatalf("ensureMainDistroReady() error = %T, want *wslBootstrapRequiredError", err)
	}
	if !slices.Contains(bootstrapErr.Actions, "repair-distro") {
		t.Fatalf("bootstrap actions = %v, want repair-distro", bootstrapErr.Actions)
	}
	if callIndex != len(sequence) {
		t.Fatalf("runCommandOutput() calls = %d, want %d", callIndex, len(sequence))
	}
	if _, err := os.Stat(filepath.Join(installDir, "marker.txt")); err != nil {
		t.Fatalf("install dir was modified during non-elevating recovery path: %v", err)
	}
}

func TestEnsureMainDistroReadyRetriesWhenDistroTemporarilyStopsDuringStartup(t *testing.T) {
	root := t.TempDir()
	varDiskPath := filepath.Join(root, "var.vhdx")
	if err := os.WriteFile(varDiskPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := NewManager(&config.Config{
		WSLDistroName:  "discobot",
		WSLStateDir:    root,
		WSLVarDiskPath: varDiskPath,
	})

	type expectedCommand struct {
		name   string
		args   []string
		output string
		err    error
	}
	sequence := []expectedCommand{
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Stopped")},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "true"}},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "systemctl", "is-system-running"}, err: errors.New("exit status 1")},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Stopped")},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "systemctl", "is-system-running"}, output: "running\n"},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "mountpoint", "-q", "/var"}},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "systemctl", "is-active", "docker.service"}, output: "active\n"},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) (string, error) {
		if callIndex >= len(sequence) {
			t.Fatalf("unexpected extra command: %s %v", name, args)
		}
		expected := sequence[callIndex]
		callIndex++

		if name != expected.name {
			t.Fatalf("command %d name = %q, want %q", callIndex, name, expected.name)
		}
		if !slices.Equal(args, expected.args) {
			t.Fatalf("command %d args = %v, want %v", callIndex, args, expected.args)
		}
		return expected.output, expected.err
	}

	distro, err := manager.ensureMainDistroReady(context.Background(), progressReporter{})
	if err != nil {
		t.Fatalf("ensureMainDistroReady() error = %v", err)
	}
	if distro.State != "Running" {
		t.Fatalf("ensureMainDistroReady() distro state = %q, want %q", distro.State, "Running")
	}
	if callIndex != len(sequence) {
		t.Fatalf("runCommandOutput() calls = %d, want %d", callIndex, len(sequence))
	}
}

func TestEnsureHostStartupWithPowerShellRunsElevatedExecuteWhenCheckNeedsActions(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(&config.Config{
		WSLDistroName:  "discobot",
		WSLStateDir:    root,
		WSLVarDiskPath: filepath.Join(root, "var.vhdx"),
	})

	originalFindWSLStartupScriptPath := findWSLStartupScriptPath
	originalRunWSLStartupPowerShell := runWSLStartupPowerShell
	originalRunElevatedWSLStartupPowerShell := runElevatedWSLStartupPowerShell
	t.Cleanup(func() {
		findWSLStartupScriptPath = originalFindWSLStartupScriptPath
		runWSLStartupPowerShell = originalRunWSLStartupPowerShell
		runElevatedWSLStartupPowerShell = originalRunElevatedWSLStartupPowerShell
	})

	scriptPath := filepath.Join(root, "discobot-wsl-startup.ps1")
	findWSLStartupScriptPath = func() (string, error) {
		return scriptPath, nil
	}

	checks := []wslStartupScriptResult{
		{
			ExitCode: wslStartupExitActionsRequired,
			Message:  "WSL host startup actions require elevation: create-var-disk, attach-var-disk.",
			Actions:  []string{"create-var-disk", "attach-var-disk"},
		},
		{
			ExitCode: wslStartupExitOK,
			Message:  "WSL /var disk is attached.",
		},
	}
	runWSLStartupPowerShell = func(_ context.Context, gotScriptPath string, args ...string) (wslStartupScriptResult, error) {
		if gotScriptPath != scriptPath {
			t.Fatalf("runWSLStartupPowerShell() scriptPath = %q, want %q", gotScriptPath, scriptPath)
		}
		assertStartupScriptModeArg(t, args, "check")
		if len(checks) == 0 {
			t.Fatal("runWSLStartupPowerShell() called too many times")
		}
		result := checks[0]
		checks = checks[1:]
		return result, nil
	}

	elevatedCalled := false
	runElevatedWSLStartupPowerShell = func(_ context.Context, gotScriptPath string, resultPath string, args ...string) (wslStartupScriptResult, error) {
		elevatedCalled = true
		if gotScriptPath != scriptPath {
			t.Fatalf("runElevatedWSLStartupPowerShell() scriptPath = %q, want %q", gotScriptPath, scriptPath)
		}
		if strings.TrimSpace(resultPath) == "" {
			t.Fatal("runElevatedWSLStartupPowerShell() resultPath is empty")
		}
		assertStartupScriptModeArg(t, args, "execute")
		return wslStartupScriptResult{
			ExitCode: wslStartupExitOK,
			Message:  "WSL host startup changes applied.",
		}, nil
	}

	var operations []string
	err := manager.ensureHostStartupWithPowerShell(context.Background(), progressReporter{
		update: func(_ int, currentOperation string) {
			operations = append(operations, currentOperation)
		},
	})
	if err != nil {
		t.Fatalf("ensureHostStartupWithPowerShell() error = %v", err)
	}
	if !elevatedCalled {
		t.Fatal("ensureHostStartupWithPowerShell() did not run elevated execute")
	}
	if len(checks) != 0 {
		t.Fatalf("remaining check results = %d, want 0", len(checks))
	}
	if got := operations[len(operations)-1]; got != "WSL host startup requirements are ready" {
		t.Fatalf("last progress operation = %q, want ready", got)
	}
}

func TestDefaultWSLStartupScriptPathStagesEmbeddedScript(t *testing.T) {
	root := t.TempDir()
	t.Setenv(wslStartupScriptEnv, "")
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "localappdata"))

	scriptPath, err := defaultWSLStartupScriptPath()
	if err != nil {
		t.Fatalf("defaultWSLStartupScriptPath() error = %v", err)
	}
	if filepath.Ext(scriptPath) != ".ps1" {
		t.Fatalf("defaultWSLStartupScriptPath() extension = %q, want .ps1", filepath.Ext(scriptPath))
	}

	staged, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", scriptPath, err)
	}
	if !bytes.Equal(staged, embeddedWSLStartupScript) {
		t.Fatal("staged WSL startup script does not match embedded script")
	}
}

func TestEnsureVMRunningWithProgressRunsElevatedHostStartupWhenRequired(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	if err := os.MkdirAll(fakeBin, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "wsl.exe"), []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "localappdata"))

	manager := NewManager(&config.Config{
		WSLDistroName:  "discobot",
		WSLStateDir:    root,
		WSLInstallDir:  filepath.Join(root, "distro"),
		WSLVarDiskPath: filepath.Join(root, "var.vhdx"),
	})

	originalFindWSLStartupScriptPath := findWSLStartupScriptPath
	originalRunWSLStartupPowerShell := runWSLStartupPowerShell
	originalRunElevatedWSLStartupPowerShell := runElevatedWSLStartupPowerShell
	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		findWSLStartupScriptPath = originalFindWSLStartupScriptPath
		runWSLStartupPowerShell = originalRunWSLStartupPowerShell
		runElevatedWSLStartupPowerShell = originalRunElevatedWSLStartupPowerShell
		runCommandOutput = originalRunCommandOutput
	})

	scriptPath := filepath.Join(root, "discobot-wsl-startup.ps1")
	findWSLStartupScriptPath = func() (string, error) {
		return scriptPath, nil
	}

	checks := []wslStartupScriptResult{
		{
			ExitCode: wslStartupExitActionsRequired,
			Message:  "WSL host startup actions require elevation: attach-var-disk.",
			Actions:  []string{"attach-var-disk", "format-var-disk-if-needed"},
		},
		{
			ExitCode: wslStartupExitOK,
			Message:  "WSL /var disk is attached.",
		},
	}
	runWSLStartupPowerShell = func(_ context.Context, gotScriptPath string, args ...string) (wslStartupScriptResult, error) {
		if gotScriptPath != scriptPath {
			t.Fatalf("runWSLStartupPowerShell() scriptPath = %q, want %q", gotScriptPath, scriptPath)
		}
		assertStartupScriptModeArg(t, args, "check")
		if len(checks) == 0 {
			t.Fatal("runWSLStartupPowerShell() called too many times")
		}
		result := checks[0]
		checks = checks[1:]
		return result, nil
	}

	elevatedCalled := false
	runElevatedWSLStartupPowerShell = func(_ context.Context, gotScriptPath string, resultPath string, args ...string) (wslStartupScriptResult, error) {
		elevatedCalled = true
		if gotScriptPath != scriptPath {
			t.Fatalf("runElevatedWSLStartupPowerShell() scriptPath = %q, want %q", gotScriptPath, scriptPath)
		}
		if strings.TrimSpace(resultPath) == "" {
			t.Fatal("runElevatedWSLStartupPowerShell() resultPath is empty")
		}
		assertStartupScriptModeArg(t, args, "execute")
		return wslStartupScriptResult{
			ExitCode: wslStartupExitOK,
			Message:  "WSL host startup changes applied.",
		}, nil
	}

	type expectedCommand struct {
		name   string
		args   []string
		output string
	}
	sequence := []expectedCommand{
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "systemctl", "is-system-running"}, output: "running\n"},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "mountpoint", "-q", "/var"}},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "systemctl", "is-active", "docker.service"}, output: "active\n"},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
	}
	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) (string, error) {
		if callIndex >= len(sequence) {
			t.Fatalf("unexpected extra command: %s %v", name, args)
		}
		expected := sequence[callIndex]
		callIndex++
		if name != expected.name {
			t.Fatalf("command %d name = %q, want %q", callIndex, name, expected.name)
		}
		if !slices.Equal(args, expected.args) {
			t.Fatalf("command %d args = %v, want %v", callIndex, args, expected.args)
		}
		return expected.output, nil
	}

	if err := manager.ensureVMRunningWithProgress(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("ensureVMRunningWithProgress() error = %v", err)
	}
	if !elevatedCalled {
		t.Fatal("ensureVMRunningWithProgress() did not run elevated host startup")
	}
	if len(checks) != 0 {
		t.Fatalf("remaining check results = %d, want 0", len(checks))
	}
	if callIndex != len(sequence) {
		t.Fatalf("runCommandOutput() calls = %d, want %d", callIndex, len(sequence))
	}
}

func TestEnsureHostStartupWithPowerShellReturnsWellKnownWSLUnavailableError(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(&config.Config{
		WSLDistroName:  "discobot",
		WSLStateDir:    root,
		WSLVarDiskPath: filepath.Join(root, "var.vhdx"),
	})

	originalFindWSLStartupScriptPath := findWSLStartupScriptPath
	originalRunWSLStartupPowerShell := runWSLStartupPowerShell
	t.Cleanup(func() {
		findWSLStartupScriptPath = originalFindWSLStartupScriptPath
		runWSLStartupPowerShell = originalRunWSLStartupPowerShell
	})

	findWSLStartupScriptPath = func() (string, error) {
		return filepath.Join(root, "discobot-wsl-startup.ps1"), nil
	}
	runWSLStartupPowerShell = func(context.Context, string, ...string) (wslStartupScriptResult, error) {
		return wslStartupScriptResult{
			ExitCode: wslStartupExitWSLUnavailable,
			Message:  "wsl_not_installed: WSL is not installed.",
		}, nil
	}

	err := manager.ensureHostStartupWithPowerShell(context.Background(), progressReporter{})
	if err == nil {
		t.Fatal("ensureHostStartupWithPowerShell() error = nil, want WSL unavailable error")
	}
	var startupErr *wslStartupScriptError
	if !errors.As(err, &startupErr) {
		t.Fatalf("ensureHostStartupWithPowerShell() error = %T, want *wslStartupScriptError", err)
	}
	if startupErr.ExitCode != wslStartupExitWSLUnavailable {
		t.Fatalf("startup error exit code = %d, want %d", startupErr.ExitCode, wslStartupExitWSLUnavailable)
	}
	if startupErr.Code != "wsl_not_installed" {
		t.Fatalf("startup error code = %q, want wsl_not_installed", startupErr.Code)
	}
}

func TestReadWSLStartupScriptResultAcceptsUTF8BOM(t *testing.T) {
	t.Parallel()

	resultPath := filepath.Join(t.TempDir(), "result.json")
	raw := append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"mode":"check","exitCode":10,"message":"needs elevation","actions":["import-distro"]}`)...)
	if err := os.WriteFile(resultPath, raw, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := readWSLStartupScriptResultForExitCode(resultPath, []byte("script output"), wslStartupExitActionsRequired)
	if err != nil {
		t.Fatalf("readWSLStartupScriptResultForExitCode() error = %v", err)
	}
	if result.Mode != "check" {
		t.Fatalf("result mode = %q, want check", result.Mode)
	}
	if result.ExitCode != wslStartupExitActionsRequired {
		t.Fatalf("result exit code = %d, want %d", result.ExitCode, wslStartupExitActionsRequired)
	}
	if result.Message != "needs elevation" {
		t.Fatalf("result message = %q, want needs elevation", result.Message)
	}
	if !slices.Equal(result.Actions, []string{"import-distro"}) {
		t.Fatalf("result actions = %v, want import-distro", result.Actions)
	}
	if result.Output != "script output" {
		t.Fatalf("result output = %q, want script output", result.Output)
	}
}

func assertStartupScriptModeArg(t *testing.T, args []string, want string) {
	t.Helper()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-Mode" && args[i+1] == want {
			return
		}
	}
	t.Fatalf("startup script args = %v, want -Mode %s", args, want)
}

func TestVerifyInstalledLockedDoesNotRequireVarDiskBeforeDistroChecks(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   root,
	})

	type expectedCommand struct {
		name   string
		args   []string
		output string
		err    error
	}
	sequence := []expectedCommand{
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) (string, error) {
		if callIndex >= len(sequence) {
			t.Fatalf("unexpected extra command: %s %v", name, args)
		}
		expected := sequence[callIndex]
		callIndex++
		if name != expected.name {
			t.Fatalf("command %d name = %q, want %q", callIndex, name, expected.name)
		}
		if !slices.Equal(args, expected.args) {
			t.Fatalf("command %d args = %v, want %v", callIndex, args, expected.args)
		}
		return expected.output, expected.err
	}

	if err := manager.verifyInstalledLocked(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("verifyInstalledLocked() error = %v", err)
	}
	if callIndex != len(sequence) {
		t.Fatalf("runCommandOutput() calls = %d, want %d", callIndex, len(sequence))
	}
}

func TestHideWindowsTerminalWSLProfilesInSettings(t *testing.T) {
	t.Parallel()

	settings := []byte("{\n" +
		"  \"profiles\": {\n" +
		"    \"list\": [\n" +
		"      {\n" +
		"        \"guid\": \"{one}\",\n" +
		"        \"hidden\": false,\n" +
		"        \"name\": \"Discobot\",\n" +
		"        \"source\": \"Microsoft.WSL\"\n" +
		"      },\n" +
		"      {\n" +
		"        \"guid\": \"{two}\",\n" +
		"        \"name\": \"Discobot\",\n" +
		"        \"source\": \"Windows.Terminal.Wsl\"\n" +
		"      },\n" +
		"      {\n" +
		"        \"guid\": \"{three}\",\n" +
		"        \"hidden\": false,\n" +
		"        \"name\": \"Ubuntu-24.04\",\n" +
		"        \"source\": \"Microsoft.WSL\"\n" +
		"      }\n" +
		"    ]\n" +
		"  }\n" +
		"}\n")

	updated, changed, err := hideWindowsTerminalWSLProfilesInSettings(settings, "Discobot", `C:\Program Files\Discobot\icon.ico`)
	if err != nil {
		t.Fatalf("hideWindowsTerminalWSLProfilesInSettings() error = %v", err)
	}
	if !changed {
		t.Fatal("hideWindowsTerminalWSLProfilesInSettings() changed = false, want true")
	}

	updatedText := string(updated)
	if strings.Count(updatedText, `"name": "Discobot"`) != 2 {
		t.Fatalf("updated settings missing discobot profiles:\n%s", updatedText)
	}
	if strings.Count(updatedText, `"hidden": true`) != 2 {
		t.Fatalf("updated settings hidden=true count = %d, want 2\n%s", strings.Count(updatedText, `"hidden": true`), updatedText)
	}
	if strings.Contains(updatedText, "\"hidden\": false,\n        \"name\": \"Discobot\"") {
		t.Fatalf("updated settings still contain visible discobot profile:\n%s", updatedText)
	}
	if strings.Count(updatedText, `"icon": "C:\\Program Files\\Discobot\\icon.ico"`) != 2 {
		t.Fatalf("updated settings icon count = %d, want 2\n%s", strings.Count(updatedText, `"icon": "C:\\Program Files\\Discobot\\icon.ico"`), updatedText)
	}
	if !strings.Contains(updatedText, "\"hidden\": false,\n        \"name\": \"Ubuntu-24.04\"") {
		t.Fatalf("updated settings changed non-discobot WSL profile unexpectedly:\n%s", updatedText)
	}
}

func TestManagerHideWindowsTerminalWSLProfilesUpdatesExistingSettingsFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)

	settingsPath := filepath.Join(root, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	original := "{\n" +
		"  \"profiles\": {\n" +
		"    \"list\": [\n" +
		"      {\n" +
		"        \"guid\": \"{one}\",\n" +
		"        \"hidden\": false,\n" +
		"        \"name\": \"Discobot\",\n" +
		"        \"source\": \"Microsoft.WSL\"\n" +
		"      }\n" +
		"    ]\n" +
		"  }\n" +
		"}\n"
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := NewManager(&config.Config{
		WSLDistroName:   "Discobot",
		WSLStateDir:     t.TempDir(),
		DesktopIconPath: filepath.Join(root, "Discobot", "icon.ico"),
	})
	if err := manager.hideWindowsTerminalWSLProfiles(); err != nil {
		t.Fatalf("hideWindowsTerminalWSLProfiles() error = %v", err)
	}

	updated, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(updated), `"hidden": true`) {
		t.Fatalf("updated settings missing hidden=true:\n%s", string(updated))
	}
	if !strings.Contains(string(updated), `"icon": `) {
		t.Fatalf("updated settings missing icon:\n%s", string(updated))
	}
}

func TestCustomizeImportRootfsTarReplacesDiscobotWSLEnvFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourceTarPath := filepath.Join(root, "rootfs.tar")
	if err := writeTestTar(sourceTarPath, map[string]string{
		"./etc/default/discobot-wsl": "DISCOBOT_GUEST_PLATFORM='vz'\n",
		"./etc/hostname":             "discobot\n",
	}); err != nil {
		t.Fatalf("writeTestTar() error = %v", err)
	}

	wantEnvFile := "DISCOBOT_GUEST_PLATFORM='wsl'\nDISCOBOT_VAR_DISK_LABEL='discobot-var'\n"
	customizedTarPath, cleanup, err := customizeImportRootfsTar(sourceTarPath, root, wantEnvFile)
	if err != nil {
		t.Fatalf("customizeImportRootfsTar() error = %v", err)
	}
	defer cleanup()

	files, err := readTestTar(customizedTarPath)
	if err != nil {
		t.Fatalf("readTestTar() error = %v", err)
	}

	if got := files["etc/default/discobot-wsl"]; got != wantEnvFile {
		t.Fatalf("customized env file = %q, want %q", got, wantEnvFile)
	}
	if got := files["etc/hostname"]; got != "discobot\n" {
		t.Fatalf("hostname file = %q, want %q", got, "discobot\n")
	}
}

func TestCleanupTempRootfsFileRetriesTransientRemoveFailure(t *testing.T) {
	root := t.TempDir()
	tempPath := filepath.Join(root, "discobot-rootfs-test.tar")
	if err := os.WriteFile(tempPath, []byte("temp"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalRemove := removePath
	originalSleep := sleep
	t.Cleanup(func() {
		removePath = originalRemove
		sleep = originalSleep
	})

	attempts := 0
	removePath = func(path string) error {
		attempts++
		if attempts == 1 {
			return &os.PathError{Op: "remove", Path: path, Err: windows.ERROR_SHARING_VIOLATION}
		}
		return os.Remove(path)
	}
	sleep = func(time.Duration) {}

	if err := cleanupTempRootfsFile(tempPath); err != nil {
		t.Fatalf("cleanupTempRootfsFile() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("remove attempts = %d, want 2", attempts)
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file still exists after cleanup: %v", err)
	}
}

func TestCleanupStaleRootfsTempFilesRemovesOnlyOldMatchingFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	oldRootfsPath := filepath.Join(root, "discobot-rootfs-old.tar")
	oldImportPath := filepath.Join(root, "discobot-rootfs-import-old.tar")
	recentRootfsPath := filepath.Join(root, "discobot-rootfs-recent.tar")
	otherPath := filepath.Join(root, "keep-me.tar")

	for _, filePath := range []string{oldRootfsPath, oldImportPath, recentRootfsPath, otherPath} {
		if err := os.WriteFile(filePath, []byte("temp"), 0644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", filePath, err)
		}
	}

	oldTime := time.Now().Add(-staleRootfsTempFileMaxAge - time.Minute)
	for _, filePath := range []string{oldRootfsPath, oldImportPath, otherPath} {
		if err := os.Chtimes(filePath, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", filePath, err)
		}
	}

	manager := NewManager(&config.Config{WSLStateDir: root})
	if err := manager.cleanupStaleRootfsTempFiles(); err != nil {
		t.Fatalf("cleanupStaleRootfsTempFiles() error = %v", err)
	}

	for _, removedPath := range []string{oldRootfsPath, oldImportPath} {
		if _, err := os.Stat(removedPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale temp file %q still exists: %v", removedPath, err)
		}
	}
	for _, keptPath := range []string{recentRootfsPath, otherPath} {
		if _, err := os.Stat(keptPath); err != nil {
			t.Fatalf("expected file %q to remain, stat error = %v", keptPath, err)
		}
	}
}

func encodeUTF16ForTest(s string, order binary.ByteOrder) []byte {
	words := utf16.Encode([]rune(s))
	encoded := make([]byte, len(words)*2)
	for i, word := range words {
		order.PutUint16(encoded[i*2:], word)
	}
	return encoded
}

func writeTestTar(tarPath string, files map[string]string) error {
	file, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := tar.NewWriter(file)
	defer writer.Close()

	for name, contents := range files {
		if err := writer.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(contents)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			return err
		}
		if _, err := writer.Write([]byte(contents)); err != nil {
			return err
		}
	}

	return nil
}

func readTestTar(tarPath string) (map[string]string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	files := make(map[string]string)
	reader := tar.NewReader(file)
	for {
		hdr, err := reader.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, err
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		files[path.Clean(strings.TrimPrefix(strings.TrimPrefix(hdr.Name, "./"), "/"))] = string(data)
	}
}

func distroListForTest(state string) string {
	return fmt.Sprintf("  NAME                   STATE           VERSION\r\n* Ubuntu-24.04           Running         2\r\n  discobot               %s         2\r\n", state)
}
