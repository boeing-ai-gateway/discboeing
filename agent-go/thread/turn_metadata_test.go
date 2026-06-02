package thread

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
)

func TestRunTurn_PersistsUserMessageMetadata(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread-metadata"

	prov := &mockProvider{
		responses: [][]message.ProviderMessageChunk{
			{
				message.StreamStartChunk{},
				message.TextStartChunk{ID: "t1"},
				message.TextDeltaChunk{ID: "t1", Delta: "Hello!"},
				message.TextEndChunk{ID: "t1"},
				message.FinishChunk{FinishReason: message.FinishReason{Unified: "stop"}},
			},
		},
	}

	meta := json.RawMessage(`{"discobot":{"kind":"hook-failure","hookName":"Go Check","exitCode":2}}`)
	chunks := collectChunks(t, RunTurn(
		context.Background(), prov, &mockExecutor{}, store,
		threadID, "", TurnConfig{
			Model:     "test-model",
			UserParts: []message.Part{message.TextPart{Text: "### Hook failed: Go Check"}},
			Metadata:  meta,
		},
	))

	var userChunk message.UserMessageChunk
	foundUserChunk := false
	for _, chunk := range chunks {
		next, ok := chunk.(message.UserMessageChunk)
		if !ok {
			continue
		}
		userChunk = next
		foundUserChunk = true
		break
	}
	if !foundUserChunk {
		t.Fatal("expected a UserMessageChunk")
	}

	var gotChunkMeta map[string]any
	if err := json.Unmarshal(userChunk.Data.Message.Metadata, &gotChunkMeta); err != nil {
		t.Fatalf("unmarshal user chunk metadata: %v", err)
	}
	var wantMeta map[string]any
	if err := json.Unmarshal(meta, &wantMeta); err != nil {
		t.Fatalf("unmarshal expected metadata: %v", err)
	}
	if gotChunkMeta["originalText"] != nil {
		t.Fatalf("user chunk metadata unexpectedly included originalText: %#v", gotChunkMeta)
	}
	gotDiscobot, ok := gotChunkMeta["discobot"].(map[string]any)
	if !ok {
		t.Fatalf("user chunk metadata missing discobot payload: %#v", gotChunkMeta)
	}
	if gotDiscobot["kind"] != "hook-failure" || gotDiscobot["hookName"] != "Go Check" || gotDiscobot["exitCode"] != float64(2) {
		t.Fatalf("user chunk metadata lost existing discobot fields: %#v", gotChunkMeta)
	}
	if turnID, ok := gotDiscobot["turnId"].(string); !ok || turnID == "" {
		t.Fatalf("user chunk metadata missing turnId: %#v", gotChunkMeta)
	}

	stored, err := store.LoadMessage(threadID, userChunk.Data.Message.ID)
	if err != nil {
		t.Fatalf("LoadMessage() failed: %v", err)
	}
	var gotStoredMeta map[string]any
	if err := json.Unmarshal(stored.Message.Metadata, &gotStoredMeta); err != nil {
		t.Fatalf("unmarshal stored metadata: %v", err)
	}
	if !reflect.DeepEqual(gotStoredMeta, gotChunkMeta) {
		t.Fatalf("stored message metadata = %#v, want %#v", gotStoredMeta, gotChunkMeta)
	}
}

