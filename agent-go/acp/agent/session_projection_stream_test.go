package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	"github.com/obot-platform/discobot/agent-go/message"
)

func TestSessionProjectionKeepsTextBlockOpen(t *testing.T) {
	projection := newSessionProjection()

	chunks, err := projection.push(textUpdate(t, protocol.SessionUpdateAgentMessageChunkSessionUpdate, "message-1", "hello "))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks,
		message.TextStartChunk{ID: "message-1"},
		message.TextDeltaChunk{ID: "message-1", Delta: "hello "},
	)

	chunks, err = projection.push(textUpdate(t, protocol.SessionUpdateAgentMessageChunkSessionUpdate, "message-1", "world"))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks, message.TextDeltaChunk{ID: "message-1", Delta: "world"})

	assertChunks(t, projection.closeAll(), message.TextEndChunk{ID: "message-1"})
}

func TestSessionProjectionClosesMissingBlocksInOrder(t *testing.T) {
	projection := newSessionProjection()

	if _, err := projection.push(textUpdate(t, protocol.SessionUpdateAgentMessageChunkSessionUpdate, "message-1", "hello")); err != nil {
		t.Fatal(err)
	}
	chunks, err := projection.push(textUpdate(t, protocol.SessionUpdateAgentThoughtChunkSessionUpdate, "thought-1", "thinking"))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks,
		message.TextEndChunk{ID: "message-1"},
		message.ReasoningStartChunk{ID: "thought-1"},
		message.ReasoningDeltaChunk{ID: "thought-1", Delta: "thinking"},
	)

	chunks, err = projection.push(textUpdate(t, protocol.SessionUpdateAgentMessageChunkSessionUpdate, "message-2", "answer"))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks,
		message.ReasoningEndChunk{ID: "thought-1"},
		message.TextStartChunk{ID: "message-2"},
		message.TextDeltaChunk{ID: "message-2", Delta: "answer"},
	)

	assertChunks(t, projection.closeAll(), message.TextEndChunk{ID: "message-2"})
}

func TestSessionProjectionRestartsWhenKindChangesForSameID(t *testing.T) {
	projection := newSessionProjection()

	if _, err := projection.push(textUpdate(t, protocol.SessionUpdateAgentMessageChunkSessionUpdate, "shared", "answer")); err != nil {
		t.Fatal(err)
	}
	chunks, err := projection.push(textUpdate(t, protocol.SessionUpdateAgentThoughtChunkSessionUpdate, "shared", "reason"))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks,
		message.TextEndChunk{ID: "shared"},
		message.ReasoningStartChunk{ID: "shared"},
		message.ReasoningDeltaChunk{ID: "shared", Delta: "reason"},
	)
}

func TestSessionProjectionClosesTextBeforeFileAndSource(t *testing.T) {
	projection := newSessionProjection()

	if _, err := projection.push(textUpdate(t, protocol.SessionUpdateAgentMessageChunkSessionUpdate, "message-1", "hello")); err != nil {
		t.Fatal(err)
	}
	chunks, err := projection.push(update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
		"content": map[string]any{
			"type":     protocol.ContentBlockImageType,
			"mimeType": "image/png",
			"data":     "aW1hZ2U=",
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks,
		message.TextEndChunk{ID: "message-1"},
		message.FileChunk{MediaType: "image/png", Data: "aW1hZ2U="},
	)

	chunks, err = projection.push(update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
		"content": map[string]any{
			"type":     protocol.ContentBlockResourceLinkType,
			"uri":      "file:///workspace/main.go",
			"mimeType": "text/x-go",
			"name":     "main.go",
			"title":    "Main source",
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks, message.SourceChunk{
		SourceType: "url",
		SourceID:   "file:///workspace/main.go",
		URL:        "file:///workspace/main.go",
		MediaType:  "text/x-go",
		Title:      "Main source",
		Filename:   "main.go",
	})
}

func TestSessionProjectionToolCallAndResult(t *testing.T) {
	projection := newSessionProjection()

	if _, err := projection.push(textUpdate(t, protocol.SessionUpdateAgentMessageChunkSessionUpdate, "message-1", "before tool")); err != nil {
		t.Fatal(err)
	}
	chunks, err := projection.push(update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateToolCallSessionUpdate,
		"toolCallId":    "call-1",
		"title":         "Read main.go",
		"kind":          "read",
		"rawInput":      map[string]any{"path": "main.go"},
		"status":        "in_progress",
	}))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks,
		message.TextEndChunk{ID: "message-1"},
		message.ToolCallChunk{
			ToolCallID:       "call-1",
			ToolName:         "read",
			Input:            `{"path":"main.go"}`,
			ProviderExecuted: new(true),
			Dynamic:          new(true),
		},
	)

	chunks, err = projection.push(update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateToolCallUpdateSessionUpdate,
		"toolCallId":    "call-1",
		"status":        "completed",
		"rawOutput":     map[string]any{"text": "done"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks, message.ToolResultChunk{
		ToolCallID: "call-1",
		ToolName:   "read",
		Result:     json.RawMessage(`{"text":"done"}`),
		IsError:    new(false),
		Dynamic:    new(true),
	})
}

