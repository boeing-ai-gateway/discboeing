//go:build e2e_mock_llm

package e2emockllm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
)

func TestFixtureMatching(t *testing.T) {
	fixtures := &FixtureSet{
		Responses: []Entry{
			{Name: "exact", Match: Match{Exact: "hello"}, Response: Response{Text: "exact response"}},
			{Name: "contains", Match: Match{Contains: "deploy"}, Response: Response{Text: "contains response"}},
			{Name: "regex", Match: Match{Regex: `issue-\d+`}, Response: Response{Text: "regex response"}},
		},
		Fallback: &Response{Text: "fallback response"},
	}

	for _, tt := range []struct {
		input string
		want  string
	}{
		{input: "hello", want: "exact response"},
		{input: "please deploy", want: "contains response"},
		{input: "fix issue-123", want: "regex response"},
		{input: "other", want: "fallback response"},
	} {
		resp, _, ok, err := fixtures.Match(tt.input)
		if err != nil {
			t.Fatalf("Match(%q): %v", tt.input, err)
		}
		if !ok || resp.Text != tt.want {
			t.Fatalf("Match(%q) = (%q, %v), want %q", tt.input, resp.Text, ok, tt.want)
		}
	}
}

func TestLoadFixturesFromDir(t *testing.T) {
	dir := t.TempDir()
	data := `{
	  "responses": [
	    {"name":"one","match":{"exact":"one"},"response":{"text":"first"}},
	    {"name":"tool","match":{"contains":"run tool"},"response":{"toolCalls":[{"name":"Bash","input":"{\"command\":\"pwd\"}"}]}}
	  ],
	  "fallback": {"text":"fallback"}
	}`
	if err := os.WriteFile(filepath.Join(dir, "fixtures.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	fixtures, err := LoadFixturesFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(fixtures.Responses))
	}
	resp, _, ok, err := fixtures.Match("please run tool")
	if err != nil || !ok {
		t.Fatalf("expected tool match, ok=%v err=%v", ok, err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "Bash" {
		t.Fatalf("unexpected tool calls: %#v", resp.ToolCalls)
	}
}

func TestRequestInputUsesLatestUserText(t *testing.T) {
	input := RequestInput(providers.CompleteRequest{Messages: []message.Message{
		{Role: "system", Synthetic: true, Parts: []message.Part{message.TextPart{Text: "ignore"}}},
		{Role: "user", Parts: []message.Part{message.TextPart{Text: "first"}}},
		{Role: "assistant", Parts: []message.Part{message.TextPart{Text: "assistant"}}},
		{Role: "user", Parts: []message.Part{message.TextPart{Text: "second"}}},
	}})
	if input != "second" {
		t.Fatalf("expected latest user input, got %q", input)
	}
}

func TestCompleteEmitsTextAndFinish(t *testing.T) {
	p := &Provider{fixtures: &FixtureSet{Responses: []Entry{{Match: Match{Exact: "hello"}, Response: Response{Text: "hi"}}}}}
	var chunks []message.ProviderMessageChunk
	for chunk, err := range p.Complete(context.Background(), providers.CompleteRequest{
		Model:    providers.ModelRef{ProviderID: ProviderID, ModelID: defaultModelID},
		Messages: []message.Message{{Role: "user", Parts: []message.Part{message.TextPart{Text: "hello"}}}},
	}) {
		if err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 6 {
		t.Fatalf("expected 6 chunks, got %d: %#v", len(chunks), chunks)
	}
	if delta, ok := chunks[3].(message.TextDeltaChunk); !ok || delta.Delta != "hi" {
		t.Fatalf("expected text delta hi at chunks[3], got %#v", chunks[3])
	}
	if finish, ok := chunks[5].(message.FinishChunk); !ok || finish.FinishReason.Unified != "stop" {
		t.Fatalf("expected stop finish, got %#v", chunks[5])
	}
}
