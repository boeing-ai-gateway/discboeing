package agent

import (
	"encoding/json"
	"testing"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	"github.com/obot-platform/discobot/agent-go/message"
)

func TestSessionProjectionGroupsRolesAndContentParts(t *testing.T) {
	projection := newSessionProjection()

	pushProjection(t, projection, update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateUserMessageChunkSessionUpdate,
		"content": map[string]any{
			"type": protocol.ContentBlockTextType,
			"_meta": map[string]any{
				"id": "user-meta-id",
			},
			"text": "imported prompt",
		},
	}))
	pushProjection(t, projection, update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateAgentThoughtChunkSessionUpdate,
		"content": map[string]any{
			"type": protocol.ContentBlockTextType,
			"id":   "thought-id",
			"text": "thinking",
		},
	}))
	pushProjection(t, projection, update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateAgentMessageChunkSessionUpdate,
		"content": map[string]any{
			"type": protocol.ContentBlockResourceLinkType,
			"uri":  "file:///workspace/main.go",
			"name": "main.go",
		},
	}))

	messages := projection.messages()
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2: %#v", len(messages), messages)
	}
	if messages[0].Role != "user" || len(messages[0].Parts) != 1 {
		t.Fatalf("user message = %#v, want one part", messages[0])
	}
	text, ok := messages[0].Parts[0].(message.TextPart)
	if !ok || text.ID != "user-meta-id" || text.Text != "imported prompt" || text.State != "done" {
		t.Fatalf("user text part = %#v, want projected text", messages[0].Parts[0])
	}
	if messages[1].Role != "assistant" || len(messages[1].Parts) != 2 {
		t.Fatalf("assistant message = %#v, want two parts", messages[1])
	}
	reasoning, ok := messages[1].Parts[0].(message.ReasoningPart)
	if !ok || reasoning.ID != "thought-id" || reasoning.Text != "thinking" || reasoning.State != "done" {
		t.Fatalf("assistant reasoning part = %#v, want projected reasoning", messages[1].Parts[0])
	}
	source, ok := messages[1].Parts[1].(message.SourceURLPart)
	if !ok || source.SourceID != "file:///workspace/main.go" || source.URL != "file:///workspace/main.go" || source.Title != "main.go" {
		t.Fatalf("assistant source part = %#v, want projected source", messages[1].Parts[1])
	}
}

func TestSessionProjectionProjectsToolCallsAndUpdates(t *testing.T) {
	projection := newSessionProjection()

	pushProjection(t, projection, update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateToolCallSessionUpdate,
		"toolCallId":    "call-1",
		"title":         "Read main.go",
		"kind":          "read",
		"rawInput":      map[string]any{"path": "main.go"},
		"status":        "in_progress",
	}))
	pushProjection(t, projection, update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateToolCallUpdateSessionUpdate,
		"toolCallId":    "call-1",
		"status":        "completed",
		"rawOutput":     "done",
	}))
	pushProjection(t, projection, update(t, map[string]any{
		"sessionUpdate": protocol.SessionUpdateToolCallUpdateSessionUpdate,
		"toolCallId":    "call-2",
		"status":        "failed",
		"content": []map[string]any{{
			"type": protocol.ToolCallContentContentType,
			"content": map[string]any{
				"type": protocol.ContentBlockTextType,
				"text": "tool failed",
			},
		}},
	}))

	messages := projection.messages()
	if len(messages) != 1 || messages[0].Role != "assistant" || len(messages[0].Parts) != 3 {
		t.Fatalf("projected messages = %#v, want one assistant message with three parts", messages)
	}
	call, ok := messages[0].Parts[0].(message.ToolCallPart)
	if !ok || call.ToolCallID != "call-1" || call.ToolName != "read" || call.Input != `{"path":"main.go"}` || call.ProviderExecuted == nil || !*call.ProviderExecuted {
		t.Fatalf("tool call part = %#v, want read call", messages[0].Parts[0])
	}
	result, ok := messages[0].Parts[1].(message.ToolResultPart)
	if !ok || result.ToolCallID != "call-1" || result.ToolName != "read" {
		t.Fatalf("tool result part = %#v, want read result", messages[0].Parts[1])
	}
	textOutput, ok := result.Output.(message.TextOutput)
	if !ok || textOutput.Value != "done" {
		t.Fatalf("tool result output = %#v, want text done", result.Output)
	}
	failed, ok := messages[0].Parts[2].(message.ToolResultPart)
	if !ok || failed.ToolCallID != "call-2" || failed.ToolName != "call-2" {
		t.Fatalf("failed tool result part = %#v, want fallback name", messages[0].Parts[2])
	}
	errorOutput, ok := failed.Output.(message.ErrorTextOutput)
	if !ok || errorOutput.Value != "tool failed" {
		t.Fatalf("failed tool output = %#v, want error text", failed.Output)
	}
}

func pushProjection(t *testing.T, projection *sessionProjection, update protocol.SessionUpdate) []message.MessageChunk {
	t.Helper()
	chunks, err := projection.push(update)
	if err != nil {
		t.Fatal(err)
	}
	return chunks
}

func TestProjectedToolOutputUsesJSONAndErrorJSON(t *testing.T) {
	output := projectedToolOutput(false, json.RawMessage(`{"ok":true}`), nil)
	jsonOutput, ok := output.(message.JSONOutput)
	if !ok || string(jsonOutput.Value) != `{"ok":true}` {
		t.Fatalf("json output = %#v, want JSONOutput", output)
	}

	output = projectedToolOutput(true, json.RawMessage(`{"error":"boom"}`), nil)
	errorOutput, ok := output.(message.ErrorJSONOutput)
	if !ok || string(errorOutput.Value) != `{"error":"boom"}` {
		t.Fatalf("error json output = %#v, want ErrorJSONOutput", output)
	}
}
