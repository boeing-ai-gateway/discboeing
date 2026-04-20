//go:build integration

package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
)

// readCodexToken reads the Codex token from CODEX_TOKEN.
func readCodexToken(t *testing.T) string {
	t.Helper()
	if token := strings.TrimSpace(os.Getenv("CODEX_TOKEN")); token != "" {
		return token
	}
	t.Skip("CODEX_TOKEN not set")
	return ""
}

// readAPIKey reads the OpenAI API key from OPENAI_API_KEY env var or key.txt.
func readAPIKey(t *testing.T) string {
	t.Helper()
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	// Try reading from key.txt at repo root.
	for _, path := range []string{"../../../key.txt", "../../../../key.txt"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "export OPENAI_API_KEY=") {
				key := strings.TrimPrefix(line, "export OPENAI_API_KEY=")
				key = strings.Trim(key, `"'`)
				if key != "" {
					return key
				}
			}
		}
	}
	t.Skip("OPENAI_API_KEY not set and key.txt not found")
	return ""
}

// readCodexWebSocketConfig returns config for real Codex websocket integration
// tests and skips when the required token is not available.
func readCodexWebSocketConfig(t *testing.T) providers.Config {
	t.Helper()

	return providers.Config{
		"auth_token":       readCodexToken(t),
		configUseWebSocket: "true",
	}
}

func codexWebSocketProvider(t *testing.T) providers.Provider {
	t.Helper()
	p, err := providers.New(codexProviderID, readCodexWebSocketConfig(t))
	if err != nil {
		t.Fatalf("create codex provider: %v", err)
	}
	return p
}

// readCodexSSEConfig returns config for real Codex SSE integration tests.
func readCodexSSEConfig(t *testing.T) providers.Config {
	t.Helper()
	return providers.Config{
		"auth_token": readCodexToken(t),
		"base_url":   strings.Replace(codexDefaultBaseURL, "wss://", "https://", 1),
	}
}

func codexSSEProvider(t *testing.T) providers.Provider {
	t.Helper()
	p, err := providers.New(codexProviderID, readCodexSSEConfig(t))
	if err != nil {
		t.Fatalf("create codex sse provider: %v", err)
	}
	return p
}

func completeAndCaptureResponseID(ctx context.Context, p providers.Provider, req providers.CompleteRequest) (string, error) {
	var responseID string
	for chunk, err := range p.Complete(ctx, req) {
		if err != nil {
			return "", err
		}
		if meta, ok := chunk.(message.ResponseMetadataChunk); ok && meta.ID != "" {
			responseID = meta.ID
		}
	}
	if responseID == "" {
		return "", fmt.Errorf("missing response id from stream")
	}
	return responseID, nil
}

func completeExpectError(ctx context.Context, p providers.Provider, req providers.CompleteRequest) error {
	for _, err := range p.Complete(ctx, req) {
		if err != nil {
			return err
		}
	}
	return nil
}

