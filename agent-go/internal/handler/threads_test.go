package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
	r.Get("/threads/activity/stream", h.StreamSessionActivity)
	return httptest.NewServer(r)
}

type activityStreamThreadManager struct {
	mu    sync.Mutex
	infos []agent.ThreadInfo
}

func (m *activityStreamThreadManager) ListThreadInfos() ([]agent.ThreadInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]agent.ThreadInfo(nil), m.infos...), nil
}

func (m *activityStreamThreadManager) GetThreadInfo(string) (agent.ThreadInfo, error) {
	return agent.ThreadInfo{}, nil
}

func (m *activityStreamThreadManager) CreateThread(context.Context, agent.CreateThreadRequest) (agent.ThreadInfo, error) {
	return agent.ThreadInfo{}, nil
}

func (m *activityStreamThreadManager) UpdateThread(context.Context, string, agent.UpdateThreadRequest) (agent.ThreadInfo, error) {
	return agent.ThreadInfo{}, nil
}

func (m *activityStreamThreadManager) DeleteThread(context.Context, string) error {
	return nil
}

func (m *activityStreamThreadManager) setInfos(infos []agent.ThreadInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infos = append([]agent.ThreadInfo(nil), infos...)
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

func TestStreamSessionActivityEmitsInitialAndChangedSnapshots(t *testing.T) {
	manager := &activityStreamThreadManager{}
	h := &Handler{
		threadManager: manager,
		activity:      newActivityNotifier(),
		chatPingEvery: time.Hour,
		activityEvery: time.Hour,
	}
	ts := newThreadsTestServer(t, h)
	defer ts.Close()

	ctx := t.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/threads/activity/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	initial := readActivityStreamSnapshot(t, reader)
	if initial.Status != "idle" {
		t.Fatalf("expected initial idle snapshot, got %q", initial.Status)
	}

	manager.setInfos([]agent.ThreadInfo{{
		ID:            "thread-1",
		ActiveCommand: "pnpm test",
	}})
	h.notifyActivityChanged()

	changed := readActivityStreamSnapshot(t, reader)
	if changed.Status != "running" {
		t.Fatalf("expected changed running snapshot, got %q", changed.Status)
	}
	if changed.RunningCount != 1 || changed.RepresentativeThreadID != "thread-1" {
		t.Fatalf("unexpected changed snapshot: %+v", changed)
	}
}

func TestStreamSessionActivityPeriodicallyEmitsSnapshots(t *testing.T) {
	manager := &activityStreamThreadManager{}
	h := &Handler{
		threadManager: manager,
		activity:      newActivityNotifier(),
		chatPingEvery: time.Hour,
		activityEvery: 25 * time.Millisecond,
	}
	ts := newThreadsTestServer(t, h)
	defer ts.Close()

	ctx := t.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/threads/activity/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	initial := readActivityStreamSnapshot(t, reader)
	if initial.Status != "idle" {
		t.Fatalf("expected initial idle snapshot, got %q", initial.Status)
	}

	manager.setInfos([]agent.ThreadInfo{{
		ID:            "thread-1",
		ActiveCommand: "pnpm test",
	}})

	var periodic api.SessionActivityResponse
	for range 10 {
		periodic = readActivityStreamSnapshot(t, reader)
		if periodic.Status == "running" {
			break
		}
	}
	if periodic.Status != "running" {
		t.Fatalf("expected periodic running snapshot, got %q", periodic.Status)
	}
	if periodic.RunningCount != 1 || periodic.RepresentativeThreadID != "thread-1" {
		t.Fatalf("unexpected periodic snapshot: %+v", periodic)
	}
}

func readActivityStreamSnapshot(t *testing.T, reader *bufio.Reader) api.SessionActivityResponse {
	t.Helper()

	type result struct {
		snapshot api.SessionActivityResponse
		err      error
	}
	resultCh := make(chan result, 1)
	go func() {
		for {
			eventName := ""
			data := ""
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					resultCh <- result{err: err}
					return
				}
				line = strings.TrimRight(line, "\r\n")
				if line == "" {
					break
				}
				if field, value, ok := strings.Cut(line, ":"); ok {
					value = strings.TrimPrefix(value, " ")
					switch field {
					case "event":
						eventName = value
					case "data":
						if data == "" {
							data = value
						} else {
							data += "\n" + value
						}
					}
				}
			}
			if eventName != "activity" {
				continue
			}
			var snapshot api.SessionActivityResponse
			if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
				resultCh <- result{err: err}
				return
			}
			resultCh <- result{snapshot: snapshot}
			return
		}
	}()

	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.snapshot
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for activity stream snapshot")
	}
	return api.SessionActivityResponse{}
}
