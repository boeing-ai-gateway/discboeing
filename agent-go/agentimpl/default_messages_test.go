package agentimpl

import (
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func TestMessagesIncludesReplacedAssistantMessage(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	threadID := "thread-replacement-history"

	if err := store.SaveMessage(threadID, thread.StoredMessage{
		ID: "user-1",
		Message: message.Message{
			ID:    "user-1",
			Role:  "user",
			Parts: []message.Part{message.TextPart{Text: "hello"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage(threadID, thread.StoredMessage{
		ID:       "assistant-partial",
		ParentID: "user-1",
		Message: message.Message{
			ID:    "assistant-partial",
			Role:  "assistant",
			Parts: []message.Part{message.TextPart{Text: "partial answer"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage(threadID, thread.StoredMessage{
		ID:         "assistant-retry",
		ParentID:   "user-1",
		ReplacesID: "assistant-partial",
		Message: message.Message{
			ID:    "assistant-retry",
			Role:  "assistant",
			Parts: []message.Part{message.TextPart{Text: "complete answer"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	agentImpl := NewDefaultAgent(store, nil, nil, t.TempDir(), MCPConfig{})
	messages, err := agentImpl.Messages(threadID, "assistant-retry")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected user, replaced assistant, and replacement assistant messages, got %#v", messages)
	}
	if messages[1].ID != "assistant-partial" || messages[1].ReplacedByMessageID != "assistant-retry" {
		t.Fatalf("expected partial assistant to be marked replaced, got %#v", messages[1])
	}
	if messages[2].ID != "assistant-retry" || messages[2].ReplacesMessageID != "assistant-partial" {
		t.Fatalf("expected retry assistant to replace partial, got %#v", messages[2])
	}
	if got := uiText(messages[1]); got != "partial answer" {
		t.Fatalf("unexpected partial text %q", got)
	}
	if got := uiText(messages[2]); got != "complete answer" {
		t.Fatalf("unexpected replacement text %q", got)
	}
}

func uiText(msg message.UIMessage) string {
	var text strings.Builder
	for _, part := range msg.Parts {
		if textPart, ok := part.(message.UITextPart); ok {
			text.WriteString(textPart.Text)
		}
	}
	return text.String()
}
