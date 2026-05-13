package agent

import (
	"context"

	discobotagent "github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func (a *Agent) ListThreadInfos() ([]discobotagent.ThreadInfo, error) {
	infos, err := a.store.ListThreadInfos()
	if err != nil {
		return nil, err
	}
	result := make([]discobotagent.ThreadInfo, 0, len(infos))
	for _, info := range infos {
		result = append(result, threadInfoToAgent(info))
	}
	return result, nil
}

func (a *Agent) GetThreadInfo(threadID string) (discobotagent.ThreadInfo, error) {
	info, err := a.store.GetThreadInfo(threadID)
	if err != nil {
		return discobotagent.ThreadInfo{}, err
	}
	return threadInfoToAgent(info), nil
}

func (a *Agent) CreateThread(_ context.Context, req discobotagent.CreateThreadRequest) (discobotagent.ThreadInfo, error) {
	info, err := a.store.CreateThreadInfo(a.cwd, thread.CreateThreadRequest(req))
	if err != nil {
		return discobotagent.ThreadInfo{}, err
	}
	return threadInfoToAgent(info), nil
}

func (a *Agent) UpdateThread(_ context.Context, threadID string, req discobotagent.UpdateThreadRequest) (discobotagent.ThreadInfo, error) {
	info, err := a.store.UpdateThreadInfo(threadID, thread.UpdateThreadRequest(req))
	if err != nil {
		return discobotagent.ThreadInfo{}, err
	}
	return threadInfoToAgent(info), nil
}

func threadInfoToAgent(info thread.Info) discobotagent.ThreadInfo {
	return discobotagent.ThreadInfo{
		ID:              info.ID,
		Name:            info.Name,
		CWD:             info.CWD,
		LastMessage:     info.LastMessage,
		ErrorMessage:    info.ErrorMessage,
		Model:           info.Model,
		Reasoning:       info.Reasoning,
		ServiceTier:     info.ServiceTier,
		State:           discobotagent.ThreadState(info.State),
		PendingQuestion: info.PendingQuestion,
		ActiveCommand:   info.ActiveCommand,
		Metadata:        info.Metadata,
	}
}

func (a *Agent) DeleteThread(_ context.Context, threadID string) error {
	a.Cancel(threadID)
	return a.store.DeleteThreadInfo(threadID)
}
