//go:build windows

package browser

import (
	"os/exec"
)

func configureBrowserCommand(_ *exec.Cmd) {}

func killBrowserCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
