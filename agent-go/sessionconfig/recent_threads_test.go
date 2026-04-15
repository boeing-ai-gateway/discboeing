package sessionconfig

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRuntimeEnvironmentReminder(t *testing.T) {
	got := FormatRuntimeEnvironmentReminder(RuntimeEnvironmentSnapshot{
		CurrentWorkingDirectory: "/tmp/work",
		CurrentModel:            "Claude Sonnet 4",
		CurrentDateTime:         time.Date(2026, time.April, 14, 21, 36, 53, 0, time.UTC),
		GitState:                "branch=main, working_tree=clean",
	})

	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "- Current working directory: /tmp/work") {
		t.Fatalf("expected cwd in reminder, got %q", got)
	}
	if !strings.Contains(got, "- Current model: Claude Sonnet 4") {
		t.Fatalf("expected model in reminder, got %q", got)
	}
	if !strings.Contains(got, "- Current date/time: 2026-04-14T21:36:53Z") {
		t.Fatalf("expected timestamp in reminder, got %q", got)
	}
	if !strings.Contains(got, "branch=main, working_tree=clean") {
		t.Fatalf("expected git state in reminder, got %q", got)
	}
}

func TestFormatRecentThreadsReminder(t *testing.T) {
	got := FormatRecentThreadsReminder("thread-current", "/tmp/read-thread", "/tmp/list-threads", []RecentThreadReference{{
		ThreadID: "thread-1",
		Label:    "Fix the thread bootstrap bug",
	}})

	if !strings.Contains(got, "<system-reminder>") {
		t.Fatalf("expected system reminder, got %q", got)
	}
	if !strings.Contains(got, "Current thread ID: thread-current") {
		t.Fatalf("expected current thread id in reminder, got %q", got)
	}
	if !strings.Contains(got, "Use /tmp/read-thread <thread-id> to print a thread transcript.") {
		t.Fatalf("expected reader command in reminder, got %q", got)
	}
	if !strings.Contains(got, "Use /tmp/list-threads to list available thread IDs and names. It skips the current thread automatically when DISCOBOT_SESSION_ID is set.") {
		t.Fatalf("expected list command in reminder, got %q", got)
	}
	if !strings.Contains(got, "Fix the thread bootstrap bug") {
		t.Fatalf("expected thread label in reminder, got %q", got)
	}
	if !strings.Contains(got, "thread ID: thread-1") {
		t.Fatalf("expected thread id in reminder, got %q", got)
	}
}

func TestFormatRecentThreadsReminder_Empty(t *testing.T) {
	if got := FormatRecentThreadsReminder("thread-current", "/tmp/read-thread", "/tmp/list-threads", nil); got != "" {
		t.Fatalf("expected empty reminder, got %q", got)
	}
}
