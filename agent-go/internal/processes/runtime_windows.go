//go:build windows

package processes

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"syscall"

	"github.com/charmbracelet/x/conpty"
	"golang.org/x/sys/windows"
)

type platformProcess struct {
	pid  int
	pgid int
}

type platformStream struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	conpty        *conpty.ConPty
	processHandle windows.Handle
}

func startPlatform(_ context.Context, req CreateRequest) (Stream, platformProcess, error) {
	if req.User != "" {
		current, err := user.Current()
		if err != nil || (req.User != current.Username && req.User != current.Uid) {
			return nil, platformProcess{}, ErrUserSwitchUnsupported
		}
	}
	if len(req.Cmd) == 0 {
		return nil, platformProcess{}, errors.New("command is required")
	}
	if req.TTY {
		return startConPTY(req)
	}
	return startPiped(req)
}

func startConPTY(req CreateRequest) (Stream, platformProcess, error) {
	pty, err := conpty.New(req.Cols, req.Rows, 0)
	if err != nil {
		if errors.Is(err, conpty.ErrUnsupported) {
			return nil, platformProcess{}, ErrTTYUnsupported
		}
		return nil, platformProcess{}, err
	}

	env := os.Environ()
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}
	args := append([]string{req.Cmd[0]}, req.Cmd[1:]...)
	pid, handle, err := pty.Spawn(req.Cmd[0], args, &syscall.ProcAttr{
		Dir: req.WorkDir,
		Env: env,
	})
	if err != nil {
		_ = pty.Close()
		return nil, platformProcess{}, err
	}

	return &platformStream{conpty: pty, processHandle: windows.Handle(handle)}, platformProcess{pid: pid}, nil
}

func startPiped(req CreateRequest) (Stream, platformProcess, error) {
	cmd := exec.Command(req.Cmd[0], req.Cmd[1:]...)
	cmd.Dir = req.WorkDir
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stream := &platformStream{cmd: cmd}
	var err error
	stream.stdin, err = cmd.StdinPipe()
	if err != nil {
		return nil, platformProcess{}, err
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, platformProcess{}, err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return nil, platformProcess{}, err
	}
	stream.stdout = stdoutReader
	stream.stderr = stderrReader
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if err := cmd.Start(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		return nil, platformProcess{}, err
	}
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	return stream, platformProcess{pid: cmd.Process.Pid}, nil
}

func (s *platformStream) Read(p []byte) (int, error) {
	if s.conpty != nil {
		return s.conpty.Read(p)
	}
	return s.stdout.Read(p)
}

func (s *platformStream) Write(p []byte) (int, error) {
	if s.conpty != nil {
		return s.conpty.Write(p)
	}
	return s.stdin.Write(p)
}

func (s *platformStream) Stderr() io.Reader {
	if s.conpty != nil {
		return nil
	}
	return s.stderr
}

func (s *platformStream) Resize(_ context.Context, rows, cols int) error {
	if s.conpty == nil {
		return nil
	}
	return s.conpty.Resize(cols, rows)
}

func (s *platformStream) CloseWrite() error {
	if s.conpty != nil {
		return nil
	}
	return s.stdin.Close()
}

func (s *platformStream) Close() error {
	if s.conpty != nil {
		return s.conpty.Close()
	}
	_ = s.stdin.Close()
	_ = s.stdout.Close()
	return s.stderr.Close()
}

func (s *platformStream) Wait(ctx context.Context) (int, error) {
	if s.conpty != nil {
		return s.waitConPTY(ctx)
	}
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case err := <-done:
		return exitCode(s.cmd, err), err
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

func (s *platformStream) waitConPTY(ctx context.Context) (int, error) {
	type waitResult struct {
		code int
		err  error
	}
	done := make(chan waitResult, 1)
	go func() {
		defer func() {
			_ = windows.CloseHandle(s.processHandle)
		}()
		event, err := windows.WaitForSingleObject(s.processHandle, windows.INFINITE)
		if err != nil {
			done <- waitResult{code: -1, err: err}
			return
		}
		if event == windows.WAIT_FAILED {
			done <- waitResult{code: -1, err: syscall.GetLastError()}
			return
		}
		var code uint32
		if err := windows.GetExitCodeProcess(s.processHandle, &code); err != nil {
			done <- waitResult{code: -1, err: err}
			return
		}
		done <- waitResult{code: int(code)}
	}()

	select {
	case result := <-done:
		return result.code, result.err
	case <-ctx.Done():
		return -1, ctx.Err()
	}
}

func exitCode(cmd *exec.Cmd, err error) int {
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}

func killPlatform(stream Stream, pid, _ int) error {
	if pid > 0 {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	}
	if stream != nil {
		return stream.Close()
	}
	return nil
}

func cleanupPlatform(pid, _ int) {
	if pid > 0 {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	}
}

func shouldCleanupAbandoned(s Session) bool {
	return s.PID > 0
}

func platformCapabilities() Capabilities {
	return Capabilities{
		Platform:        runtime.GOOS,
		Runtime:         "conpty",
		TTY:             true,
		Resize:          true,
		ProcessTreeKill: true,
		UserSwitching:   false,
	}
}