func TestRunTurn_PersistsPreludeMessagesBeforeUserMessage(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread-prelude"

	prov := &mockProvider{
		responses: [][]message.ProviderMessageChunk{
			{
				message.StreamStartChunk{},
				message.TextStartChunk{ID: "t1"},
				message.TextDeltaChunk{ID: "t1", Delta: "Hello!"},
				message.TextEndChunk{ID: "t1"},
				message.FinishChunk{FinishReason: message.FinishReason{Unified: "stop"}},
			},
		},
	}

	prelude := message.Message{
		Role:      "user",
		Synthetic: true,
		Parts: []message.Part{message.TextPart{
			Text: "prelude reminder",
			ProviderMetadata: message.MarshalProviderMetadata(message.DiscobotPartMetadata{
				ReminderKind: "credentials",
			}),
		}},
	}

	chunks := collectChunks(t, RunTurn(
		context.Background(), prov, &mockExecutor{}, store,
		threadID, "", TurnConfig{
			Model:           "test-model",
			PreludeMessages: []message.Message{prelude},
			UserParts:       []message.Part{message.TextPart{Text: "hello"}},
		},
	))

	userChunkCount := 0
	var userChunk message.UserMessageChunk
	for _, chunk := range chunks {
		next, ok := chunk.(message.UserMessageChunk)
		if ok {
			userChunkCount++
			userChunk = next
		}
	}
	if userChunkCount != 1 {
		t.Fatalf("expected exactly 1 user message chunk, got %d", userChunkCount)
	}

	storedUser, err := store.LoadMessage(threadID, userChunk.Data.Message.ID)
	if err != nil {
		t.Fatalf("load stored user message: %v", err)
	}
	if storedUser.ParentID == "" {
		t.Fatal("expected stored user message to have prelude parent")
	}
	storedPrelude, err := store.LoadMessage(threadID, storedUser.ParentID)
	if err != nil {
		t.Fatalf("load stored prelude message: %v", err)
	}

	if !storedPrelude.Message.Synthetic {
		t.Fatal("expected prelude message to be synthetic")
	}
	preludeText, ok := storedPrelude.Message.Parts[0].(message.TextPart)
	if !ok || preludeText.Text != "prelude reminder" {
		t.Fatalf("unexpected prelude message %#v", storedPrelude.Message)
	}
	userText, ok := storedUser.Message.Parts[0].(message.TextPart)
	if !ok || userText.Text != "hello" {
		t.Fatalf("unexpected user message %#v", storedUser.Message)
	}
}

func TestRunTurn_SyntheticUserMessageIsHiddenFromUI(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread-synthetic-user"

	prov := &mockProvider{
		responses: [][]message.ProviderMessageChunk{
			{
				message.StreamStartChunk{},
				message.TextStartChunk{ID: "t1"},
				message.TextDeltaChunk{ID: "t1", Delta: "Continuing."},
				message.TextEndChunk{ID: "t1"},
				message.FinishChunk{FinishReason: message.FinishReason{Unified: "stop"}},
			},
		},
	}

	chunks := collectChunks(t, RunTurn(
		context.Background(), prov, &mockExecutor{}, store,
		threadID, "", TurnConfig{
			Model:         "test-model",
			UserSynthetic: true,
			UserParts: []message.Part{message.TextPart{
				Text: "<system-reminder>\nContinue after startup recovery.\n</system-reminder>",
				ProviderMetadata: message.MarshalProviderMetadata(message.DiscobotPartMetadata{
					ReminderKind: "startup-interruption",
				}),
			}},
		},
	))

	for _, chunk := range chunks {
		if _, ok := chunk.(message.UserMessageChunk); ok {
			t.Fatal("synthetic user message should not be emitted to UI stream")
		}
	}
	if len(prov.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(prov.requests))
	}
	if got := messageText(prov.requests[0].Messages[len(prov.requests[0].Messages)-1]); got != "<system-reminder>\nContinue after startup recovery.\n</system-reminder>" {
		t.Fatalf("provider did not receive synthetic reminder, got %q", got)
	}

	leafID, err := store.FindLeaf(threadID)
	if err != nil {
		t.Fatalf("find leaf: %v", err)
	}
	entries, err := store.BuildHistoryWithIDs(threadID, leafID)
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	messages := entriesToMessages(entries)
	if len(messages) < 2 {
		t.Fatalf("expected synthetic user and assistant messages, got %d", len(messages))
	}
	if !messages[0].Synthetic {
		t.Fatal("expected stored user message to be synthetic")
	}
	uiMessages, err := message.ProjectUIMessages(messages)
	if err != nil {
		t.Fatalf("project UI messages: %v", err)
	}
	if len(uiMessages) != 1 || uiMessages[0].Role != "assistant" {
		t.Fatalf("expected only assistant UI message, got %#v", uiMessages)
	}
}
