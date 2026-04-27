package sandbox

import (
	"context"
	"log"
	"sync"
	"time"
)

const defaultIdleRuntimeCheckInterval = time.Minute

// IdleRuntimeController exposes the minimum host-runtime lifecycle surface needed
// by IdleRuntimeMonitor.
//
// This is intentionally separate from Provider because session sandboxes and
// host runtimes operate at different scopes:
//   - Provider manages individual session sandboxes
//   - IdleRuntimeController manages a shared host runtime that may serve many
//     sandboxes, such as a project VM or a managed WSL distro
type IdleRuntimeController interface {
	// ListRuntimeIDs returns the identifiers of runtimes that currently exist and
	// should be considered for idle shutdown tracking.
	//
	// Implementations should not create or start new runtimes as a side effect of
	// this method. A stopped or missing runtime should usually be omitted.
	ListRuntimeIDs(ctx context.Context) ([]string, error)

	// RunningSandboxCount returns the number of currently running session
	// sandboxes inside the specified runtime.
	//
	// The runtimeID value will always be one previously returned by
	// ListRuntimeIDs.
	RunningSandboxCount(ctx context.Context, runtimeID string) (int, error)

	// StopRuntime gracefully stops the specified runtime after the monitor has
	// determined that it has remained idle longer than the configured timeout.
	StopRuntime(ctx context.Context, runtimeID string) error
}

// IdleRuntimeMonitor tracks shared host runtimes and stops them once they have
// had no running sandboxes for a configured idle period.
//
// The monitor is generic over any runtime model that can report:
//   - which runtimes currently exist
//   - how many sandboxes are actively running inside each runtime
//   - how to stop an idle runtime
//
// Example runtimes include:
//   - one VM per project
//   - one managed WSL distro shared by all sessions
type IdleRuntimeMonitor struct {
	controller    IdleRuntimeController
	label         string
	idleTimeout   time.Duration
	checkInterval time.Duration

	mu        sync.Mutex
	running   bool
	idleSince map[string]time.Time
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewIdleRuntimeMonitor creates a new runtime-idle monitor.
//
// If checkInterval is zero or negative, a one-minute default is used.
func NewIdleRuntimeMonitor(controller IdleRuntimeController, label string, idleTimeout, checkInterval time.Duration) *IdleRuntimeMonitor {
	if checkInterval <= 0 {
		checkInterval = defaultIdleRuntimeCheckInterval
	}
	return &IdleRuntimeMonitor{
		controller:    controller,
		label:         label,
		idleTimeout:   idleTimeout,
		checkInterval: checkInterval,
		idleSince:     make(map[string]time.Time),
		stopCh:        make(chan struct{}),
	}
}

// Start launches the background idle-runtime monitor loop.
//
// Calling Start more than once is safe; only the first call starts the loop.
func (m *IdleRuntimeMonitor) Start(ctx context.Context) {
	if m == nil || m.controller == nil || m.idleTimeout <= 0 {
		return
	}

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.wg.Add(1)
	go m.loop(ctx)

	log.Printf("Idle runtime monitor started for %s (idle_timeout=%s check_interval=%s)", m.label, m.idleTimeout, m.checkInterval)
}

// Stop terminates the background monitor loop and waits for it to exit.
func (m *IdleRuntimeMonitor) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = false
	close(m.stopCh)
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *IdleRuntimeMonitor) loop(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.check(ctx)
		}
	}
}

func (m *IdleRuntimeMonitor) check(ctx context.Context) {
	runtimeIDs, err := m.controller.ListRuntimeIDs(ctx)
	if err != nil {
		log.Printf("Idle runtime monitor (%s): list runtimes failed: %v", m.label, err)
		return
	}

	now := time.Now()
	active := make(map[string]struct{}, len(runtimeIDs))

	for _, runtimeID := range runtimeIDs {
		active[runtimeID] = struct{}{}

		runningCount, err := m.controller.RunningSandboxCount(ctx, runtimeID)
		if err != nil {
			log.Printf("Idle runtime monitor (%s): count running sandboxes for %s failed: %v", m.label, runtimeID, err)
			continue
		}

		m.mu.Lock()
		if runningCount > 0 {
			delete(m.idleSince, runtimeID)
			m.mu.Unlock()
			continue
		}

		idleStart, exists := m.idleSince[runtimeID]
		if !exists {
			m.idleSince[runtimeID] = now
			m.mu.Unlock()
			continue
		}
		if now.Sub(idleStart) < m.idleTimeout {
			m.mu.Unlock()
			continue
		}
		delete(m.idleSince, runtimeID)
		m.mu.Unlock()

		log.Printf("Idle runtime monitor (%s): stopping runtime %s after %v idle", m.label, runtimeID, now.Sub(idleStart))
		if err := m.controller.StopRuntime(ctx, runtimeID); err != nil {
			log.Printf("Idle runtime monitor (%s): stop runtime %s failed: %v", m.label, runtimeID, err)
		}
	}

	m.mu.Lock()
	for runtimeID := range m.idleSince {
		if _, ok := active[runtimeID]; !ok {
			delete(m.idleSince, runtimeID)
		}
	}
	m.mu.Unlock()
}
