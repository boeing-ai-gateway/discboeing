//go:build windows

package tools

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsProcessGroupControllerCancelStopsCommand(t *testing.T) {
	skipIfBashUnavailable(t)

	shellPath, err := resolveBashCommand()
	if err != nil {
		t.Fatalf("resolveBashCommand() error = %v", err)
	}

	cmd := exec.Command(shellPath, shellCommandArgsForOS("windows", "Start-Sleep -Seconds 30")...)
	controller := newProcessGroupController()
	controller.configure(cmd)
	defer controller.close()

	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := controller.afterStart(cmd); err != nil {
		t.Fatalf("afterStart() error = %v", err)
	}

	start := time.Now()
	if err := controller.cancel(cmd); err != nil {
		t.Fatalf("cancel() error = %v", err)
	}
	err = cmd.Wait()
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Wait() took too long after cancel: %s", elapsed)
	}
	if err == nil {
		t.Fatal("expected Wait() to report cancellation")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "exit status") {
		t.Fatalf("unexpected Wait() error: %v", err)
	}
}

func TestWindowsProcessGroupControllerAfterStartReturnsCreateJobObjectError(t *testing.T) {
	originalCreateJobObject := createJobObject
	t.Cleanup(func() {
		createJobObject = originalCreateJobObject
	})

	createErr := windows.ERROR_ACCESS_DENIED
	createJobObject = func(*windows.SecurityAttributes, *uint16) (windows.Handle, error) {
		return 0, createErr
	}

	controller := &windowsProcessGroupController{}
	err := controller.afterStart(&exec.Cmd{Process: &os.Process{Pid: 1234}})
	if !errors.Is(err, createErr) {
		t.Fatalf("afterStart() error = %v, want %v", err, createErr)
	}
}
