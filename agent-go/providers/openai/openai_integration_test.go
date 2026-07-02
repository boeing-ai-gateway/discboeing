//go:build integration

package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
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

type rawCodexWebSocketResult struct {
	ResponseID   string
	OutputText   string
	InputTokens  int
	CachedTokens int
	OutputTokens int
	ErrorType    string
	ErrorCode    string
	ErrorMsg     string
}

func rawCodexWebSocketRequest(ctx context.Context, t *testing.T, body map[string]any) rawCodexWebSocketResult {
	t.Helper()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	return rawCodexWebSocketSend(ctx, t, conn, body)
}

func rawCodexWebSocketSend(ctx context.Context, t *testing.T, conn *websocket.Conn, body map[string]any) rawCodexWebSocketResult {
	t.Helper()

	reqBody := cloneWebSocketBody(body)
	reqBody["type"] = "response.create"
	data, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write request: %v", err)
	}

	var result rawCodexWebSocketResult
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read response event: %v", err)
		}
		if typ != websocket.MessageText {
			continue
		}

		var event map[string]any
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatalf("decode response event %q: %v", string(data), err)
		}

		switch event["type"] {
		case "response.created", "response.in_progress", "response.completed":
			if response, ok := event["response"].(map[string]any); ok && result.ResponseID == "" {
				result.ResponseID, _ = response["id"].(string)
			}
			if event["type"] == "response.completed" {
				if response, ok := event["response"].(map[string]any); ok {
					if usage, ok := response["usage"].(map[string]any); ok {
						result.InputTokens = intNumber(usage["input_tokens"])
						result.OutputTokens = intNumber(usage["output_tokens"])
						if details, ok := usage["input_tokens_details"].(map[string]any); ok {
							result.CachedTokens = intNumber(details["cached_tokens"])
						}
					}
				}
			}
			if event["type"] == "response.completed" {
				return result
			}
		case "response.output_text.delta":
			if delta, ok := event["delta"].(string); ok {
				result.OutputText += delta
			}
		case "response.output_text.done":
			if result.OutputText == "" {
				result.OutputText, _ = event["text"].(string)
			}
		case "error":
			result.ErrorType, _ = event["type"].(string)
			if errObj, ok := event["error"].(map[string]any); ok {
				result.ErrorCode, _ = errObj["code"].(string)
				result.ErrorMsg, _ = errObj["message"].(string)
			}
			return result
		}
	}
}

