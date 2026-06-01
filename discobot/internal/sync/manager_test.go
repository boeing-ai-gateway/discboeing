package sync

import (
	"encoding/json"
	"testing"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestApplyThreadMessageAppendsHistoryMessages(t *testing.T) {
	cache := testProjectCache()

	changed := applyThreadMessage(&cache, threadMessage{
		sessionID: "session-1",
		threadID:  "thread-1",
		event:     serverapi.HistoryMessage,
		message:   testMessage("message-1", "user", "hello"),
	})

	if !changed {
		t.Fatal("expected history message to change cache")
	}
	messages := cache.Session["session-1"].Thread["thread-1"].Messages
	if len(messages) != 1 || messages[0].ID != "message-1" {
		t.Fatalf("messages = %#v, want message-1", messages)
	}
}

func TestApplyThreadMessageSwapsPendingHistoryOnEnd(t *testing.T) {
	cache := testProjectCache()
	threadData := cache.Session["session-1"].Thread["thread-1"]
	threadData.Messages = []serverapi.Message{testMessage("old-message", "user", "old")}
	cache.Session["session-1"].Thread["thread-1"] = threadData

	if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryStart}); changed {
		t.Fatal("expected history start to defer visible cache change")
	}
	if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryMessage, message: testMessage("new-message", "user", "new")}); changed {
		t.Fatal("expected pending history message to defer visible cache change")
	}
	if got := cache.Session["session-1"].Thread["thread-1"].Messages[0].ID; got != "old-message" {
		t.Fatalf("current message ID = %q, want old-message", got)
	}
	if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryEnd}); !changed {
		t.Fatal("expected history end to publish swapped messages")
	}

	threadData = cache.Session["session-1"].Thread["thread-1"]
	if threadData.PendingHistory {
		t.Fatal("expected pending history to be cleared")
	}
	if len(threadData.PendingMessages) != 0 {
		t.Fatalf("expected pending messages to be cleared, got %d", len(threadData.PendingMessages))
	}
	if len(threadData.Messages) != 1 || threadData.Messages[0].ID != "new-message" || messageText(threadData.Messages[0]) != "new" {
		t.Fatalf("unexpected swapped messages: %#v", threadData.Messages)
	}
}

func TestApplyThreadMessageBuffersChunksDuringHistoryReplay(t *testing.T) {
	cache := testProjectCache()

	if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryStart}); changed {
		t.Fatal("expected history start to defer visible cache change")
	}
	if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryMessage, message: testMessage("history-user", "user", "old message")}); changed {
		t.Fatal("expected history message to defer visible cache change")
	}
	chunks := []serverapi.MessageChunk{
		agentmessage.StartChunk{MessageID: "assistant-1"},
		agentmessage.TextStartChunk{ID: "part-1"},
		agentmessage.TextDeltaChunk{ID: "part-1", Delta: "live reply"},
		agentmessage.TextEndChunk{ID: "part-1"},
	}
	for _, chunk := range chunks {
		if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.Chunk, chunk: chunk}); changed {
			t.Fatal("expected live chunk during history replay to defer visible cache change")
		}
	}
	if got := cache.Session["session-1"].Thread["thread-1"].Messages; len(got) != 0 {
		t.Fatalf("visible messages = %#v, want empty before history end", got)
	}

	if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryEnd}); !changed {
		t.Fatal("expected history end to publish buffered messages")
	}

	messages := cache.Session["session-1"].Thread["thread-1"].Messages
	if len(messages) != 2 || messages[0].ID != "history-user" || messages[1].ID != "assistant-1" {
		t.Fatalf("messages = %#v, want history-user then assistant-1", messages)
	}
	if messageText(messages[1]) != "live reply" {
		t.Fatalf("assistant text = %q, want live reply", messageText(messages[1]))
	}
	part, ok := messages[1].Parts[0].(agentmessage.UITextPart)
	if !ok || part.State != "done" {
		t.Fatalf("assistant part = %#v, want done text", messages[1].Parts[0])
	}
}

