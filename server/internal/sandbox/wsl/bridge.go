//go:build windows

package wsl

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

const (
	defaultBridgeReadyTimeout = 10 * time.Second
	defaultBridgePollDelay    = 100 * time.Millisecond
	defaultBridgeDialTimeout  = 250 * time.Millisecond
)

type dockerBridge interface {
	Dial(ctx context.Context) (net.Conn, error)
	Close() error
	Running() bool
}

var startWSLDockerBridge = defaultStartWSLDockerBridge

type wslDockerBridge struct {
	port int
	cmd  *exec.Cmd

	closeOnce sync.Once
	waitDone  chan struct{}
	waitErr   error
}

func defaultStartWSLDockerBridge(ctx context.Context, distro string) (dockerBridge, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	port, err := reserveLocalTCPPort()
	if err != nil {
		return nil, err
	}

	args := []string{
		"-d", distro,
		"--",
		"socat",
		fmt.Sprintf("TCP-LISTEN:%d,bind=127.0.0.1,reuseaddr,fork", port),
		"UNIX-CONNECT:/var/run/docker.sock",
	}
	cmd := exec.Command("wsl.exe", args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("start WSL Docker bridge stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start WSL Docker bridge: %w", err)
	}

	bridge := &wslDockerBridge{
		port:     port,
		cmd:      cmd,
		waitDone: make(chan struct{}),
	}
	go func() {
		_, _ = io.Copy(io.Discard, stderr)
		bridge.waitErr = cmd.Wait()
		close(bridge.waitDone)
	}()

	if err := bridge.waitReady(ctx); err != nil {
		_ = bridge.Close()
		return nil, err
	}
	return bridge, nil
}

func (b *wslDockerBridge) Dial(ctx context.Context) (net.Conn, error) {
	if !b.Running() {
		return nil, fmt.Errorf("WSL Docker bridge is not running")
	}
	var dialer net.Dialer
	return dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(b.port)))
}

func (b *wslDockerBridge) Close() error {
	b.closeOnce.Do(func() {
		if b.cmd.Process != nil {
			_ = b.cmd.Process.Kill()
		}
		<-b.waitDone
	})
	return nil
}

func (b *wslDockerBridge) Running() bool {
	select {
	case <-b.waitDone:
		return false
	default:
		return true
	}
}

func (b *wslDockerBridge) waitReady(ctx context.Context) error {
	readyCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		readyCtx, cancel = context.WithTimeout(ctx, defaultBridgeReadyTimeout)
		defer cancel()
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(b.port))
	for {
		if err := probeTCPBridge(readyCtx, addr); err == nil {
			return nil
		}

		select {
		case <-b.waitDone:
			if b.waitErr != nil {
				return fmt.Errorf("WSL Docker bridge exited before becoming ready: %w", b.waitErr)
			}
			return fmt.Errorf("WSL Docker bridge exited before becoming ready")
		case <-readyCtx.Done():
			return fmt.Errorf("wait for WSL Docker bridge readiness: %w", readyCtx.Err())
		case <-time.After(defaultBridgePollDelay):
		}
	}
}

func probeTCPBridge(ctx context.Context, addr string) error {
	dialCtx, cancel := context.WithTimeout(ctx, defaultBridgeDialTimeout)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return err
	}
	return conn.Close()
}

func reserveLocalTCPPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve local TCP port for WSL Docker bridge: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr.Port == 0 {
		return 0, fmt.Errorf("reserve local TCP port for WSL Docker bridge: invalid address %v", listener.Addr())
	}
	return addr.Port, nil
}
