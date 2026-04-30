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
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/klauspost/compress/zstd"

	"github.com/obot-platform/discobot/server/internal/config"
)

func TestPrepareInstallDirForImportMovesStaleInstallDirAside(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	installDir := filepath.Join(root, "distro")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	markerPath := filepath.Join(installDir, "marker.txt")
	if err := os.WriteFile(markerPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := &Manager{cfg: &config.Config{
		WSLDistroName: "discobot",
		WSLInstallDir: installDir,
	}}
	if err := manager.prepareInstallDirForImport(); err != nil {
		t.Fatalf("prepareInstallDirForImport() error = %v", err)
	}

	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("install dir still exists after prepareInstallDirForImport(): %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ReadDir() entries = %d, want 1", len(entries))
	}
	if !strings.HasPrefix(entries[0].Name(), "distro.stale-") {
		t.Fatalf("backup dir = %q, want prefix %q", entries[0].Name(), "distro.stale-")
	}
	if _, err := os.Stat(filepath.Join(root, entries[0].Name(), "marker.txt")); err != nil {
		t.Fatalf("backup marker missing: %v", err)
	}
}

func TestPrepareInstallDirForImportRetriesTransientRenameFailure(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "distro")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	markerPath := filepath.Join(installDir, "marker.txt")
	if err := os.WriteFile(markerPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	originalRename := renamePath
	originalSleep := sleep
	t.Cleanup(func() {
		renamePath = originalRename
		sleep = originalSleep
	})

	attempts := 0
	renamePath = func(oldpath, newpath string) error {
		attempts++
		if attempts == 1 {
			return &os.PathError{Op: "rename", Path: oldpath, Err: syscall.ERROR_ACCESS_DENIED}
		}
		return os.Rename(oldpath, newpath)
	}
	sleep = func(time.Duration) {}

	manager := &Manager{cfg: &config.Config{
		WSLDistroName: "discobot",
		WSLInstallDir: installDir,
	}}
	if err := manager.prepareInstallDirForImport(); err != nil {
		t.Fatalf("prepareInstallDirForImport() error = %v", err)
	}

	if attempts != 2 {
		t.Fatalf("rename attempts = %d, want 2", attempts)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("install dir still exists after prepareInstallDirForImport(): %v", err)
	}
}

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

	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if !slices.Equal(args, []string{"-d", "discobot", "--", "systemctl", "is-system-running"}) {
			t.Fatalf("unexpected command args: %v", args)
		}
		return []byte("degraded\n"), errors.New("exit status 1")
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
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if !slices.Equal(args, []string{"--list", "--verbose"}) {
			t.Fatalf("unexpected command args: %v", args)
		}
		callIndex++
		if callIndex == 1 {
			return []byte(distroListForTest("Installing")), nil
		}
		return []byte(distroListForTest("Stopped")), nil
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

func TestStatusReportsInstallingDistroAsStarting(t *testing.T) {
	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
		WSLBridgeType: BridgeTypeTCP,
		WSLBridgePort: 23755,
	})

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if !slices.Equal(args, []string{"--list", "--verbose"}) {
			t.Fatalf("unexpected command args: %v", args)
		}
		return []byte(distroListForTest("Installing")), nil
	}

	status := manager.Status()
	if status.State != "starting" {
		t.Fatalf("Status().State = %q, want %q", status.State, "starting")
	}
	if !strings.Contains(status.Message, "import is still being finalized") {
		t.Fatalf("Status().Message = %q, want importing message", status.Message)
	}
}

func TestStartTCPBridgeUsesSystemdRun(t *testing.T) {
	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
	})

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		wantPrefix := []string{"-d", "discobot", "--", "sh", "-lc"}
		if len(args) != len(wantPrefix)+1 || !slices.Equal(args[:len(wantPrefix)], wantPrefix) {
			t.Fatalf("unexpected command args: %v", args)
		}
		command := args[len(args)-1]
		if !strings.Contains(command, "systemd-run --unit=discobot-docker-bridge") {
			t.Fatalf("startTCPBridge() command = %q, want systemd-run", command)
		}
		if !strings.Contains(command, "exec socat TCP-LISTEN:23755,bind=0.0.0.0,reuseaddr,fork UNIX-CONNECT:/var/run/docker.sock") {
			t.Fatalf("startTCPBridge() command = %q, want socat command", command)
		}
		return nil, nil
	}

	if err := manager.startTCPBridge(context.Background(), 23755); err != nil {
		t.Fatalf("startTCPBridge() error = %v", err)
	}
}