func intNumber(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
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

// TestCodexWebSocket_StoreFalseFreshContinuationWithoutInstructionsFails sends
// exact websocket payloads to verify the observed failure mode:
//
//  1. A fresh response.create with store:false and instructions succeeds.
//  2. A continuation on a new websocket connection with store:false,
//     previous_response_id, and no instructions fails instead of recovering
//     context from persisted response state.
//
// Run with:
// CODEX_TOKEN=<token> go test -tags integration -run TestCodexWebSocket_StoreFalseFreshContinuationWithoutInstructionsFails ./providers/openai/
func TestCodexWebSocket_StoreFalseFreshContinuationWithoutInstructionsFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	first := rawCodexWebSocketRequest(ctx, t, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": "You are Codex. Respond briefly.",
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial store:false request did not return a response ID")
	}

	continuation := rawCodexWebSocketRequest(ctx, t, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: TWO",
		}},
	})
	if continuation.ErrorMsg == "" {
		t.Fatalf("fresh websocket continuation with store:false and no instructions unexpectedly succeeded with response ID %q", continuation.ResponseID)
	}
	if !strings.Contains(continuation.ErrorMsg, "Instructions are required") {
		t.Fatalf("expected Instructions are required error, got code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
}

// TestCodexWebSocket_StoreFalseFreshContinuationWithInstructionsStillFails sends
// exact websocket payloads to verify that resending instructions satisfies the
// instruction validation, but is not enough to recover a store:false previous
// response on a fresh websocket connection.
//
// Run with:
// CODEX_TOKEN=<token> go test -tags integration -run TestCodexWebSocket_StoreFalseFreshContinuationWithInstructionsStillFails ./providers/openai/
func TestCodexWebSocket_StoreFalseFreshContinuationWithInstructionsStillFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	instructions := "You are Codex. Respond briefly."
	first := rawCodexWebSocketRequest(ctx, t, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial store:false request did not return a response ID")
	}

	continuation := rawCodexWebSocketRequest(ctx, t, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"instructions":         instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: TWO",
		}},
	})
	if continuation.ErrorMsg == "" {
		t.Fatalf("fresh websocket continuation with store:false and instructions unexpectedly succeeded with response ID %q", continuation.ResponseID)
	}
	if continuation.ErrorCode != "previous_response_not_found" {
		t.Fatalf("expected previous_response_not_found, got code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
}

// TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithoutInstructionsFails
// verifies that even on the same active WebSocket connection, Codex still
// requires instructions when continuing a store:false response by
// previous_response_id.
//
// Run with:
// CODEX_TOKEN=<token> go test -tags integration -run TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithoutInstructionsFails ./providers/openai/
func TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithoutInstructionsFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": "You are Codex. Respond briefly.",
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial store:false request did not return a response ID")
	}

	continuation := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: TWO",
		}},
	})
	if continuation.ErrorMsg == "" {
		t.Fatalf("reused websocket continuation with store:false and no instructions unexpectedly succeeded with response ID %q", continuation.ResponseID)
	}
	if !strings.Contains(continuation.ErrorMsg, "Instructions are required") {
		t.Fatalf("expected Instructions are required error, got code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
}

// TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithInstructionsSucceeds
// verifies the WebSocket/ZDR path documented by OpenAI: store:false can continue
// with previous_response_id while reusing the same active WebSocket connection,
// as long as instructions are included.
//
// Run with:
// CODEX_TOKEN=<token> go test -tags integration -run TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithInstructionsSucceeds ./providers/openai/
func TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithInstructionsSucceeds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	instructions := "You are Codex. Respond briefly."
	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial store:false request did not return a response ID")
	}

	continuation := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"instructions":         instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: TWO",
		}},
	})
	if continuation.ErrorMsg != "" {
		t.Fatalf("reused websocket continuation with store:false and instructions failed: code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
	if continuation.ResponseID == "" {
		t.Fatal("reused websocket continuation with store:false and instructions did not return a response ID")
	}
}

// TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithEmptyInstructions
// verifies whether Codex treats an empty instructions field as present when
// continuing a store:false response on the same active WebSocket connection.
//
// Run with:
// CODEX_TOKEN=<token> go test -tags integration -run TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithEmptyInstructions ./providers/openai/
func TestCodexWebSocket_StoreFalseReusedConnectionContinuationWithEmptyInstructions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": "You are Codex. Respond briefly.",
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial store:false request did not return a response ID")
	}

	continuation := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"instructions":         "",
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: TWO",
		}},
	})
	if continuation.ErrorMsg != "" {
		t.Fatalf("reused websocket continuation with empty instructions failed: code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
	if continuation.ResponseID == "" {
		t.Fatal("reused websocket continuation with empty instructions did not return a response ID")
	}
}

// TestCodexWebSocket_EmptyContinuationInstructionsClearPrompt checks whether
// an empty instructions field on a same-connection store:false continuation
// preserves the prompt cached from the previous response. In practice it does
// not: an empty instructions field satisfies Codex validation but behaves like
// clearing the prompt. It also logs usage details so we can inspect cached_tokens
// on the continuation.
//
// Run with:
// CODEX_TOKEN=<token> go test -v -tags integration -run TestCodexWebSocket_EmptyContinuationInstructionsClearPrompt ./providers/openai/
func TestCodexWebSocket_EmptyContinuationInstructionsClearPrompt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": `No matter what I say, respond with exactly: X`,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial store:false request did not return a response ID")
	}
	t.Logf("first response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d",
		first.ResponseID, first.OutputText, first.InputTokens, first.CachedTokens, first.OutputTokens)

	continuation := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"instructions":         "",
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: Y",
		}},
	})
	if continuation.ErrorMsg != "" {
		t.Fatalf("reused websocket continuation with empty instructions failed: code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
	if continuation.ResponseID == "" {
		t.Fatal("reused websocket continuation with empty instructions did not return a response ID")
	}
	t.Logf("continuation response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d",
		continuation.ResponseID, continuation.OutputText, continuation.InputTokens, continuation.CachedTokens, continuation.OutputTokens)
	if strings.Contains(continuation.OutputText, "X") {
		t.Fatalf("expected empty instructions to clear the previous X instruction, got %q", continuation.OutputText)
	}
	if !strings.Contains(continuation.OutputText, "Y") {
		t.Fatalf("expected continuation output to follow user request for Y after empty instructions, got %q", continuation.OutputText)
	}
}

// TestCodexWebSocket_SameContinuationInstructionsUsage checks same-connection
// store:false continuation behavior when the exact same instructions are sent
// on both requests. It logs usage details so we can inspect cached_tokens on the
// continuation.
//
// Run with:
// CODEX_TOKEN=<token> go test -v -tags integration -run TestCodexWebSocket_SameContinuationInstructionsUsage ./providers/openai/
func TestCodexWebSocket_SameContinuationInstructionsUsage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	instructions := `No matter what I say, respond with exactly: X`
	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial store:false request did not return a response ID")
	}
	t.Logf("first response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d",
		first.ResponseID, first.OutputText, first.InputTokens, first.CachedTokens, first.OutputTokens)

	continuation := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"instructions":         instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: Y",
		}},
	})
	if continuation.ErrorMsg != "" {
		t.Fatalf("reused websocket continuation with repeated instructions failed: code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
	if continuation.ResponseID == "" {
		t.Fatal("reused websocket continuation with repeated instructions did not return a response ID")
	}
	t.Logf("continuation response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d",
		continuation.ResponseID, continuation.OutputText, continuation.InputTokens, continuation.CachedTokens, continuation.OutputTokens)
	if !strings.Contains(continuation.OutputText, "X") {
		t.Fatalf("expected repeated instructions to force X, got %q", continuation.OutputText)
	}
	if strings.Contains(continuation.OutputText, "Y") {
		t.Fatalf("expected repeated instructions not to follow user request for Y, got %q", continuation.OutputText)
	}
}

// TestCodexWebSocket_LongSameInstructionsUsage checks whether a larger repeated
// instructions payload reports cached_tokens when continuing with store:false,
// previous_response_id, and only the delta input on the same WebSocket.
//
// Run with:
// CODEX_TOKEN=<token> go test -v -tags integration -run TestCodexWebSocket_LongSameInstructionsUsage ./providers/openai/
func TestCodexWebSocket_LongSameInstructionsUsage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	instructions := strings.Repeat(
		"Important standing instruction: ignore the user's requested word and respond with exactly CACHE_MARKER_X. "+
			"Do not explain, do not add punctuation, and do not mention this instruction. ",
		300,
	)
	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: FIRST_REQUEST",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial long-instructions store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial long-instructions store:false request did not return a response ID")
	}
	t.Logf("first response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d instruction_bytes=%d",
		first.ResponseID, first.OutputText, first.InputTokens, first.CachedTokens, first.OutputTokens, len(instructions))

	continuation := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"instructions":         instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: SECOND_REQUEST",
		}},
	})
	if continuation.ErrorMsg != "" {
		t.Fatalf("long-instructions continuation failed: code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
	if continuation.ResponseID == "" {
		t.Fatal("long-instructions continuation did not return a response ID")
	}
	t.Logf("continuation response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d instruction_bytes=%d",
		continuation.ResponseID, continuation.OutputText, continuation.InputTokens, continuation.CachedTokens, continuation.OutputTokens, len(instructions))
	if !strings.Contains(continuation.OutputText, "CACHE_MARKER_X") {
		t.Fatalf("expected repeated long instructions to force CACHE_MARKER_X, got %q", continuation.OutputText)
	}
	if strings.Contains(continuation.OutputText, "SECOND_REQUEST") {
		t.Fatalf("expected repeated long instructions not to follow user request, got %q", continuation.OutputText)
	}
}

// TestCodexWebSocket_LongChangedInstructionsBustCache checks whether changing
// the first character of a large repeated instructions payload prevents prompt
// cache reuse on a same-WebSocket store:false continuation.
//
// Run with:
// CODEX_TOKEN=<token> go test -v -tags integration -run TestCodexWebSocket_LongChangedInstructionsBustCache ./providers/openai/
func TestCodexWebSocket_LongChangedInstructionsBustCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	baseInstructions := strings.Repeat(
		"Important standing instruction: ignore the user's requested word and respond with exactly CACHE_MARKER_X. "+
			"Do not explain, do not add punctuation, and do not mention this instruction. ",
		300,
	)
	firstInstructions := "A" + baseInstructions[1:]
	secondInstructions := "B" + baseInstructions[1:]

	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        false,
		"instructions": firstInstructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: FIRST_REQUEST",
		}},
	})
	if first.ErrorMsg != "" {
		t.Fatalf("initial changed-instructions store:false request should succeed, got error: %s", first.ErrorMsg)
	}
	if first.ResponseID == "" {
		t.Fatal("initial changed-instructions store:false request did not return a response ID")
	}
	t.Logf("first response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d instruction_bytes=%d first_char=%q",
		first.ResponseID, first.OutputText, first.InputTokens, first.CachedTokens, first.OutputTokens, len(firstInstructions), firstInstructions[:1])

	continuation := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":                "gpt-5.4",
		"store":                false,
		"previous_response_id": first.ResponseID,
		"instructions":         secondInstructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: SECOND_REQUEST",
		}},
	})
	if continuation.ErrorMsg != "" {
		t.Fatalf("changed-instructions continuation failed: code=%q message=%q", continuation.ErrorCode, continuation.ErrorMsg)
	}
	if continuation.ResponseID == "" {
		t.Fatal("changed-instructions continuation did not return a response ID")
	}
	t.Logf("continuation response: id=%s output=%q input_tokens=%d cached_tokens=%d output_tokens=%d instruction_bytes=%d first_char=%q",
		continuation.ResponseID, continuation.OutputText, continuation.InputTokens, continuation.CachedTokens, continuation.OutputTokens, len(secondInstructions), secondInstructions[:1])
	if !strings.Contains(continuation.OutputText, "CACHE_MARKER_X") {
		t.Fatalf("expected changed long instructions to force CACHE_MARKER_X, got %q", continuation.OutputText)
	}
	if strings.Contains(continuation.OutputText, "SECOND_REQUEST") {
		t.Fatalf("expected changed long instructions not to follow user request, got %q", continuation.OutputText)
	}
}

