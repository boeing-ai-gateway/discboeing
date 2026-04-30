package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"

	"github.com/obot-platform/discobot/server/internal/config"
)

// NewAPIClient creates a Docker SDK client from Discobot config.
//
// On Windows, DISCOBOT_DOCKER_WSL_DISTRO can route host Docker access through
// `wsl.exe -d <distro> -- docker system dial-stdio` so local dev builds and
// host-to-VM image transfer can reuse a user-managed WSL Docker daemon.
func NewAPIClient(cfg *config.Config) (*client.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("docker config is required")
	}

	if runtime.GOOS == "windows" && cfg.DockerHost == "" && strings.TrimSpace(cfg.DockerWSLDistro) != "" {
		httpClient, err := newWSLDockerHTTPClient(cfg.DockerWSLDistro)
		if err != nil {
			return nil, err
		}
		return client.NewClientWithOpts(
			client.WithHost("http://localhost"),
			client.WithHTTPClient(httpClient),
			client.WithAPIVersionNegotiation(),
		)
	}

	clientOpts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if cfg.DockerHost != "" {
		clientOpts = append(clientOpts, client.WithHost(cfg.DockerHost))
	} else if host := DetectDockerHost(); host != "" {
		clientOpts = append(clientOpts, client.WithHost(host))
	}
	return client.NewClientWithOpts(clientOpts...)
}

func newWSLDockerHTTPClient(distroName string) (*http.Client, error) {
	distroName = strings.TrimSpace(distroName)
	if distroName == "" {
		return nil, fmt.Errorf("docker WSL distro name is required")
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialWSLDocker(ctx, distroName)
		},
	}
	return &http.Client{Transport: transport}, nil
}

func dialWSLDocker(ctx context.Context, distroName string) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	name, args, err := wslDockerDialCommand(distroName)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("start WSL Docker stdio proxy stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("start WSL Docker stdio proxy stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("start WSL Docker stdio proxy stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start WSL Docker stdio proxy: %w", err)
	}

	conn := &commandConn{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		waitDone: make(chan struct{}),
	}
	go func() {
		_, _ = io.Copy(io.Discard, stderr)
		conn.waitErr = cmd.Wait()
		close(conn.waitDone)
	}()
	if err := ctx.Err(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func wslDockerDialCommand(distroName string) (string, []string, error) {
	distroName = strings.TrimSpace(distroName)
	if distroName == "" {
		return "", nil, fmt.Errorf("docker WSL distro name is required")
	}
	return "wsl.exe", []string{"-d", distroName, "--", "docker", "system", "dial-stdio"}, nil
}

type commandConn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	closeOnce sync.Once
	waitDone  chan struct{}
	waitErr   error
}

func (c *commandConn) Read(p []byte) (int, error) {
	return c.stdout.Read(p)
}

func (c *commandConn) Write(p []byte) (int, error) {
	return c.stdin.Write(p)
}

func (c *commandConn) Close() error {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		_ = c.stdout.Close()
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		<-c.waitDone
	})
	return nil
}

func (c *commandConn) LocalAddr() net.Addr {
	return commandConnAddr("wsl-docker-stdio-local")
}

func (c *commandConn) RemoteAddr() net.Addr {
	return commandConnAddr("wsl-docker-stdio-remote")
}

func (c *commandConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *commandConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *commandConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

type commandConnAddr string

func (a commandConnAddr) Network() string {
	return "wsl-stdio"
}

func (a commandConnAddr) String() string {
	return string(a)
}
