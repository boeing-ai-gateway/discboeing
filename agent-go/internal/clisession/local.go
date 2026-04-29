package clisession

import (
	"context"
	"iter"
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
	threadIDs, err := s.agent.ListThreads()
	if err != nil {
		return nil, err
	}
	threads := make([]api.Thread, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		cfg, _ := s.store.LoadConfig(threadID)
		pending := false
		if state, err := s.store.LoadTurnState(threadID); err == nil && state != nil {
			pending = state.Phase == thread.PhaseWaitingForAnswer
		}
		mode := "build"
		if strings.EqualFold(strings.TrimSpace(cfg.Mode.Value), "plan") {
			mode = "plan"
		}
		state := string(cfg.LastTurnState)
		if interrupted, err := s.agent.HasInterruptedTurn(threadID); err == nil && interrupted {
			state = string(thread.StateInterrupted)
		}
		threads = append(threads, api.Thread{
			ID:              threadID,
			Name:            strings.TrimSpace(cfg.Name),
			CWD:             strings.TrimSpace(cfg.CWD),
			LastMessage:     strings.TrimSpace(cfg.LastMessage),
			ErrorMessage:    strings.TrimSpace(cfg.ErrorMessage),
			Model:           cfg.Model,
			Reasoning:       string(cfg.Reasoning),
			Mode:            mode,
			State:           state,
			PendingQuestion: pending,
			ActiveCommand:   strings.TrimSpace(cfg.ActiveCommand),
		})
	}
	return threads, nil
}

func (s *Local) GetThread(ctx context.Context, threadID string) (api.Thread, error) {
	threads, err := s.ListThreads(ctx)
	if err != nil {
		return api.Thread{}, err
	}
	for _, thread := range threads {
		if thread.ID == threadID {
			return thread, nil
		}
	}
	return api.Thread{}, ErrNotFound
}

func (s *Local) UpdateThread(_ context.Context, threadID string, req api.UpdateThreadRequest) (api.Thread, error) {
	cfg, err := s.store.LoadConfig(threadID)
	if err != nil {
		return api.Thread{}, err
	}
	if trimmedName := strings.TrimSpace(req.Name); trimmedName != "" {
		cfg.Name = trimmedName
		cfg.NameSource = thread.ThreadNameSourceUser
	}
	if trimmedCWD := strings.TrimSpace(req.CWD); trimmedCWD != "" {
		cfg.CWD = trimmedCWD
	}
	if err := s.store.SaveConfig(threadID, cfg); err != nil {
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
