package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/obot-platform/discobot/server/internal/conntrack"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
)

var (
	// ErrInvalidLocalhostPort indicates that a requested host or target port is
	// outside the valid TCP port range.
	ErrInvalidLocalhostPort = errors.New("invalid localhost port")

	// ErrLocalhostPortInUse indicates that the requested host port cannot be
	// bound, usually because another process is already listening on it.
	ErrLocalhostPortInUse = errors.New("localhost port already in use")
)

type localhostBind struct {
	sessionID string
	serviceID string
	listener  net.Listener
	cancel    context.CancelFunc
	snapshot  sandboxapi.ServiceLocalhostBind
}

// LocalhostExecStreamer runs bidirectional streaming commands in a sandbox
// session for localhost service forwarding.
type LocalhostExecStreamer interface {
	ExecStream(ctx context.Context, sessionID string, cmd []string, opts sandbox.ExecStreamOptions) (sandbox.Stream, error)
}

// LocalhostBindManager owns host-side localhost TCP listeners for
// service ports exposed inside sandbox sessions.
type LocalhostBindManager struct {
	execStreamer      LocalhostExecStreamer
	connectionTracker *conntrack.Tracker

	mu    sync.Mutex
	binds map[string]*localhostBind
}

// NewLocalhostBindManager creates a manager that forwards accepted host
// connections into sandbox services via the same streaming mechanism used by SSH
// direct TCP forwarding.
func NewLocalhostBindManager(execStreamer LocalhostExecStreamer, connectionTracker *conntrack.Tracker) *LocalhostBindManager {
	return &LocalhostBindManager{
		execStreamer:      execStreamer,
		connectionTracker: connectionTracker,
		binds:             make(map[string]*localhostBind),
	}
}

// Bind starts or replaces a localhost listener for a service.
func (m *LocalhostBindManager) Bind(sessionID, serviceID string, port, targetPort int, scheme string) (*sandboxapi.ServiceLocalhostBind, error) {
	if m == nil || m.execStreamer == nil {
		return nil, fmt.Errorf("localhost service binding is unavailable")
	}
	if !validTCPPort(port) || !validTCPPort(targetPort) {
		return nil, ErrInvalidLocalhostPort
	}
	if scheme != "https" {
		scheme = "http"
	}
	url := fmt.Sprintf("%s://localhost:%d", scheme, port)

	key := localhostBindKey(sessionID, serviceID)
	m.mu.Lock()
	if existing := m.binds[key]; existing != nil && existing.snapshot.Port == port && existing.snapshot.TargetPort == targetPort && existing.snapshot.URL == url {
		snapshot := existing.snapshot
		m.mu.Unlock()
		return &snapshot, nil
	}
	m.mu.Unlock()

	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("%w: %d", ErrLocalhostPortInUse, port)
	}

	ctx, cancel := context.WithCancel(context.Background())
	bind := &localhostBind{
		sessionID: sessionID,
		serviceID: serviceID,
		listener:  listener,
		cancel:    cancel,
		snapshot: sandboxapi.ServiceLocalhostBind{
			Host:       "127.0.0.1",
			Port:       port,
			TargetPort: targetPort,
			URL:        url,
		},
	}

	var previous *localhostBind
	m.mu.Lock()
	previous = m.binds[key]
	m.binds[key] = bind
	m.mu.Unlock()

	if previous != nil {
		previous.close()
	}

	go m.accept(ctx, bind)

	snapshot := bind.snapshot
	return &snapshot, nil
}

// Unbind closes the localhost listener for a service, if one exists.
func (m *LocalhostBindManager) Unbind(sessionID, serviceID string) {
	if m == nil {
		return
	}

	key := localhostBindKey(sessionID, serviceID)
	m.mu.Lock()
	bind := m.binds[key]
	delete(m.binds, key)
	m.mu.Unlock()

	if bind != nil {
		bind.close()
	}
}

// Close closes all active localhost listeners.
func (m *LocalhostBindManager) Close() {
	if m == nil {
		return
	}

	m.mu.Lock()
	binds := make([]*localhostBind, 0, len(m.binds))
	for key, bind := range m.binds {
		binds = append(binds, bind)
		delete(m.binds, key)
	}
	m.mu.Unlock()

	for _, bind := range binds {
		bind.close()
	}
}

// Get returns the current localhost binding for a service, if present.
func (m *LocalhostBindManager) Get(sessionID, serviceID string) *sandboxapi.ServiceLocalhostBind {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	bind := m.binds[localhostBindKey(sessionID, serviceID)]
	if bind == nil {
		return nil
	}
	snapshot := bind.snapshot
	return &snapshot
}

func (m *LocalhostBindManager) accept(ctx context.Context, bind *localhostBind) {
	for {
		conn, err := bind.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("[ServiceLocalhostBind] accept failed for session %s service %s on port %d: %v", bind.sessionID, bind.serviceID, bind.snapshot.Port, err)
				continue
			}
		}

		go m.forward(ctx, bind, conn)
	}
}

func (m *LocalhostBindManager) forward(ctx context.Context, bind *localhostBind, conn net.Conn) {
	defer conn.Close()

	var release func()
	if m.connectionTracker != nil {
		release = m.connectionTracker.Track(bind.sessionID)
	}
	if release != nil {
		defer release()
	}

	cmd := serviceLocalhostBindCommand(bind.snapshot.TargetPort)
	stream, err := m.execStreamer.ExecStream(ctx, bind.sessionID, cmd, sandbox.ExecStreamOptions{})
	if err != nil {
		log.Printf("[ServiceLocalhostBind] failed to connect session %s service %s target port %d: %v", bind.sessionID, bind.serviceID, bind.snapshot.TargetPort, err)
		return
	}

	outputDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(stream, conn)
		_ = stream.CloseWrite()
	}()
	go func() {
		_, _ = io.Copy(conn, stream)
		close(outputDone)
	}()

	_, _ = stream.Wait(ctx)
	_ = stream.Close()
	_ = conn.Close()
	<-outputDone
}

func (b *localhostBind) close() {
	b.cancel()
	_ = b.listener.Close()
}

func localhostBindKey(sessionID, serviceID string) string {
	return sessionID + "\x00" + serviceID
}

func validTCPPort(port int) bool {
	return port > 0 && port <= 65535
}

func serviceLocalhostBindCommand(port int) []string {
	return []string{"socat", "-", fmt.Sprintf("TCP4:127.0.0.1:%d", port)}
}
