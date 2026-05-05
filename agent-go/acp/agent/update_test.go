package agent

import (
	"encoding/json"
	"testing"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func TestApplySessionUpdatePersistsSessionInfo(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	agent := New(nil, "/workspace", store)
	title := "Imported session"
	updatedAt := "2026-04-30T17:00:00Z"

	chunk, ok := agent.sessionManager.applyUpdate("thread-1", protocol.SessionNotification{
		SessionID: "acp-session-1",
		Update: protocol.SessionUpdateSessionInfoUpdate{
			SessionInfoUpdate: protocol.SessionInfoUpdate{
				Meta:      map[string]any{"source": "acp"},
				Title:     &title,
				UpdatedAt: &updatedAt,
			},
		}.SessionUpdate(),
	})
	if !ok {
		t.Fatal("expected thread update chunk")
	}
	threadUpdate, ok := chunk.(message.ThreadUpdateChunk)
	if !ok || threadUpdate.Data.Thread.Name != title {
		t.Fatalf("thread update = %#v, want thread name %q", chunk, title)
	}

	state, ok := agent.sessionManager.state.Get("thread-1")
	if !ok || state.SessionID != "acp-session-1" {
		t.Fatalf("session state = %#v, ok = %v; want updated session id", state, ok)
	}
	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != title || cfg.Metadata.ACPSession.SessionID != "acp-session-1" || cfg.Metadata.ACPSession.Title == nil || *cfg.Metadata.ACPSession.Title != title {
		t.Fatalf("config = %#v, want persisted ACP session info", cfg)
	}
	if got, _ := cfg.Metadata.ACPSession.Meta["source"].(string); got != "acp" {
		t.Fatalf("metadata source = %q, want acp", got)
	}
}

func TestApplySessionUpdateIgnoresMismatchedSessionInfo(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	agent := New(nil, "/workspace", store)
	originalTitle := "Original"
	updatedTitle := "Other"
	if err := agent.sessionManager.saveThreadSession("thread-1", protocol.SessionInfo{
		Cwd:       "/workspace",
		SessionID: "acp-session-1",
		Title:     &originalTitle,
	}, nil); err != nil {
		t.Fatal(err)
	}

	chunk, ok := agent.sessionManager.applyUpdate("thread-1", protocol.SessionNotification{
		SessionID: "other-session",
		Update: protocol.SessionUpdateSessionInfoUpdate{
			SessionInfoUpdate: protocol.SessionInfoUpdate{Title: &updatedTitle},
		}.SessionUpdate(),
	})
	if ok || chunk != nil {
		t.Fatalf("thread update = %#v, %v; want none", chunk, ok)
	}

	session, ok := agent.sessionManager.loadStoredSession("thread-1")
	if !ok || session.Title == nil || *session.Title != originalTitle {
		t.Fatalf("stored session = %#v, want unchanged title %q", session, originalTitle)
	}
	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Metadata.ACPSession.Title == nil || *cfg.Metadata.ACPSession.Title != originalTitle {
		t.Fatalf("persisted title = %#v, want unchanged %q", cfg.Metadata.ACPSession.Title, originalTitle)
	}
}

func TestApplySessionUpdatePersistsConfigOptions(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	agent := New(nil, "/workspace", store)

	option := protocol.NewSessionConfigOptionRaw(json.RawMessage(`{"type":"select","currentValue":"fast","options":[]}`))
	agent.sessionManager.applyUpdate("thread-1", protocol.SessionNotification{
		SessionID: "acp-session-1",
		Update: protocol.SessionUpdateConfigOptionUpdate{
			ConfigOptionUpdate: protocol.ConfigOptionUpdate{
				ConfigOptions: []protocol.SessionConfigOption{option},
			},
		}.SessionUpdate(),
	})

	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Metadata.ACPSession.ConfigOptions) != 1 || string(cfg.Metadata.ACPSession.ConfigOptions[0]) != string(option.Raw()) {
		t.Fatalf("config options = %#v, want raw ACP option", cfg.Metadata.ACPSession.ConfigOptions)
	}
}
