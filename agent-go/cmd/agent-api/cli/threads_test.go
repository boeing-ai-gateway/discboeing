package cli

import (
	"context"
	"io"
	"iter"
	"os"
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/clisession"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/config"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

func TestSelectInitialThreadID_ForceNewThread(t *testing.T) {
	cfg := &config.Config{SessionID: "explicit-session"}

	threadID := selectInitialThreadID(cfg, true, "")
	if threadID == cfg.SessionID {
		t.Fatalf("expected force-new to ignore explicit session id, got %q", threadID)
	}
	if !strings.HasPrefix(threadID, "thread-") {
		t.Fatalf("expected generated thread ID with prefix thread-, got %q", threadID)
	}
}

func TestSelectInitialThreadID_UsesExplicitSessionIDByDefault(t *testing.T) {
	cfg := &config.Config{SessionID: "explicit-session"}

	threadID := selectInitialThreadID(cfg, false, "")
	if threadID != cfg.SessionID {
		t.Fatalf("expected explicit session id %q, got %q", cfg.SessionID, threadID)
	}
}

func TestSelectInitialThreadID_UsesResumeFlagBeforeSessionID(t *testing.T) {
	cfg := &config.Config{SessionID: "explicit-session"}

	threadID := selectInitialThreadID(cfg, false, "resumed-thread")
	if threadID != "resumed-thread" {
		t.Fatalf("expected resume flag thread id %q, got %q", "resumed-thread", threadID)
	}
}

func TestSelectInitialThreadID_DefaultsToFreshThread(t *testing.T) {
	cfg := &config.Config{SessionID: "default"}

	threadID := selectInitialThreadID(cfg, false, "")
	if !strings.HasPrefix(threadID, "thread-") {
		t.Fatalf("expected generated thread ID with prefix thread-, got %q", threadID)
	}
}

func TestPrintThreadHistory_UsesPromptStyleForUserAndInlineAssistant(t *testing.T) {
	threadID := "thread-1"

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutReader.Close()

	oldStdout := os.Stdout
	oldNoColor := noColor
	os.Stdout = stdoutWriter
	noColor = true
	defer func() {
		os.Stdout = oldStdout
		noColor = oldNoColor
	}()

	session := &threadHistorySession{messages: map[string][]message.UIMessage{
		threadID: {
			{ID: "msg-1", Role: "user", Parts: []message.UIPart{message.UITextPart{Text: "hello"}}},
			{ID: "msg-2", Role: "assistant", Parts: []message.UIPart{message.UITextPart{Text: "world"}}},
		},
	}}
	if !printThreadHistory(context.Background(), session, threadID) {
		t.Fatal("expected printable history")
	}

	if err := stdoutWriter.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatal(err)
	}

	got := string(output)
	if strings.Contains(got, "[User]") || strings.Contains(got, "[Assistant]") {
		t.Fatalf("expected prompt-style history without role labels, got %q", got)
	}
	for _, want := range []string{"> hello", "world"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected history output to contain %q, got %q", want, got)
		}
	}
}

type threadHistorySession struct {
	messages map[string][]message.UIMessage
}

func (s *threadHistorySession) WorkspaceRoot() string { return "" }
func (s *threadHistorySession) Close()                {}
func (s *threadHistorySession) ListCommands(context.Context) ([]api.Command, error) {
	return nil, nil
}
func (s *threadHistorySession) ListThreads(context.Context) ([]api.Thread, error) {
	return nil, nil
}
func (s *threadHistorySession) GetThread(context.Context, string) (api.Thread, error) {
	return api.Thread{}, clisession.ErrNotFound
}
func (s *threadHistorySession) UpdateThread(context.Context, string, api.UpdateThreadRequest) (api.Thread, error) {
	return api.Thread{}, nil
}
func (s *threadHistorySession) Messages(_ context.Context, threadID string) ([]message.UIMessage, error) {
	return s.messages[threadID], nil
}
func (s *threadHistorySession) HasInterruptedTurn(context.Context, string) (bool, error) {
	return false, nil
}
func (s *threadHistorySession) PendingQuestion(context.Context, string) (*agent.PendingQuestion, error) {
	return nil, nil
}
func (s *threadHistorySession) SubmitAnswer(context.Context, string, string, api.AnswerQuestionRequest) error {
	return nil
}
func (s *threadHistorySession) Prompt(context.Context, string, agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	return nil, nil
}
func (s *threadHistorySession) Resume(context.Context, string, agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	return nil, nil
}
