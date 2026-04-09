package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func newThreadsTestServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/threads", h.CreateThread)
	r.Get("/threads/{id}", h.GetThread)
	return httptest.NewServer(r)
}

func TestCreateThread_DoesNotReturnThreadIDAsNameFallback(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	defaultAgent := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	h := New("", agent.NewCompletionManager(&streamTestAgent{}), nil, nil, defaultAgent)
	ts := newThreadsTestServer(t, h)
	defer ts.Close()

	resp, err := ts.Client().Post(
		ts.URL+"/threads",
		"application/json",
		strings.NewReader(`{"id":"thread-1"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var got api.Thread
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "thread-1" {
		t.Fatalf("expected id %q, got %q", "thread-1", got.ID)
	}
	if got.Name != "" {
		t.Fatalf("expected empty name, got %q", got.Name)
	}
}
