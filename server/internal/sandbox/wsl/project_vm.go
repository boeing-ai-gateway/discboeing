//go:build windows

package wsl

import (
	"context"
	"net"
	"strconv"
)

type projectVM struct {
	manager   *DistroManager
	projectID string
}

func (p *projectVM) ProjectID() string {
	return p.projectID
}

func (p *projectVM) DockerDialer() func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		return p.manager.dialDockerBridge(ctx)
	}
}

func (p *projectVM) PortDialer(port uint32) func(context.Context, string, string) (net.Conn, error) {
	return p.tcpPortDialer(port)
}

func (p *projectVM) WorkspaceMountSource(source string) (string, error) {
	return TranslatePath(source)
}

func (p *projectVM) Shutdown() error {
	return nil
}

func (p *projectVM) tcpPortDialer(port uint32) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	}
}