func TestCodexWebSocket_GPT53CodexSparkToolCall(t *testing.T) {
	p := codexWebSocketProvider(t)

	req := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: codexProviderID, ModelID: "gpt-5.3-codex-spark"},
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You are testing tool calling. You must call the get_weather tool exactly once. Do not answer in plain text."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "What is the weather in Paris? Use the tool."},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get the current weather for a location.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}},"required":["location"],"additionalProperties":false}`),
			},
		},
	}

	acc := message.NewChunkAccumulator()
	var (
		responseID   string
		finishReason string
		sawToolCall  bool
		toolCallID   string
		toolName     string
		toolInput    string
	)
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatalf("codex spark completion failed: %v", err)
		}
		if chunk != nil {
			acc.Push(chunk)
		}
		switch c := chunk.(type) {
		case message.ResponseMetadataChunk:
			responseID = c.ID
		case message.ToolCallChunk:
			sawToolCall = true
			toolCallID = c.ToolCallID
			toolName = c.ToolName
			toolInput = c.Input
		case message.ToolInputStartChunk:
			sawToolCall = true
			toolCallID = c.ToolCallID
			toolName = c.ToolName
		case message.ToolInputDeltaChunk:
			toolInput += c.InputTextDelta
		case message.FinishChunk:
			finishReason = c.FinishReason.Unified
		}
	}
	acc.Close()

	if responseID == "" {
		t.Fatal("expected non-empty response ID")
	}
	if finishReason != "tool-calls" {
		t.Fatalf("expected finish reason 'tool-calls', got %q", finishReason)
	}
	if !sawToolCall {
		assistantMsg := acc.Message()
		for _, part := range assistantMsg.Parts {
			if tc, ok := part.(message.ToolCallPart); ok {
				sawToolCall = true
				toolCallID = tc.ToolCallID
				toolName = tc.ToolName
				toolInput = tc.Input
				break
			}
		}
		if !sawToolCall {
			t.Fatal("expected at least one tool call")
		}
	}
	if toolCallID == "" {
		t.Fatal("expected non-empty tool call ID")
	}
	if toolName != "get_weather" {
		t.Fatalf("expected tool name 'get_weather', got %q", toolName)
	}
	if strings.TrimSpace(toolInput) == "" {
		t.Fatal("expected non-empty tool input")
	}
}

func TestCodexSSE_GPT53CodexSparkToolCall(t *testing.T) {
	p := codexSSEProvider(t)

	req := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: codexProviderID, ModelID: "gpt-5.3-codex-spark"},
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You are testing tool calling. You must call the get_weather tool exactly once. Do not answer in plain text."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "What is the weather in Paris? Use the tool."},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get the current weather for a location.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}},"required":["location"],"additionalProperties":false}`),
			},
		},
	}

	acc := message.NewChunkAccumulator()
	var (
		responseID   string
		finishReason string
		sawToolCall  bool
		toolCallID   string
		toolName     string
		toolInput    string
	)
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatalf("codex spark sse completion failed: %v", err)
		}
		if chunk != nil {
			acc.Push(chunk)
		}
		switch c := chunk.(type) {
		case message.ResponseMetadataChunk:
			responseID = c.ID
		case message.ToolCallChunk:
			sawToolCall = true
			toolCallID = c.ToolCallID
			toolName = c.ToolName
			toolInput = c.Input
		case message.ToolInputStartChunk:
			sawToolCall = true
			toolCallID = c.ToolCallID
			toolName = c.ToolName
		case message.ToolInputDeltaChunk:
			toolInput += c.InputTextDelta
		case message.FinishChunk:
			finishReason = c.FinishReason.Unified
		}
	}
	acc.Close()

	if responseID == "" {
		t.Fatal("expected non-empty response ID")
	}
	if finishReason != "tool-calls" {
		t.Fatalf("expected finish reason 'tool-calls', got %q", finishReason)
	}
	if !sawToolCall {
		assistantMsg := acc.Message()
		for _, part := range assistantMsg.Parts {
			if tc, ok := part.(message.ToolCallPart); ok {
				sawToolCall = true
				toolCallID = tc.ToolCallID
				toolName = tc.ToolName
				toolInput = tc.Input
				break
			}
		}
		if !sawToolCall {
			t.Fatal("expected at least one tool call")
		}
	}
	if toolCallID == "" {
		t.Fatal("expected non-empty tool call ID")
	}
	if toolName != "get_weather" {
		t.Fatalf("expected tool name 'get_weather', got %q", toolName)
	}
	if strings.TrimSpace(toolInput) == "" {
		t.Fatal("expected non-empty tool input")
	}
}

