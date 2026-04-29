package tools

import "os/exec"

// processGroupController manages a command and any child processes it spawns
// as a single cancelable unit.
type processGroupController interface {
	configure(*exec.Cmd)
	afterStart(*exec.Cmd) error
	cancel(*exec.Cmd) error
	close() error
}

