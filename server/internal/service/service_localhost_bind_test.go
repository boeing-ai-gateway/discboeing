package service

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

type fakeLocalhostExecStreamer struct{}

func (fakeLocalhostExecStreamer) ExecStream(context.Context, string, []string, sandbox.ExecStreamOptions) (sandbox.Stream, error) {
	return nil, errors.New("unexpected ExecStream call")
}

func TestLocalhostBindManagerBindPortConflict(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve localhost port: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	manager := NewLocalhostBindManager(fakeLocalhostExecStreamer{}, nil)
	defer manager.Close()

	_, err = manager.Bind("session-a", "web", port, 3000, "http")
	if !errors.Is(err, ErrLocalhostPortInUse) {
		t.Fatalf("expected ErrLocalhostPortInUse, got %v", err)
	}
}

func TestLocalhostBindManagerUnbindReleasesPort(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve localhost port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("failed to release reserved port: %v", err)
	}

	manager := NewLocalhostBindManager(fakeLocalhostExecStreamer{}, nil)
	bind, err := manager.Bind("session-a", "web", port, 3000, "https")
	if err != nil {
		t.Fatalf("failed to bind localhost port: %v", err)
	}
	if bind.Port != port || bind.TargetPort != 3000 || bind.URL != "https://localhost:"+strconv.Itoa(port) {
		t.Fatalf("unexpected bind snapshot: %#v", bind)
	}

	manager.Unbind("session-a", "web")

	probe, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("expected unbound port to be reusable: %v", err)
	}
	_ = probe.Close()
}
