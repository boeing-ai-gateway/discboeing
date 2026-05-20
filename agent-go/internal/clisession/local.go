package clisession

import (
	"context"
	"iter"
	"os"
	"strings"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type Local struct {
	agent *agentimpl.DefaultAgent
	store *thread.Store
	cwd   string
}

func NewLocal(a *agentimpl.DefaultAgent, store *thread.Store, cwd string) *Local {
	return &Local{agent: a, store: store, cwd: cwd}
}

func (s *Local) WorkspaceRoot() string { return s.cwd }
func (s *Local) Close()                { s.agent.Close() }

func (s *Local) ListCommands(_ context.Context) ([]agent.Command, error) {
	return s.agent.ListCommands()
}

func (s *Local) ListThreads(_ context.Context) ([]api.Thread, error) {
	infos, err := s.agent.ListThreadInfos()
	if err != nil {
		return nil, err
	}
	threads := make([]api.Thread, 0, len(infos))
	for _, info := range infos {
		threads = append(threads, api.Thread{
			ID:              info.ID,
			Name:            strings.TrimSpace(info.Name),
			CWD:             strings.TrimSpace(info.CWD),
			LastMessage:     strings.TrimSpace(info.LastMessage),
			ErrorMessage:    strings.TrimSpace(info.ErrorMessage),
			Model:           info.Model,
			Reasoning:       info.Reasoning,
			ServiceTier:     info.ServiceTier,
			State:           string(info.State),
			PendingQuestion: info.PendingQuestion,
			ActiveCommand:   strings.TrimSpace(info.ActiveCommand),
			Metadata:        info.Metadata,
		})
	}
	return threads, nil
}

func (s *Local) GetThread(_ context.Context, threadID string) (api.Thread, error) {
	info, err := s.agent.GetThreadInfo(threadID)
	if err != nil {
		if os.IsNotExist(err) {
			return api.Thread{}, ErrNotFound
		}
		return api.Thread{}, err
	}
	return api.Thread{
		ID:              info.ID,
		Name:            strings.TrimSpace(info.Name),
		CWD:             strings.TrimSpace(info.CWD),
		LastMessage:     strings.TrimSpace(info.LastMessage),
		ErrorMessage:    strings.TrimSpace(info.ErrorMessage),
		Model:           info.Model,
		Reasoning:       info.Reasoning,
		ServiceTier:     info.ServiceTier,
		State:           string(info.State),
		PendingQuestion: info.PendingQuestion,
		ActiveCommand:   strings.TrimSpace(info.ActiveCommand),
		Metadata:        info.Metadata,
	}, nil
}

func (s *Local) UpdateThread(_ context.Context, threadID string, req api.UpdateThreadRequest) (api.Thread, error) {
	name := strings.TrimSpace(req.Name)
	if _, err := s.agent.UpdateThread(context.Background(), threadID, agent.UpdateThreadRequest{Name: &name}); err != nil {
		return api.Thread{}, err
	}
	return s.GetThread(context.Background(), threadID)
}

func (s *Local) Messages(_ context.Context, threadID string) ([]message.UIMessage, error) {
	return s.agent.Messages(threadID, "")
}

func (s *Local) HasInterruptedTurn(_ context.Context, threadID string) (bool, error) {
	return s.agent.HasInterruptedTurn(threadID)
}

func (s *Local) PendingQuestion(_ context.Context, threadID string) (*agent.PendingQuestion, error) {
	return s.agent.PendingQuestion(threadID)
}

func (s *Local) SubmitAnswer(_ context.Context, threadID, approvalID string, req api.AnswerQuestionRequest) error {
	return s.agent.SubmitAnswer(threadID, approvalID, req)
}

func (s *Local) Prompt(ctx context.Context, threadID string, req agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	return s.agent.Prompt(ctx, threadID, req), nil
}

func (s *Local) Resume(ctx context.Context, threadID string, req agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	resumed, err := s.agent.Resume(ctx, threadID, req)
	if err != nil {
		return nil, err
	}
	return resumed.Stream, nil
}

var ErrNotFound = &apiError{status: 404, message: "not found"}

type apiError struct {
	status  int
	message string
}

func (e *apiError) Error() string { return e.message }
