//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

type unixProcessGroupController struct{}

func newProcessGroupController() processGroupController {
	return &unixProcessGroupController{}
}

// configure runs the command in its own process group so cancellation can
// signal the whole tree instead of only the shell process.
func (*unixProcessGroupController) configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func (*unixProcessGroupController) afterStart(_ *exec.Cmd) error {
	return nil
}

func (*unixProcessGroupController) cancel(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func (*unixProcessGroupController) close() error {
	return nil
}