func TestProbeBridgeReadyUsesHTTPForTCPBridge(t *testing.T) {
	manager := NewManager(&config.Config{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_ping" {
			t.Fatalf("request path = %q, want %q", r.URL.Path, "/_ping")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	portText := server.URL[strings.LastIndex(server.URL, ":")+1:]
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", portText, err)
	}

	ready, err := manager.probeBridgeReady(context.Background(), BridgeInfo{
		Type:       BridgeTypeTCP,
		DockerHost: fmt.Sprintf("tcp://127.0.0.1:%d", port),
		Port:       port,
	})
	if err != nil {
		t.Fatalf("probeBridgeReady() error = %v", err)
	}
	if !ready {
		t.Fatal("probeBridgeReady() = false, want true")
	}
}

func TestWaitForTCPBridgeReadyFailsWhenDistroStops(t *testing.T) {
	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
	})

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if !slices.Equal(args, []string{"--list", "--verbose"}) {
			t.Fatalf("unexpected command args: %v", args)
		}
		return []byte(distroListForTest("Stopped")), nil
	}

	err := manager.waitForTCPBridgeReady(context.Background(), BridgeInfo{
		Type:       BridgeTypeTCP,
		DockerHost: "tcp://127.0.0.1:9",
		Port:       9,
	})
	if err == nil {
		t.Fatal("waitForTCPBridgeReady() error = nil, want stopped distro error")
	}
	if !strings.Contains(err.Error(), "managed WSL distro \"discobot\" stopped while waiting for WSL Docker bridge readiness") {
		t.Fatalf("waitForTCPBridgeReady() error = %v", err)
	}
}

func TestShouldRecoverBrokenDistroRecognizesUnexpectedStop(t *testing.T) {
	if !shouldRecoverBrokenDistro(errors.New("managed WSL distro \"discobot\" stopped while waiting for WSL Docker bridge readiness")) {
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

	if got := manager.varDiskLabel(); got != "discobot-data-var" {
		t.Fatalf("varDiskLabel() = %q, want %q", got, "discobot-data-var")
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
	if !strings.Contains(got, "DISCOBOT_VAR_DISK_LABEL='discobot-data-var'") {
		t.Fatalf("buildDiscobotWSLEnvFile() missing quoted disk label: %q", got)
	}
}

func TestEnsureVarDiskFileUsesElevatedHelperWhenNewVHDNeedsPrivileges(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(&config.Config{
		WSLDistroName:  "discobot",
		WSLStateDir:    root,
		WSLVarDiskPath: filepath.Join(root, "var.vhdx"),
	})

	originalRunCommandOutput := runCommandOutput
	originalRunElevatedWSLHelper := runElevatedWSLHelper
	originalFindWSLElevationHelperPath := findWSLElevationHelperPath
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
		runElevatedWSLHelper = originalRunElevatedWSLHelper
		findWSLElevationHelperPath = originalFindWSLElevationHelperPath
	})

	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "powershell.exe" {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		return []byte("New-VHD : You do not have the required permission to complete this task."), errors.New("exit status 1")
	}

	helperCalled := false
	findWSLElevationHelperPath = func() (string, error) {
		return filepath.Join(root, "discobot-wsl-helper.exe"), nil
	}
	runElevatedWSLHelper = func(_ context.Context, helperPath string, args ...string) (string, error) {
		helperCalled = true
		if helperPath == "" {
			t.Fatal("runElevatedWSLHelper() helperPath = empty")
		}
		wantArgs := []string{"create-vhd", "--path", filepath.Join(root, "var.vhdx"), "--size-gb", "100"}
		if !slices.Equal(args, wantArgs) {
			t.Fatalf("runElevatedWSLHelper() args = %v, want %v", args, wantArgs)
		}
		return "", nil
	}

	err := manager.ensureVarDiskFile(context.Background())
	if err != nil {
		t.Fatalf("ensureVarDiskFile() error = %v", err)
	}
	if !helperCalled {
		t.Fatal("ensureVarDiskFile() did not invoke elevated helper")
	}
}