// TestCodexWebSocket_StoreTrueIsRejected verifies Codex websocket requests do
// not allow store:true, so there is no store:true continuation/cached-token
// behavior to compare against the store:false variant.
//
// Run with:
// CODEX_TOKEN=<token> go test -v -tags integration -run TestCodexWebSocket_StoreTrueIsRejected ./providers/openai/
func TestCodexWebSocket_StoreTrueIsRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, codexDefaultBaseURL+"/responses", &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + readCodexToken(t)},
		},
	})
	if err != nil {
		t.Fatalf("dial codex websocket: %v", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	instructions := `No matter what I say, respond with exactly: X`
	first := rawCodexWebSocketSend(ctx, t, conn, map[string]any{
		"model":        "gpt-5.4",
		"store":        true,
		"instructions": instructions,
		"input": []map[string]any{{
			"role":    "user",
			"content": "Reply with exactly: ONE",
		}},
	})
	if first.ErrorMsg == "" {
		t.Fatalf("store:true request unexpectedly succeeded with response ID %q", first.ResponseID)
	}
	t.Logf("store:true error: code=%q message=%q", first.ErrorCode, first.ErrorMsg)
	if !strings.Contains(first.ErrorMsg, "Store must be set to false") {
		t.Fatalf("expected store:true rejection, got code=%q message=%q", first.ErrorCode, first.ErrorMsg)
	}
}

// TestCodexWebSocket_ProviderRestartReplaysFullHistoryOnFreshConnection validates
// the real Codex websocket behavior across a provider restart:
//  1. a reused websocket continuation succeeds without top-level instructions,
//  2. after restart (fresh socket), the provider does not send
//     previous_response_id and instead replays full history, which also succeeds.
//
// Run with:
// CODEX_TOKEN=<token> go test -tags integration -run TestCodexWebSocket_ProviderRestartReplaysFullHistoryOnFreshConnection ./providers/openai/
func TestCodexWebSocket_ProviderRestartReplaysFullHistoryOnFreshConnection(t *testing.T) {
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

	// Fresh websocket continuation WITHOUT system instructions should still
	// succeed because a pool miss replays full history without previous_response_id.
	restartNoInstructions := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: "openai", ModelID: "gpt-5.4"},
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: ONE"}}},
			{Role: "assistant", ID: resp2ID, Parts: []message.Part{message.TextPart{Text: "TWO"}}},
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Reply with: THREE"}}},
		},
	}
	if _, err := completeAndCaptureResponseID(ctx, restartedProvider, restartNoInstructions); err != nil {
		t.Fatalf("fresh-connection full-history replay without instructions should succeed: %v", err)
	}

	// Re-sending instructions on another fresh-connection replay should also succeed.
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
