package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/config"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

func TestPrintPromptInput_UsesArgs(t *testing.T) {
	got, err := printPromptInput([]string{"hello", "there"}, strings.NewReader("ignored"))
	if err != nil {
		t.Fatalf("printPromptInput() error = %v", err)
	}
	if got != "hello there" {
		t.Fatalf("prompt = %q, want %q", got, "hello there")
	}
}

func TestPrintPromptInput_UsesStdin(t *testing.T) {
	got, err := printPromptInput(nil, strings.NewReader("\nhello from stdin\n"))
	if err != nil {
		t.Fatalf("printPromptInput() error = %v", err)
	}
	if got != "hello from stdin" {
		t.Fatalf("prompt = %q, want %q", got, "hello from stdin")
	}
}

func TestPrintPromptInput_RequiresPrompt(t *testing.T) {
	if _, err := printPromptInput(nil, strings.NewReader(" \n")); err == nil {
		t.Fatal("printPromptInput() error = nil, want error")
	}
}

func TestAddFlags_PrintAlias(t *testing.T) {
	oldCommandLine := flag.CommandLine
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flag.CommandLine = fs
	defer func() {
		flag.CommandLine = oldCommandLine
	}()

	flags := AddFlags()
	if err := flag.CommandLine.Parse([]string{"-p", "hello"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !flags.PrintMode() {
		t.Fatal("PrintMode() = false, want true")
	}
	if got := flag.CommandLine.Args(); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("args = %#v, want [hello]", got)
	}
}

func TestAddFlags_PrintOptions(t *testing.T) {
	oldCommandLine := flag.CommandLine
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flag.CommandLine = fs
	defer func() {
		flag.CommandLine = oldCommandLine
	}()

	flags := AddFlags()
	if err := flag.CommandLine.Parse([]string{
		"-p",
		"--json",
		"--model", "openai/gpt-5.5",
		"--reasoning=false",
		"--agent", "reviewer",
		"--service-priority", "fast",
		"hello",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !flags.PrintMode() {
		t.Fatal("PrintMode() = false, want true")
	}
	if !flags.JSONOutput() {
		t.Fatal("JSONOutput() = false, want true")
	}

	req := buildPrintPromptRequest(&config.Config{Model: "default/model"}, flags, "hello")
	if req.Model != "openai/gpt-5.5" {
		t.Fatalf("Model = %q, want openai/gpt-5.5", req.Model)
	}
	if req.Reasoning != "" {
		t.Fatalf("Reasoning = %q, want empty", req.Reasoning)
	}
	if req.SubagentType != "reviewer" {
		t.Fatalf("SubagentType = %q, want reviewer", req.SubagentType)
	}
	if req.ServiceTier != "fast" {
		t.Fatalf("ServiceTier = %q, want fast", req.ServiceTier)
	}
}

func TestRunPrintTurnLoop_CollectsFinalText(t *testing.T) {
	s := &printTestSession{
		chunks: []message.MessageChunk{
			message.TextDeltaChunk{Delta: "hello"},
			message.TextDeltaChunk{Delta: " world"},
		},
	}
	out, err := runPrintTurnLoop(context.Background(), s, "thread", agent.PromptRequest{})
	if err != nil {
		t.Fatalf("runPrintTurnLoop() error = %v", err)
	}
	if out != "hello world" {
		t.Fatalf("output = %q, want %q", out, "hello world")
	}
}

func TestCollectPrintOutput_ReturnsStreamError(t *testing.T) {
	wantErr := errors.New("boom")
	seq := func(yield func(message.MessageChunk, error) bool) {
		yield(nil, wantErr)
	}
	var out strings.Builder
	err := collectPrintOutput(context.Background(), seq, &out, newToolRenderState())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestCollectPrintJSONL_WritesOneChunkPerLine(t *testing.T) {
	seq := func(yield func(message.MessageChunk, error) bool) {
		yield(message.TextDeltaChunk{ID: "t1", Delta: "hello"}, nil)
		yield(message.ReasoningDeltaChunk{ID: "r1", Delta: "thinking"}, nil)
	}
	var out bytes.Buffer
	if err := collectPrintJSONL(context.Background(), seq, &out); err != nil {
		t.Fatalf("collectPrintJSONL() error = %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2: %q", len(lines), out.String())
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal first line: %v", err)
	}
	if first["type"] != "text-delta" || first["delta"] != "hello" {
		t.Fatalf("first line = %#v, want text-delta hello", first)
	}

	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("unmarshal second line: %v", err)
	}
	if second["type"] != "reasoning-delta" || second["delta"] != "thinking" {
		t.Fatalf("second line = %#v, want reasoning-delta thinking", second)
	}
}

func TestRunPrintJSONLTurnLoop_StreamsEvents(t *testing.T) {
	s := &printTestSession{
		chunks: []message.MessageChunk{
			message.TextDeltaChunk{Delta: "hello"},
			message.ToolInputStartChunk{ToolCallID: "tc1", ToolName: "read"},
		},
	}
	var out bytes.Buffer
	if err := runPrintJSONLTurnLoop(context.Background(), s, "thread", agent.PromptRequest{}, &out); err != nil {
		t.Fatalf("runPrintJSONLTurnLoop() error = %v", err)
	}
	if !strings.Contains(out.String(), `"type":"text-delta"`) {
		t.Fatalf("JSONL output missing text event: %q", out.String())
	}
	if !strings.Contains(out.String(), `"type":"tool-input-start"`) {
		t.Fatalf("JSONL output missing tool event: %q", out.String())
	}
}

type printTestSession struct {
	testSession
	chunks []message.MessageChunk
}

func (s *printTestSession) Prompt(context.Context, string, agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	return func(yield func(message.MessageChunk, error) bool) {
		for _, chunk := range s.chunks {
			if !yield(chunk, nil) {
				return
			}
		}
	}, nil
}

func (s *printTestSession) PendingQuestion(context.Context, string) (*agent.PendingQuestion, error) {
	return nil, nil
}

func (s *printTestSession) SubmitAnswer(context.Context, string, string, api.AnswerQuestionRequest) error {
	return nil
}