func TestCodexWebSocket_GPT53CodexSparkCustomToolCall(t *testing.T) {
	p := codexWebSocketProvider(t)

	req := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: codexProviderID, ModelID: "gpt-5.3-codex-spark"},
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You are testing tool calling. You must call the apply_patch tool exactly once. Do not answer in plain text."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Call apply_patch to add a file named codex-tool-test.txt with the contents 'codex spark tool test'."},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Type:        "custom",
				Name:        "apply_patch",
				Description: "Apply a structured patch to one or more files.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}},"required":["input"],"additionalProperties":false}`),
				Format: &providers.ToolFormat{
					Type:       "grammar",
					Syntax:     "lark",
					Definition: "start: /[\\s\\S]+/",
				},
			},
		},
	}

	acc := message.NewChunkAccumulator()
	var (
		responseID   string
		finishReason string
		toolCallID   string
		toolName     string
		toolInput    string
	)
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatalf("codex spark websocket custom-tool completion failed: %v", err)
		}
		if chunk != nil {
			acc.Push(chunk)
		}
		switch c := chunk.(type) {
		case message.ResponseMetadataChunk:
			responseID = c.ID
		case message.ToolCallChunk:
			toolCallID = c.ToolCallID
			toolName = c.ToolName
			toolInput = c.Input
		case message.FinishChunk:
			finishReason = c.FinishReason.Unified
		}
	}
	acc.Close()

	if responseID == "" {
		t.Fatal("expected non-empty response ID")
	}
	if finishReason != "tool-calls" {
		t.Fatalf("expected finish reason 'tool-calls', got %q", finishReason)
	}
	if toolCallID == "" {
		t.Fatal("expected non-empty tool call ID")
	}
	if toolName != "apply_patch" {
		t.Fatalf("expected tool name 'apply_patch', got %q", toolName)
	}
	if strings.TrimSpace(toolInput) == "" {
		t.Fatal("expected non-empty tool input")
	}
}

func TestCodexSSE_GPT53CodexSparkCustomToolCall(t *testing.T) {
	p := codexSSEProvider(t)

	req := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: codexProviderID, ModelID: "gpt-5.3-codex-spark"},
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You are testing tool calling. You must call the apply_patch tool exactly once. Do not answer in plain text."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Call apply_patch to add a file named codex-tool-test.txt with the contents 'codex spark tool test'."},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Type:        "custom",
				Name:        "apply_patch",
				Description: "Apply a structured patch to one or more files.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"input":{"type":"string"}},"required":["input"],"additionalProperties":false}`),
				Format: &providers.ToolFormat{
					Type:       "grammar",
					Syntax:     "lark",
					Definition: "start: /[\\s\\S]+/",
				},
			},
		},
	}

	acc := message.NewChunkAccumulator()
	var (
		responseID   string
		finishReason string
		toolCallID   string
		toolName     string
		toolInput    string
	)
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatalf("codex spark sse custom-tool completion failed: %v", err)
		}
		if chunk != nil {
			acc.Push(chunk)
		}
		switch c := chunk.(type) {
		case message.ResponseMetadataChunk:
			responseID = c.ID
		case message.ToolCallChunk:
			toolCallID = c.ToolCallID
			toolName = c.ToolName
			toolInput = c.Input
		case message.FinishChunk:
			finishReason = c.FinishReason.Unified
		}
	}
	acc.Close()

	if responseID == "" {
		t.Fatal("expected non-empty response ID")
	}
	if finishReason != "tool-calls" {
		t.Fatalf("expected finish reason 'tool-calls', got %q", finishReason)
	}
	if toolCallID == "" {
		t.Fatal("expected non-empty tool call ID")
	}
	if toolName != "apply_patch" {
		t.Fatalf("expected tool name 'apply_patch', got %q", toolName)
	}
	if strings.TrimSpace(toolInput) == "" {
		t.Fatal("expected non-empty tool input")
	}
}