func TestApplyThreadMessageClearsStaleMessagesOnEmptyHistoryReplay(t *testing.T) {
	cache := testProjectCache()
	threadData := cache.Session["session-1"].Thread["thread-1"]
	threadData.Messages = []serverapi.Message{testMessage("stale-message", "assistant", "stale")}
	cache.Session["session-1"].Thread["thread-1"] = threadData

	applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryStart})
	if changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryEnd}); !changed {
		t.Fatal("expected empty history end to clear visible messages")
	}

	messages := cache.Session["session-1"].Thread["thread-1"].Messages
	if len(messages) != 0 {
		t.Fatalf("messages = %#v, want empty", messages)
	}
}

func TestApplyThreadMessageReplacesExistingHistoryMessage(t *testing.T) {
	cache := testProjectCache()
	threadData := cache.Session["session-1"].Thread["thread-1"]
	threadData.Messages = []serverapi.Message{testMessage("assistant-1", "assistant", "before")}
	cache.Session["session-1"].Thread["thread-1"] = threadData

	changed := applyThreadMessage(&cache, threadMessage{
		sessionID: "session-1",
		threadID:  "thread-1",
		event:     serverapi.HistoryMessage,
		message:   testMessage("assistant-1", "assistant", "after"),
	})
	if !changed {
		t.Fatal("expected replacement message to change cache")
	}

	messages := cache.Session["session-1"].Thread["thread-1"].Messages
	if len(messages) != 1 || messageText(messages[0]) != "after" {
		t.Fatalf("messages = %#v, want replacement text after", messages)
	}
}

func TestApplyThreadMessageAggregatesLiveTextChunks(t *testing.T) {
	cache := testProjectCache()
	chunks := []serverapi.MessageChunk{
		agentmessage.StartChunk{MessageID: "message-1"},
		agentmessage.TextStartChunk{ID: "text-1"},
		agentmessage.TextDeltaChunk{ID: "text-1", Delta: "hello"},
		agentmessage.TextDeltaChunk{ID: "text-1", Delta: " world"},
		agentmessage.TextEndChunk{ID: "text-1"},
	}
	for _, chunk := range chunks {
		changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.Chunk, chunk: chunk})
		if !changed {
			t.Fatal("expected live chunk to change cache")
		}
	}

	messages := cache.Session["session-1"].Thread["thread-1"].Messages
	if len(messages) != 1 {
		t.Fatalf("expected one aggregated assistant message, got %d", len(messages))
	}
	if messages[0].ID != "message-1" || messageText(messages[0]) != "hello world" {
		t.Fatalf("unexpected aggregated message: %#v", messages[0])
	}
	part := messages[0].Parts[0].(agentmessage.UITextPart)
	if part.State != "done" {
		t.Fatalf("text part state = %q, want done", part.State)
	}
}

func TestApplyThreadMessageIgnoresUnknownThread(t *testing.T) {
	cache := state.ProjectData{Session: map[string]state.SessionData{}}
	changed := applyThreadMessage(&cache, threadMessage{sessionID: "session-1", threadID: "thread-1", event: serverapi.HistoryMessage, message: testMessage("message-1", "user", "hello")})
	if changed {
		t.Fatal("expected unknown thread message to be ignored")
	}
}

