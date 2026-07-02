package clisession

import (
	"context"
	"iter"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

type Session interface {
	WorkspaceRoot() string
	Close()
	ListCommands(context.Context) ([]api.Command, error)
	ListThreads(context.Context) ([]api.Thread, error)
	GetThread(context.Context, string) (api.Thread, error)
	UpdateThread(context.Context, string, api.UpdateThreadRequest) (api.Thread, error)
	Messages(context.Context, string) ([]message.UIMessage, error)
	HasInterruptedTurn(context.Context, string) (bool, error)
	PendingQuestion(context.Context, string) (*agent.PendingQuestion, error)
	SubmitAnswer(context.Context, string, string, api.AnswerQuestionRequest) error
	Prompt(context.Context, string, agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error)
	Resume(context.Context, string, agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error)
}
