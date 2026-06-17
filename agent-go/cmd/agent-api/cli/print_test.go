package cli

import (
	"context"
	"errors"
	"flag"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
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