func TestCloneThreadMessageStateOnlyClonesTargetMessages(t *testing.T) {
	cache := state.ProjectData{
		Session: map[string]state.SessionData{
			"session-1": {
				Session: serverapi.Session{ID: "session-1"},
				Thread: map[string]state.ThreadData{
					"thread-1": {
						Thread:          serverapi.Thread{ID: "thread-1"},
						Messages:        []serverapi.Message{testMessage("message-1", "assistant", "visible")},
						PendingMessages: []serverapi.Message{testMessage("pending-1", "assistant", "pending")},
					},
					"thread-2": {
						Thread:   serverapi.Thread{ID: "thread-2"},
						Messages: []serverapi.Message{testMessage("message-2", "assistant", "sibling")},
					},
				},
			},
		},
	}

	cloned, ok := cloneThreadMessageState(cache, "session-1", "thread-1")
	if !ok {
		t.Fatal("expected thread message state to clone")
	}

	target := cloned.Session["session-1"].Thread["thread-1"]
	target.Messages[0].Parts[0] = agentmessage.UITextPart{Type: "text", Text: "changed", State: "done"}
	target.PendingMessages[0].Parts[0] = agentmessage.UITextPart{Type: "text", Text: "changed pending", State: "done"}

	originalTarget := cache.Session["session-1"].Thread["thread-1"]
	if got := messageText(originalTarget.Messages[0]); got != "visible" {
		t.Fatalf("original visible message text = %q, want visible", got)
	}
	if got := messageText(originalTarget.PendingMessages[0]); got != "pending" {
		t.Fatalf("original pending message text = %q, want pending", got)
	}

	originalSibling := cache.Session["session-1"].Thread["thread-2"]
	clonedSibling := cloned.Session["session-1"].Thread["thread-2"]
	if &clonedSibling.Messages[0] != &originalSibling.Messages[0] {
		t.Fatal("expected sibling thread messages to keep their backing slice")
	}
}

func TestSessionSandboxRunning(t *testing.T) {
	tests := map[string]bool{
		"ready":        true,
		"running":      true,
		"runtime":      true,
		"stopped":      false,
		"error":        false,
		"initializing": false,
		"":             false,
	}
	for status, want := range tests {
		got := sessionSandboxRunning(serverapi.Session{SandboxStatus: status})
		if got != want {
			t.Fatalf("sessionSandboxRunning(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestClearSessionThreads(t *testing.T) {
	cache := state.ProjectData{
		Session: map[string]state.SessionData{
			"session-1": {
				Session: serverapi.Session{ID: "session-1", SandboxStatus: "stopped"},
				Threads: []serverapi.Thread{{ID: "thread-1"}},
				Thread: map[string]state.ThreadData{
					"thread-1": {Thread: serverapi.Thread{ID: "thread-1"}, Messages: []serverapi.Message{testMessage("message-1", "user", "hello")}},
				},
			},
		},
	}

	clearSessionThreads(&cache, "session-1")

	sessionData := cache.Session["session-1"]
	if sessionData.Threads != nil {
		t.Fatalf("Threads = %#v, want nil", sessionData.Threads)
	}
	if sessionData.Thread != nil {
		t.Fatalf("Thread = %#v, want nil", sessionData.Thread)
	}
	if sessionData.Session.ID != "session-1" {
		t.Fatalf("Session ID = %q, want session-1", sessionData.Session.ID)
	}
}

func TestMessageChunkJSONUsesAgentGoDiscriminator(t *testing.T) {
	data, err := serverapi.MarshalMessageChunk(agentmessage.ToolInputAvailableChunk{
		ToolCallID: "tool-1",
		ToolName:   "Read",
		Input:      json.RawMessage(`{"path":"README.md"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := serverapi.UnmarshalMessageChunk(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := chunk.(agentmessage.ToolInputAvailableChunk); !ok {
		t.Fatalf("chunk = %T, want ToolInputAvailableChunk", chunk)
	}
}

func testProjectCache() state.ProjectData {
	return state.ProjectData{
		Session: map[string]state.SessionData{
			"session-1": {
				Session: serverapi.Session{ID: "session-1"},
				Thread: map[string]state.ThreadData{
					"thread-1": {Thread: serverapi.Thread{ID: "thread-1"}},
				},
			},
		},
	}
}

func testMessage(id, role, text string) serverapi.Message {
	return serverapi.Message{
		ID:   id,
		Role: role,
		Parts: []agentmessage.UIPart{
			agentmessage.UITextPart{Type: "text", Text: text, State: "done"},
		},
	}
}