func TestAttachVarDiskBareUsesElevatedHelperWhenMountNeedsPrivileges(t *testing.T) {
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

	originalRunCommandOutput := runCommandOutput
	originalRunElevatedWSLHelper := runElevatedWSLHelper
	originalFindWSLElevationHelperPath := findWSLElevationHelperPath
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
		runElevatedWSLHelper = originalRunElevatedWSLHelper
		findWSLElevationHelperPath = originalFindWSLElevationHelperPath
	})

	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		return []byte("Access is denied."), errors.New("exit status 1")
	}

	helperCalled := false
	findWSLElevationHelperPath = func() (string, error) {
		return filepath.Join(root, "discobot-wsl-helper.exe"), nil
	}
	runElevatedWSLHelper = func(_ context.Context, helperPath string, args ...string) (string, error) {
		helperCalled = true
		if helperPath == "" {
			t.Fatal("runElevatedWSLHelper() helperPath = empty")
		}
		wantArgs := []string{"mount-vhd-bare", "--path", varDiskPath}
		if !slices.Equal(args, wantArgs) {
			t.Fatalf("runElevatedWSLHelper() args = %v, want %v", args, wantArgs)
		}
		return "", nil
	}

	err := manager.attachVarDiskBare(context.Background())
	if err == nil {
		if !helperCalled {
			t.Fatal("attachVarDiskBare() did not invoke elevated helper")
		}
		return
	}
	t.Fatalf("attachVarDiskBare() error = %v", err)
}

func TestIsUnformattedVarDiskMountErrorMatchesInvalidArgumentMountFailure(t *testing.T) {
	t.Parallel()

	err := errors.New("wsl.exe --mount --vhd C:\\var.vhdx --name discobot-var --type ext4: exit status 1: The disk was attached but failed to mount: Invalid argument.")
	if !isUnformattedVarDiskMountError(err) {
		t.Fatalf("isUnformattedVarDiskMountError(%q) = false, want true", err)
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
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "wsl.exe" {
			t.Fatalf("unexpected command name: %s", name)
		}
		if slices.Equal(args, []string{"-d", "discobot", "--", "mountpoint", "-q", "/var"}) {
			mountpointCalls++
			return nil, nil
		}
		t.Fatalf("unexpected command args: %v", args)
		return nil, nil
	}

	err := manager.waitForVarReadyInDistro(context.Background(), "discobot")
	if err != nil {
		t.Fatalf("waitForVarReadyInDistro() error = %v", err)
	}
	if mountpointCalls != 1 {
		t.Fatalf("waitForVarReadyInDistro() mountpoint calls = %d, want 1", mountpointCalls)
	}
}

func TestEnsureMainDistroReadyRecoversBrokenRuntimeDistro(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	installDir := filepath.Join(root, "discobot")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "marker.txt"), []byte("stale"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rootfsArchivePath := filepath.Join(root, "discobot-rootfs.tar.zst")
	if err := writeTestRootfsArchive(rootfsArchivePath, map[string]string{
		"./etc/hostname": "discobot\n",
	}); err != nil {
		t.Fatalf("writeTestRootfsArchive() error = %v", err)
	}

	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLInstallDir: installDir,
		WSLStateDir:   stateDir,
		WSLRootfsPath: rootfsArchivePath,
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
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
		{name: "wsl.exe", args: []string{"--terminate", "discobot"}},
		{name: "wsl.exe", args: []string{"--unregister", "discobot"}},
		{name: "wsl.exe", args: []string{"--import", "discobot", installDir, "", "--version", "2"}},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Stopped")},
		{name: "wsl.exe", args: []string{"-d", "discobot", "--", "true"}},
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
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if callIndex >= len(sequence) {
			t.Fatalf("unexpected extra command: %s %v", name, args)
		}
		expected := sequence[callIndex]
		callIndex++

		if name != expected.name {
			t.Fatalf("command %d name = %q, want %q", callIndex, name, expected.name)
		}
		if len(args) != len(expected.args) {
			t.Fatalf("command %d args length = %d, want %d (%v)", callIndex, len(args), len(expected.args), args)
		}
		for i := range args {
			if expected.args[i] == "" && i == 3 && len(expected.args) > 4 && expected.args[0] == "--import" {
				if filepath.Dir(args[i]) != stateDir {
					t.Fatalf("command %d import tar path parent = %q, want %q", callIndex, filepath.Dir(args[i]), stateDir)
				}
				continue
			}
			if args[i] != expected.args[i] {
				t.Fatalf("command %d arg %d = %q, want %q", callIndex, i, args[i], expected.args[i])
			}
		}
		if len(args) > 0 && args[0] == "--import" {
			if err := os.MkdirAll(installDir, 0755); err != nil {
				t.Fatalf("MkdirAll() during import mock error = %v", err)
			}
		}
		return []byte(expected.output), expected.err
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
	if _, err := os.Stat(installDir); err != nil {
		t.Fatalf("reimported install dir missing after recovery: %v", err)
	}
}