func TestSessionProjectionToolUpdateFallsBackToID(t *testing.T) {
	projection := newSessionProjection()

	chunks, err := projection.push(update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateToolCallUpdateSessionUpdate,
		"toolCallId":    "call-unknown",
		"status":        "failed",
		"content": []map[string]any{{
			"type": protocol.ContentBlockTextType,
			"text": "no previous tool_call",
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunk count = %d, want 1", len(chunks))
	}
	result, ok := chunks[0].(message.ToolResultChunk)
	if !ok {
		t.Fatalf("chunk type = %T, want message.ToolResultChunk", chunks[0])
	}
	if result.ToolName != "call-unknown" || result.IsError == nil || !*result.IsError {
		t.Fatalf("tool result = %#v, want failed fallback named by id", result)
	}
	if !strings.Contains(string(result.Result), "no previous tool_call") {
		t.Fatalf("result = %s, want content text", result.Result)
	}
}

func TestSessionProjectionFallsBackForMalformedUpdates(t *testing.T) {
	projection := newSessionProjection()

	chunks, err := projection.push(protocol.NewSessionUpdateRaw(json.RawMessage(`{`)))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks)

	chunks, err = projection.push(protocol.NewSessionUpdateRaw(mustRawJSON(map[string]any{
		"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
		"content":       "not an object",
	})))
	if err != nil {
		t.Fatal(err)
	}
	assertChunks(t, chunks)
}

func textUpdate(t *testing.T, updateType, id, text string) protocol.SessionUpdate {
	t.Helper()
	return update(t, map[string]any{
		"sessionUpdate": updateType,
		"content": map[string]any{
			"type": protocol.ContentBlockTextType,
			"id":   id,
			"text": text,
		},
	})
}

func update(t *testing.T, value any) protocol.SessionUpdate {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return protocol.NewSessionUpdateRaw(data)
}

func assertChunks(t *testing.T, got []message.MessageChunk, want ...message.MessageChunk) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("chunk count = %d, want %d\ngot: %#v", len(got), len(want), got)
	}
	for i := range want {
		assertChunk(t, i, got[i], want[i])
	}
}

func assertChunk(t *testing.T, index int, got, want message.MessageChunk) {
	t.Helper()
	switch want := want.(type) {
	case message.TextStartChunk:
		got, ok := got.(message.TextStartChunk)
		if !ok || got.ID != want.ID {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.TextDeltaChunk:
		got, ok := got.(message.TextDeltaChunk)
		if !ok || got.ID != want.ID || got.Delta != want.Delta {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.TextEndChunk:
		got, ok := got.(message.TextEndChunk)
		if !ok || got.ID != want.ID {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.ReasoningStartChunk:
		got, ok := got.(message.ReasoningStartChunk)
		if !ok || got.ID != want.ID {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.ReasoningDeltaChunk:
		got, ok := got.(message.ReasoningDeltaChunk)
		if !ok || got.ID != want.ID || got.Delta != want.Delta {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.ReasoningEndChunk:
		got, ok := got.(message.ReasoningEndChunk)
		if !ok || got.ID != want.ID {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.FileChunk:
		got, ok := got.(message.FileChunk)
		if !ok || got.MediaType != want.MediaType || got.Data != want.Data {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.SourceChunk:
		got, ok := got.(message.SourceChunk)
		if !ok || got.SourceType != want.SourceType || got.SourceID != want.SourceID || got.URL != want.URL || got.MediaType != want.MediaType || got.Title != want.Title || got.Filename != want.Filename {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.ToolCallChunk:
		got, ok := got.(message.ToolCallChunk)
		if !ok || got.ToolCallID != want.ToolCallID || got.ToolName != want.ToolName || got.Input != want.Input || got.ProviderExecuted == nil || !*got.ProviderExecuted || got.Dynamic == nil || !*got.Dynamic {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	case message.ToolResultChunk:
		got, ok := got.(message.ToolResultChunk)
		if !ok || got.ToolCallID != want.ToolCallID || got.ToolName != want.ToolName || string(got.Result) != string(want.Result) || got.IsError == nil || want.IsError == nil || *got.IsError != *want.IsError || got.Dynamic == nil || !*got.Dynamic {
			t.Fatalf("chunk[%d] = %#v, want %#v", index, got, want)
		}
	default:
		t.Fatalf("chunk[%d] has unhandled expected type %T", index, want)
	}
}
