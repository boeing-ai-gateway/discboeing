//go:build !windows

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

	"github.com/creack/pty"
)

type platformProcess struct {
	pid  int
	pgid int
}

type platformStream struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	pty    *os.File
}

func startPlatform(_ context.Context, req CreateRequest) (Stream, platformProcess, error) {
	if len(req.Cmd) == 0 {
		return nil, platformProcess{}, errors.New("command is required")
	}
	cmd := exec.Command(req.Cmd[0], req.Cmd[1:]...)
	cmd.Dir = req.WorkDir
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	if err := configureUser(cmd, req.User); err != nil {
		return nil, platformProcess{}, err
	}

	stream := &platformStream{cmd: cmd}
	if req.TTY {
		ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(req.Rows), Cols: uint16(req.Cols)})
		if err != nil {
			return nil, platformProcess{}, err
		}
		stream.pty = ptmx
		return stream, platformProcess{pid: cmd.Process.Pid, pgid: cmd.Process.Pid}, nil
	}

	var err error
	stream.stdin, err = cmd.StdinPipe()
	if err != nil {
		return nil, platformProcess{}, err
	}
	stream.stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, platformProcess{}, err
	}
	stream.stderr, err = cmd.StderrPipe()
	if err != nil {
		return nil, platformProcess{}, err
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		return nil, platformProcess{}, err
	}
	return stream, platformProcess{pid: cmd.Process.Pid, pgid: cmd.Process.Pid}, nil
}

func configureUser(cmd *exec.Cmd, username string) error {
	if username == "" {
		return nil
	}
	current, err := user.Current()
	if err == nil && (username == current.Username || username == current.Uid) {
		return nil
	}
	target, err := user.Lookup(username)
	if err != nil {
		return err
	}
	uid, err := strconv.ParseUint(target.Uid, 10, 32)
	if err != nil {
		return err
	}
	gid, err := strconv.ParseUint(target.Gid, 10, 32)
	if err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return ErrUserSwitchUnsupported
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}
	return nil
}

func (s *platformStream) Read(p []byte) (int, error) {
	if s.pty != nil {
		return s.pty.Read(p)
	}
	return s.stdout.Read(p)
}

func (s *platformStream) Write(p []byte) (int, error) {
	if s.pty != nil {
		return s.pty.Write(p)
	}
	return s.stdin.Write(p)
}

func (s *platformStream) Stderr() io.Reader {
	if s.pty != nil {
		return nil
	}
	return s.stderr
}

func (s *platformStream) Resize(_ context.Context, rows, cols int) error {
	if s.pty == nil {
		return nil
	}
	return pty.Setsize(s.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *platformStream) CloseWrite() error {
	if s.pty != nil {
		return nil
	}
	return s.stdin.Close()
}

func (s *platformStream) Close() error {
	if s.pty != nil {
		return s.pty.Close()
	}
	_ = s.stdin.Close()
	_ = s.stdout.Close()
	return s.stderr.Close()
}

func (s *platformStream) Wait(ctx context.Context) (int, error) {
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case err := <-done:
		return exitCode(s.cmd, err), err
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

func killPlatform(stream Stream, pid, pgid int) error {
	if pgid > 0 {
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err == nil {
			return nil
		}
	}
	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	if stream != nil {
		return stream.Close()
	}
	return nil
}

func cleanupPlatform(pid, pgid int) {
	if pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func platformCapabilities() Capabilities {
	return Capabilities{
		Platform:        runtime.GOOS,
		Runtime:         "process-group",
		TTY:             true,
		Resize:          true,
		ProcessTreeKill: true,
		UserSwitching:   os.Geteuid() == 0,
	}
}