func TestEnsureVarDiskAttachedFormatsNewDiskInSystemWSL(t *testing.T) {
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
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\n"},
		{name: "wsl.exe", args: []string{"--mount", "--vhd", varDiskPath, "--bare"}},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\nsdb disk\n"},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "value", "-s", "LABEL", "/dev/sdb"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "mkfs.ext4", "-F", "-L", "discobot-var", "/dev/sdb"}},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "value", "-s", "LABEL", "/dev/sdb"}, output: "discobot-var\n"},
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if callIndex >= len(sequence) {
			t.Fatalf("unexpected extra command: %s %v", name, args)
		}
		expected := sequence[callIndex]
		callIndex++

		if name != expected.name {
			t.Fatalf("command %d name = %q, want %q", callIndex, name, expected.name)
		}
		if len(args) != len(expected.args) {
			t.Fatalf("command %d args length = %d, want %d (%v)", callIndex, len(args), len(expected.args), args)
		}
		for i := range args {
			if args[i] != expected.args[i] {
				t.Fatalf("command %d arg %d = %q, want %q", callIndex, i, args[i], expected.args[i])
			}
		}
		return []byte(expected.output), expected.err
	}

	if err := manager.ensureVarDiskAttached(context.Background()); err != nil {
		t.Fatalf("ensureVarDiskAttached() error = %v", err)
	}
	if callIndex != len(sequence) {
		t.Fatalf("runCommandOutput() calls = %d, want %d", callIndex, len(sequence))
	}
}

func TestEnsureVarDiskAttachedFailsWhenDeviceNeverAppears(t *testing.T) {
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
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\n"},
		{name: "wsl.exe", args: []string{"--mount", "--vhd", varDiskPath, "--bare"}},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\n"},
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
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
		return []byte(expected.output), expected.err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := manager.ensureVarDiskAttached(ctx)
	if err == nil {
		t.Fatal("ensureVarDiskAttached() error = nil, want hard failure")
	}
	if !strings.Contains(err.Error(), "wait for attached WSL /var disk") {
		t.Fatalf("ensureVarDiskAttached() error = %v, want attached disk failure", err)
	}
	if callIndex != len(sequence) {
		t.Fatalf("runCommandOutput() calls = %d, want %d", callIndex, len(sequence))
	}
}

func TestEnsureVarDiskAttachedRecoversAlreadyAttachedState(t *testing.T) {
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
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\n"},
		{name: "wsl.exe", args: []string{"--mount", "--vhd", varDiskPath, "--bare"}, output: "That volume is already mounted inside WSL2.\nError code: Wsl/Service/DetachDisk/WSL_E_DISK_ALREADY_MOUNTED\n", err: errors.New("exit status 1")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--unmount", varDiskPath}, output: "The disk failed to detach: Invalid argument.\n", err: errors.New("exit status 1")},
		{name: "wsl.exe", args: []string{"--shutdown"}},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\n"},
		{name: "wsl.exe", args: []string{"--mount", "--vhd", varDiskPath, "--bare"}},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\nsdb disk\n"},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "value", "-s", "LABEL", "/dev/sdb"}, output: "discobot-var\n"},
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
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
		return []byte(expected.output), expected.err
	}

	if err := manager.ensureVarDiskAttached(context.Background()); err != nil {
		t.Fatalf("ensureVarDiskAttached() error = %v", err)
	}
	if callIndex != len(sequence) {
		t.Fatalf("runCommandOutput() calls = %d, want %d", callIndex, len(sequence))
	}
}

