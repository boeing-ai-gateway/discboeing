//go:build windows

package tools

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	createJobObject          = windows.CreateJobObject
	setInformationJobObject  = windows.SetInformationJobObject
	openProcessHandle        = windows.OpenProcess
	assignProcessToJobObject = windows.AssignProcessToJobObject
	closeHandle              = windows.CloseHandle
)

type windowsProcessGroupController struct {
	mu  sync.Mutex
	job windows.Handle
}

func newProcessGroupController() processGroupController {
	return &windowsProcessGroupController{}
}

func (*windowsProcessGroupController) configure(_ *exec.Cmd) {}

// afterStart attaches the newly started process to a Windows job object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE so closing or terminating the job tears
// down the full child process tree.
func (g *windowsProcessGroupController) afterStart(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	job, err := createJobObject(nil, nil)
	if err != nil {
		return err
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = setInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = closeHandle(job)
		return err
	}

	process, err := openProcessHandle(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = closeHandle(job)
		return err
	}
	defer func() {
		_ = closeHandle(process)
	}()

	err = assignProcessToJobObject(job, process)
	if err != nil {
		_ = closeHandle(job)
		return err
	}

	g.mu.Lock()
	g.job = job
	g.mu.Unlock()

	return nil
}

func (g *windowsProcessGroupController) cancel(cmd *exec.Cmd) error {
	g.mu.Lock()
	job := g.job
	g.mu.Unlock()

	if job != 0 {
		return windows.TerminateJobObject(job, 1)
	}

	if cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}

func (g *windowsProcessGroupController) close() error {
	g.mu.Lock()
	job := g.job
	g.job = 0
	g.mu.Unlock()

	if job == 0 {
		return nil
	}

	return closeHandle(job)
}
