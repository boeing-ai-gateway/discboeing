package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/internal/processes"
)

func TestParseExecEventQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/exec/id/events?limit=25&after=7&since=2026-05-06T17:00:00.123456789Z&follow=true", nil)
	query, follow, err := parseExecEventQuery(req)
	if err != nil {
		t.Fatalf("parseExecEventQuery() failed: %v", err)
	}
	if !follow {
		t.Fatal("follow = false, want true")
	}
	if query.Limit != 25 {
		t.Fatalf("limit = %d, want 25", query.Limit)
	}
	if query.After == nil || *query.After != 7 {
		t.Fatalf("after = %v, want 7", query.After)
	}
	wantSince := time.Date(2026, 5, 6, 17, 0, 0, 123456789, time.UTC)
	if query.Since == nil || !query.Since.Equal(wantSince) {
		t.Fatalf("since = %v, want %v", query.Since, wantSince)
	}
}

func TestParseExecEventQueryUsesLastEventID(t *testing.T) {
	req := httptest.NewRequest("GET", "/exec/id/events?follow=true", nil)
	req.Header.Set("Last-Event-ID", "42")
	query, _, err := parseExecEventQuery(req)
	if err != nil {
		t.Fatalf("parseExecEventQuery() failed: %v", err)
	}
	if query.After == nil || *query.After != 42 {
		t.Fatalf("after = %v, want 42", query.After)
	}
}

func TestAttachExecSeparatesStdoutAndStderr(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	mgr := processes.NewManager(workDir)
	session, err := mgr.Start(context.Background(), processes.CreateRequest{
		Cmd: []string{"/bin/sh", "-c", "printf stdout; printf stderr >&2"},
	})
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	waitForExecStatus(t, mgr, session.ID, processes.StatusExited)

	h := &Handler{processManager: mgr}
	r := chi.NewRouter()
	r.Get("/exec/{id}/attach", h.AttachExec)
	server := httptest.NewServer(r)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/exec/" + session.ID + "/attach"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	frames := map[byte]string{}
	for {
		_, payload, err := conn.Read(ctx)
		if err != nil {
			break
		}
		if len(payload) == 0 {
			continue
		}
		frames[payload[0]] += string(payload[1:])
	}
	if frames[execFrameStdout] != "stdout" {
		t.Fatalf("stdout frame = %q, want %q; frames=%#v", frames[execFrameStdout], "stdout", frames)
	}
	if frames[execFrameStderr] != "stderr" {
		t.Fatalf("stderr frame = %q, want %q; frames=%#v", frames[execFrameStderr], "stderr", frames)
	}
}

func waitForExecStatus(t *testing.T, mgr *processes.Manager, id string, status processes.Status) {
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