func TestEnsureVarDiskAttachedSkipsAttachWhenAlreadyVisibleByLabel(t *testing.T) {
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

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
		callIndex++
		if name != "wsl.exe" {
			t.Fatalf("command %d name = %q, want %q", callIndex, name, "wsl.exe")
		}
		wantArgs := []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}
		if !slices.Equal(args, wantArgs) {
			t.Fatalf("command %d args = %v, want %v", callIndex, args, wantArgs)
		}
		return []byte("/dev/sdg\n"), nil
	}

	if err := manager.ensureVarDiskAttached(context.Background()); err != nil {
		t.Fatalf("ensureVarDiskAttached() error = %v", err)
	}
	if callIndex != 1 {
		t.Fatalf("runCommandOutput() calls = %d, want 1", callIndex)
	}
}

func TestEnsureInstalledLockedAttachesVarDiskBeforeDistroChecks(t *testing.T) {
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
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\n"},
		{name: "wsl.exe", args: []string{"--mount", "--vhd", varDiskPath, "--bare"}},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "device", "-t", "LABEL=discobot-var"}, err: errors.New("exit status 2")},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "lsblk", "-dn", "-o", "NAME,TYPE"}, output: "sda disk\nsdb disk\n"},
		{name: "wsl.exe", args: []string{"--system", "-u", "root", "--", "blkid", "-o", "value", "-s", "LABEL", "/dev/sdb"}, output: "discobot-var\n"},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
		{name: "wsl.exe", args: []string{"--list", "--verbose"}, output: distroListForTest("Running")},
	}

	originalRunCommandOutput := runCommandOutput
	t.Cleanup(func() {
		runCommandOutput = originalRunCommandOutput
	})

	callIndex := 0
	runCommandOutput = func(_ context.Context, name string, args ...string) ([]byte, error) {
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
		return []byte(expected.output), expected.err
	}

	if err := manager.ensureInstalledLocked(context.Background(), progressReporter{}); err != nil {
		t.Fatalf("ensureInstalledLocked() error = %v", err)
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
		"        \"name\": \"discobot\",\n" +
		"        \"source\": \"Microsoft.WSL\"\n" +
		"      },\n" +
		"      {\n" +
		"        \"guid\": \"{two}\",\n" +
		"        \"name\": \"discobot\",\n" +
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

	updated, changed, err := hideWindowsTerminalWSLProfilesInSettings(settings, "discobot")
	if err != nil {
		t.Fatalf("hideWindowsTerminalWSLProfilesInSettings() error = %v", err)
	}
	if !changed {
		t.Fatal("hideWindowsTerminalWSLProfilesInSettings() changed = false, want true")
	}

	updatedText := string(updated)
	if strings.Count(updatedText, `"name": "discobot"`) != 2 {
		t.Fatalf("updated settings missing discobot profiles:\n%s", updatedText)
	}
	if strings.Count(updatedText, `"hidden": true`) != 2 {
		t.Fatalf("updated settings hidden=true count = %d, want 2\n%s", strings.Count(updatedText, `"hidden": true`), updatedText)
	}
	if strings.Contains(updatedText, "\"hidden\": false,\n        \"name\": \"discobot\"") {
		t.Fatalf("updated settings still contain visible discobot profile:\n%s", updatedText)
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
		"        \"name\": \"discobot\",\n" +
		"        \"source\": \"Microsoft.WSL\"\n" +
		"      }\n" +
		"    ]\n" +
		"  }\n" +
		"}\n"
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := NewManager(&config.Config{
		WSLDistroName: "discobot",
		WSLStateDir:   t.TempDir(),
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

func writeTestRootfsArchive(archivePath string, files map[string]string) error {
	var tarBuffer bytes.Buffer
	tarWriter := tar.NewWriter(&tarBuffer)
	for name, contents := range files {
		if err := tarWriter.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(contents)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			return err
		}
		if _, err := tarWriter.Write([]byte(contents)); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}

	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder, err := zstd.NewWriter(file)
	if err != nil {
		return err
	}
	if _, err := encoder.Write(tarBuffer.Bytes()); err != nil {
		encoder.Close()
		return err
	}
	return encoder.Close()
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