// TestCodexWebSocket_ContinuationInstructionsDependOnConnection validates the
// real Codex websocket behavior across a provider restart:
// 1) a reused websocket continuation succeeds without top-level instructions,
// 2) after restart (fresh socket), continuation without instructions fails, and
// 3) resending instructions on that fresh socket succeeds.
//
// Run with:
// CODEX_TOKEN=<token> go test -tags integration -run TestCodexWebSocket_ContinuationInstructionsDependOnConnection ./providers/openai/
func TestCodexWebSocket_ContinuationInstructionsDependOnConnection(t *testing.T) {
	ctx := context.Background()

	p := codexWebSocketProvider(t)

	turn1 := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: "openai", ModelID: "gpt-5.4"},
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{message.TextPart{Text: "You are Codex. Respond briefly."}}},
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: ONE"}}},
		},
	}
	resp1ID, err := completeAndCaptureResponseID(ctx, p, turn1)
	if err != nil {
		t.Fatalf("turn 1 failed: %v", err)
	}

	// No system message on purpose: reused websocket chain should still work.
	turn2 := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: "openai", ModelID: "gpt-5.4"},
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: ONE"}}},
			{Role: "assistant", ID: resp1ID, Parts: []message.Part{message.TextPart{Text: "ONE"}}},
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: TWO"}}},
		},
	}
	resp2ID, err := completeAndCaptureResponseID(ctx, p, turn2)
	if err != nil {
		t.Fatalf("turn 2 should succeed on reused websocket without instructions: %v", err)
	}

	// Simulate agent-go restart: new provider, empty websocket pool.
	restartedProvider := codexWebSocketProvider(t)

	// Fresh websocket continuation WITHOUT system instructions should fail.
	restartNoInstructions := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: "openai", ModelID: "gpt-5.4"},
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: ONE"}}},
			{Role: "assistant", ID: resp2ID, Parts: []message.Part{message.TextPart{Text: "TWO"}}},
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: THREE"}}},
		},
	}
	err = completeExpectError(ctx, restartedProvider, restartNoInstructions)
	if err == nil {
		t.Fatal("expected fresh-connection continuation without instructions to fail")
	}
	if !strings.Contains(err.Error(), "Instructions are required") {
		t.Fatalf("expected 'Instructions are required' error, got: %v", err)
	}

	// Re-sending instructions on the fresh connection should succeed.
	restartWithInstructions := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: "openai", ModelID: "gpt-5.4"},
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{message.TextPart{Text: "You are Codex. Respond briefly."}}},
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: ONE"}}},
			{Role: "assistant", ID: resp2ID, Parts: []message.Part{message.TextPart{Text: "TWO"}}},
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: THREE"}}},
		},
	}
	if _, err := completeAndCaptureResponseID(ctx, restartedProvider, restartWithInstructions); err != nil {
		t.Fatalf("fresh-connection continuation with instructions should succeed: %v", err)
	}
}

func TestCompleteToolCallDeltasHaveCallID(t *testing.T) {
	apiKey := readAPIKey(t)

	p, err := New(providers.Config{"api_key": apiKey}, false, defaultBaseURL)
	if err != nil {
		t.Fatal(err)
	}

	req := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: "openai", ModelID: "gpt-4o"},
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Use the echo tool to echo back the text: hello world"},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "echo",
				Description: "Echo back the provided text",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string","description":"The text to echo"}},"required":["text"]}`),
			},
		},
	}

	var (
		sawToolInputStart bool
		deltaIDs          []string
		endIDs            []string
	)
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatalf("unexpected error from Complete: %v", err)
		}
		switch c := chunk.(type) {
		case message.ToolInputStartChunk:
			sawToolInputStart = true
			if c.ToolCallID == "" {
				t.Error("ToolInputStartChunk has empty ToolCallID")
			}
		case message.ToolInputDeltaChunk:
			deltaIDs = append(deltaIDs, c.ToolCallID)
		case message.ToolInputEndChunk:
			endIDs = append(endIDs, c.ToolCallID)
		}
	}

	if !sawToolInputStart {
		t.Fatal("expected at least one tool call, got none (model may have responded without tool use)")
	}

	for i, id := range deltaIDs {
		if id == "" {
			t.Errorf("ToolInputDeltaChunk[%d] has empty ToolCallID (bug: item_id→call_id lookup missing)", i)
		}
	}
	for i, id := range endIDs {
		if id == "" {
			t.Errorf("ToolInputEndChunk[%d] has empty ToolCallID (bug: item_id→call_id lookup missing)", i)
		}
	}

	if len(deltaIDs) == 0 {
		t.Error("expected at least one ToolInputDeltaChunk")
	}
}
