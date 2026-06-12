package handler

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/agent"
)

func TestStreamSessionEmitsAgentOwnedHistory(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(workspace+"/README.md", []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = workspace
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"add", "README.md"},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = workspace
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
		}
	}

	h := New(workspace, agent.NewConversationManager(&streamTestAgent{
		listThreadsFn: func() ([]string, error) {
			return []string{"thread-1"}, nil
		},
	}), nil, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	server := httptest.NewServer(r)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/session/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	reader := bufio.NewReader(resp.Body)
	events := map[string]bool{}
	for {
		event := readSessionStreamTestEvent(t, reader)
		events[event] = true
		if event == "history-end" {
			break
		}
	}

	for _, event := range []string{
		"history-start",
		"threads_updated",
		"files_updated",
		"commands_updated",
		"hooks_updated",
		"services_updated",
		"diff_status_updated",
		"diff_updated",
		"history-end",
	} {
		if !events[event] {
			t.Fatalf("missing event %q in %#v", event, events)
		}
	}
}

func readSessionStreamTestEvent(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "event: "); ok {
			return after
		}
	}
}
