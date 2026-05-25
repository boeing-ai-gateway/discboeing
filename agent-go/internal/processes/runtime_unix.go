//go:build !windows

package processes

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

const sudoPath = "/usr/bin/sudo"

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
	cmdArgs, err := commandForUser(req.Cmd, req.User)
	if err != nil {
		return nil, platformProcess{}, err
	}
	if len(cmdArgs) == 0 {
		return nil, platformProcess{}, errors.New("command is required")
	}
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = req.WorkDir
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
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
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		return nil, platformProcess{}, err
	}
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	return stream, platformProcess{pid: cmd.Process.Pid, pgid: cmd.Process.Pid}, nil
}

func commandForUser(cmd []string, targetUser string) ([]string, error) {
	if len(cmd) == 0 {
		return nil, nil
	}
	targetUser = strings.TrimSpace(targetUser)
	if targetUser == "" || isCurrentUserTarget(targetUser) {
		return cmd, nil
	}
	return sudoCommandForUser(targetUser, cmd), nil
}

func sudoCommandForUser(targetUser string, cmd []string) []string {
	args := []string{sudoPath, "-E", "-n"}
	userPart, groupPart, hasGroup := strings.Cut(targetUser, ":")
	args = append(args, "-u", sudoUserArg(userPart))
	if hasGroup && strings.TrimSpace(groupPart) != "" {
		args = append(args, "-g", sudoUserArg(groupPart))
	}
	args = append(args, "--")
	args = append(args, cmd...)
	return args
}

func sudoUserArg(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return value
		}
	}
	return "#" + value
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

func shouldCleanupAbandoned(s Session) bool {
	if s.PID <= 0 {
		return false
	}
	if s.PGID > 0 {
		pgid, err := syscall.Getpgid(s.PID)
		if err != nil || pgid != s.PGID {
			return false
		}
		if pgid == syscall.Getpgrp() {
			return false
		}
	}
	if len(s.Cmd) == 0 || s.Cmd[0] == "" || runtime.GOOS != "linux" {
		return true
	}
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(s.PID), "cmdline"))
	if err != nil || len(data) == 0 {
		return false
	}
	fields := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	if len(fields) == 0 || fields[0] == "" {
		return false
	}
	if filepath.Base(fields[0]) != filepath.Base(s.Cmd[0]) {
		return false
	}
	data, err = os.ReadFile(filepath.Join("/proc", strconv.Itoa(s.PID), "environ"))
	if err != nil || len(data) == 0 {
		return false
	}
	return slices.Contains(strings.Split(strings.TrimRight(string(data), "\x00"), "\x00"), processSessionEnv+"="+s.ID)
}

func platformCapabilities() Capabilities {
	return Capabilities{
		Platform:        runtime.GOOS,
		Runtime:         "process-group",
		TTY:             true,
		Resize:          true,
		ProcessTreeKill: true,
		UserSwitching:   os.Geteuid() == 0 || isExecutableFile(sudoPath),
	}
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}
