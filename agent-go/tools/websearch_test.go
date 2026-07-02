package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

func runWebSearch(t *testing.T, e *Executor, input map[string]any) message.ToolResultOutput {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: t.Name(),
		ToolName:   "WebSearch",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	return result.Result.Output
}

func TestWebSearch_UsesTavilyWhenApiKeySet(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")

	oldTavilyURL := tavilySearchURL
	defer func() { tavilySearchURL = oldTavilyURL }()

	calledTavily := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledTavily = true
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var req tavilyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.APIKey != "test-key" {
			t.Fatalf("expected api_key test-key, got %q", req.APIKey)
		}
		if req.Query != "golang" {
			t.Fatalf("expected query golang, got %q", req.Query)
		}
		if req.SearchDepth != "basic" {
			t.Fatalf("expected search_depth basic, got %q", req.SearchDepth)
		}
		if req.MaxResults != 5 {
			t.Fatalf("expected max_results 5, got %d", req.MaxResults)
		}
		if len(req.IncludeDomains) != 1 || req.IncludeDomains[0] != "go.dev" {
			t.Fatalf("unexpected include_domains: %#v", req.IncludeDomains)
		}
		if len(req.ExcludeDomains) != 1 || req.ExcludeDomains[0] != "example.com" {
			t.Fatalf("unexpected exclude_domains: %#v", req.ExcludeDomains)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Go","url":"https://go.dev","content":"The Go programming language"}]}`))
	}))
	defer server.Close()
	tavilySearchURL = server.URL

	e := New(t.TempDir(), t.TempDir(), t.Name())
	out := runWebSearch(t, e, map[string]any{
		"query":           "golang",
		"allowed_domains": []string{"go.dev"},
		"blocked_domains": []string{"example.com"},
	})

	if !calledTavily {
		t.Fatal("expected Tavily endpoint to be called")
	}
	textOut, ok := out.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput, got %T", out)
	}
	if !strings.Contains(textOut.Value, "https://go.dev") {
		t.Fatalf("expected search result URL in output, got %q", textOut.Value)
	}
	if !strings.Contains(textOut.Value, "The Go programming language") {
		t.Fatalf("expected search result content in output, got %q", textOut.Value)
	}
}

func TestWebSearchProviderRequiredMessageMentionsSupportedProviders(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")

	e := New(t.TempDir(), t.TempDir(), t.Name())
	out := runWebSearch(t, e, map[string]any{"query": "golang"})

	textOut, ok := out.(message.ErrorTextOutput)
	if !ok {
		t.Fatalf("expected ErrorTextOutput, got %T", out)
	}
	if !strings.Contains(textOut.Value, "TAVILY_API_KEY") {
		t.Fatalf("provider error should mention TAVILY_API_KEY, got %q", textOut.Value)
	}
}
