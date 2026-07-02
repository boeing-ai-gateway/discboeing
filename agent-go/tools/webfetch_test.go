package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func runWebFetch(t *testing.T, e *Executor, input map[string]string) message.ToolResultOutput {
	t.Helper()
	return runWebFetchWithContext(t, e, nil, input)
}

func runWebFetchWithContext(t *testing.T, e *Executor, toolCtx *thread.ToolContext, input map[string]string) message.ToolResultOutput {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: t.Name(),
		ToolName:   "WebFetch",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	return result.Result.Output
}

func TestWebFetch_UsesBrowserLikeUserAgent(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")

	oldClient := httpClient
	defer func() { httpClient = oldClient }()

	var gotUserAgent string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello from server"))
	}))
	defer server.Close()

	httpClient = server.Client()
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out := runWebFetch(t, e, map[string]string{
		"url": server.URL,
	})

	textOut, ok := out.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput, got %T", out)
	}
	if !strings.Contains(textOut.Value, "hello from server") {
		t.Fatalf("expected server response in output, got: %q", textOut.Value)
	}
	if !strings.Contains(gotUserAgent, "Mozilla/5.0") {
		t.Errorf("expected browser-like User-Agent, got %q", gotUserAgent)
	}
	if !strings.Contains(gotUserAgent, "Discboeing/1.0") {
		t.Errorf("expected Discboeing identifier in User-Agent, got %q", gotUserAgent)
	}
}

func TestWebFetch_UsesTavilyWhenApiKeySet(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")

	oldTavilyURL := tavilyExtractURL
	defer func() { tavilyExtractURL = oldTavilyURL }()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var req tavilyExtractRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.APIKey != "test-key" {
			t.Fatalf("expected api_key test-key, got %q", req.APIKey)
		}
		if len(req.URLs) != 1 || req.URLs[0] != "https://example.com/article" {
			t.Fatalf("unexpected urls payload: %+v", req.URLs)
		}
		if req.Query != "" {
			t.Fatalf("expected empty query when prompt omitted, got %q", req.Query)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/article","raw_content":"from tavily extract"}]}`))
	}))
	defer server.Close()
	tavilyExtractURL = server.URL

	e := New(t.TempDir(), t.TempDir(), t.Name())
	out := runWebFetch(t, e, map[string]string{
		"url": "https://example.com/article",
	})

	if !called {
		t.Fatal("expected Tavily extract endpoint to be called")
	}
	textOut, ok := out.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput, got %T", out)
	}
	if !strings.Contains(textOut.Value, "from tavily extract") {
		t.Fatalf("expected Tavily content in output, got: %q", textOut.Value)
	}
}

func TestWebFetch_SendsPromptToTavilyAndAnswersWithCurrentModel(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")

	oldTavilyURL := tavilyExtractURL
	defer func() { tavilyExtractURL = oldTavilyURL }()

	provider := &webFetchMockProvider{response: "The article is about shipping release 1.2."}
	toolCtx := &thread.ToolContext{
		ProviderID:       "mock",
		ModelID:          "mock-model",
		ProviderResolver: staticProviderResolver{"mock": provider},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req tavilyExtractRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if req.Query != "What shipped?" {
			t.Fatalf("expected query from prompt, got %q", req.Query)
		}
		if req.ChunksPerSource == nil || *req.ChunksPerSource != 3 {
			t.Fatalf("expected chunks_per_source=3, got %+v", req.ChunksPerSource)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/article","raw_content":"Discboeing release 1.2 shipped today."}]}`))
	}))
	defer server.Close()
	tavilyExtractURL = server.URL

	e := New(t.TempDir(), t.TempDir(), t.Name())
	out := runWebFetchWithContext(t, e, toolCtx, map[string]string{
		"url":    "https://example.com/article",
		"prompt": "What shipped?",
	})

	textOut, ok := out.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput, got %T", out)
	}
	if textOut.Value != "The article is about shipping release 1.2." {
		t.Fatalf("unexpected answered output: %q", textOut.Value)
	}
}

type staticProviderResolver map[string]providers.Provider

func (r staticProviderResolver) Get(id string) (providers.Provider, error) {
	p, ok := r[id]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", id)
	}
	return p, nil
}

type webFetchMockProvider struct {
	response string
	requests []providers.CompleteRequest
}

func (p *webFetchMockProvider) ID() string { return "mock" }

func (p *webFetchMockProvider) Complete(_ context.Context, req providers.CompleteRequest) iter.Seq2[message.ProviderMessageChunk, error] {
	p.requests = append(p.requests, req)
	return func(yield func(message.ProviderMessageChunk, error) bool) {
		if !yield(message.TextStartChunk{ID: "text-1"}, nil) {
			return
		}
		if !yield(message.TextDeltaChunk{ID: "text-1", Delta: p.response}, nil) {
			return
		}
		yield(message.TextEndChunk{ID: "text-1"}, nil)
	}
}

func (p *webFetchMockProvider) DefaultModels() map[string]providers.ModelRef { return nil }
func (p *webFetchMockProvider) ListModels(_ context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func TestWebFetch_PromptRequiresTavily(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")

	e := New(t.TempDir(), t.TempDir(), t.Name())
	out := runWebFetchWithContext(t, e, nil, map[string]string{
		"url":    "https://example.com/article",
		"prompt": "What shipped today?",
	})

	errOut, ok := out.(message.ErrorTextOutput)
	if !ok {
		t.Fatalf("expected ErrorTextOutput, got %T", out)
	}
	if !strings.Contains(errOut.Value, "prompt requires Tavily-backed WebFetch extraction") {
		t.Fatalf("unexpected error output: %q", errOut.Value)
	}
}
