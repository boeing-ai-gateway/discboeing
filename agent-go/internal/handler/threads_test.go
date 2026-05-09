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
	h := New("", agent.NewConversationManager(&streamTestAgent{}), nil, nil, defaultAgent)
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

func TestCreateThread_DefaultsCWDToHandlerRoot(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	root := t.TempDir()
	defaultAgent := agentimpl.NewDefaultAgent(store, nil, nil, root, agentimpl.MCPConfig{})
	h := New(root, agent.NewConversationManager(&streamTestAgent{}), nil, nil, defaultAgent)
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
	if got.CWD != root {
		t.Fatalf("expected cwd %q, got %q", root, got.CWD)
	}
}

func TestSessionActivityResponsePrioritizesNeedsAttention(t *testing.T) {
	h := &Handler{}

	got := h.sessionActivityResponse([]api.Thread{
		{
			ID:            "running-thread",
			ActiveCommand: "pnpm test",
		},
		{
			ID:              "blocked-thread",
			PendingQuestion: true,
		},
		{
			ID: "queued-thread",
			PromptQueue: []api.QueuedPrompt{
				{ID: "queued-1"},
			},
		},
	})

	if got.Status != "needs_attention" {
		t.Fatalf("expected needs_attention, got %q", got.Status)
	}
	if got.RepresentativeThreadID != "blocked-thread" {
		t.Fatalf("expected blocked representative, got %q", got.RepresentativeThreadID)
	}
	if got.NeedsAttentionCount != 1 || got.RunningCount != 1 || got.QueuedCount != 1 {
		t.Fatalf("unexpected counts: needs=%d running=%d queued=%d", got.NeedsAttentionCount, got.RunningCount, got.QueuedCount)
	}
	if len(got.Threads) != 3 {
		t.Fatalf("expected 3 sparse states, got %d", len(got.Threads))
	}
}

func TestSessionActivityResponseTreatsTerminalStatesAsNeedsAttention(t *testing.T) {
	h := &Handler{}

	got := h.sessionActivityResponse([]api.Thread{
		{ID: "interrupted-thread", State: "interrupted"},
		{ID: "cancelled-thread", State: "cancelled"},
	})

	if got.Status != "needs_attention" {
		t.Fatalf("expected needs_attention, got %q", got.Status)
	}
	if got.NeedsAttentionCount != 2 {
		t.Fatalf("expected 2 needs_attention threads, got %d", got.NeedsAttentionCount)
	}
	reasons := map[string]string{}
	for _, state := range got.Threads {
		reasons[state.ThreadID] = state.Reason
	}
	if reasons["interrupted-thread"] != "interrupted" {
		t.Fatalf("expected interrupted reason, got %q", reasons["interrupted-thread"])
	}
	if reasons["cancelled-thread"] != "cancelled" {
		t.Fatalf("expected cancelled reason, got %q", reasons["cancelled-thread"])
	}
}
