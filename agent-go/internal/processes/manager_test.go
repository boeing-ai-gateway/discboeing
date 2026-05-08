//go:build !windows

package processes

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestManagerUsesConfiguredDefaultWorkDir(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	mgr := NewManager(workDir)
	session, err := mgr.Start(context.Background(), CreateRequest{
		Cmd: []string{"/bin/pwd"},
	})
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if session.WorkDir != workDir {
		t.Fatalf("session WorkDir = %q, want %q", session.WorkDir, workDir)
	}

	waitForProcessStatus(t, mgr, session.ID, StatusExited)
	events, err := mgr.Output(session.ID)
	if err != nil {
		t.Fatalf("Output() failed: %v", err)
	}
	var output strings.Builder
	for _, event := range events {
		if event.Type == "stdout" {
			output.WriteString(event.Data)
		}
	}
	if got := strings.TrimSpace(output.String()); got != workDir {
		t.Fatalf("pwd output = %q, want %q", got, workDir)
	}
}

func TestManagerEventsSupportSequenceAndFilters(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	mgr := NewManager(workDir)
	session, err := mgr.Start(context.Background(), CreateRequest{
		Cmd: []string{"/bin/sh", "-c", "printf one; printf two >&2"},
	})
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	waitForProcessStatus(t, mgr, session.ID, StatusExited)

	events, err := mgr.Events(session.ID, EventQuery{})
	if err != nil {
		t.Fatalf("Events() failed: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("got %d events, want at least stdout, stderr, and exit: %#v", len(events), events)
	}
	for i, event := range events {
		want := int64(i + 1)
		if event.Seq != want {
			t.Fatalf("event %d seq = %d, want %d; events=%#v", i, event.Seq, want, events)
		}
	}

	after := events[0].Seq
	afterEvents, err := mgr.Events(session.ID, EventQuery{After: &after})
	if err != nil {
		t.Fatalf("Events(after) failed: %v", err)
	}
	if len(afterEvents) != len(events)-1 || afterEvents[0].Seq <= after {
		t.Fatalf("after events = %#v, want events after seq %d", afterEvents, after)
	}

	limited, err := mgr.Events(session.ID, EventQuery{Limit: 1})
	if err != nil {
		t.Fatalf("Events(limit) failed: %v", err)
	}
	if len(limited) != 1 || limited[0].Seq != events[len(events)-1].Seq {
		t.Fatalf("limited events = %#v, want latest event %#v", limited, events[len(events)-1])
	}

	since := events[0].Timestamp.Add(-time.Nanosecond)
	sinceEvents, err := mgr.Events(session.ID, EventQuery{Since: &since})
	if err != nil {
		t.Fatalf("Events(since) failed: %v", err)
	}
	if len(sinceEvents) != len(events) {
		t.Fatalf("since events = %d, want %d", len(sinceEvents), len(events))
	}
}

func TestNewManagerDoesNotCleanupCurrentProcessGroupFromStaleMetadata(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	sessionDir := filepath.Join(homeDir, ".discobot", "processes", "stale")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	session := Session{
		ID:        "stale",
		Status:    StatusRunning,
		PID:       os.Getpid(),
		PGID:      syscall.Getpgrp(),
		StartedAt: time.Now().Add(-time.Hour),
		Cmd:       []string{"/bin/sh"},
	}
	if err := writeJSON(filepath.Join(sessionDir, "session.json"), session); err != nil {
		t.Fatalf("writeJSON() failed: %v", err)
	}

	_ = NewManager(t.TempDir())

	var got Session
	if err := readJSON(filepath.Join(sessionDir, "session.json"), &got); err != nil {
		t.Fatalf("readJSON() failed: %v", err)
	}
	if got.Status != StatusKilled {
		t.Fatalf("status = %q, want %q", got.Status, StatusKilled)
	}
}

func TestNewManagerDoesNotCleanupProcessWithoutSessionMarker(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cmd := exec.Command("/bin/sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	})

	sessionDir := filepath.Join(homeDir, ".discobot", "processes", "stale")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	session := Session{
		ID:        "stale",
		Status:    StatusRunning,
		PID:       cmd.Process.Pid,
		PGID:      cmd.Process.Pid,
		StartedAt: time.Now().Add(-time.Hour),
		Cmd:       []string{"/bin/sleep", "30"},
	}
	if err := writeJSON(filepath.Join(sessionDir, "session.json"), session); err != nil {
		t.Fatalf("writeJSON() failed: %v", err)
	}

	_ = NewManager(t.TempDir())

	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("process was cleaned up without a session marker: %v", err)
	}
}

func waitForProcessStatus(t *testing.T, mgr *Manager, id string, status Status) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			session, _ := mgr.Get(id)
			t.Fatalf("timed out waiting for status %q; session=%+v", status, session)
		case <-tick.C:
			session, err := mgr.Get(id)
			if err != nil {
				t.Fatalf("Get() failed: %v", err)
			}
			if session.Status == status {
				return
			}
		}
	}
}
