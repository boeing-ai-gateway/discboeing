package thread

import (
	"fmt"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

func UpdateChunkFromConfig(threadID string, cfg Config) message.ThreadUpdateChunk {
	return UpdateChunkFromInfo(Info{
		ID:            threadID,
		Name:          cfg.Name,
		CWD:           cfg.CWD,
		LastMessage:   cfg.LastMessage,
		ErrorMessage:  cfg.ErrorMessage,
		Model:         cfg.Model,
		Reasoning:     string(cfg.Reasoning),
		ServiceTier:   cfg.ServiceTier,
		State:         cfg.LastTurnState,
		TokenUsage:    cfg.TokenUsage,
		ActiveCommand: cfg.ActiveCommand,
		Metadata:      cfg.Metadata.RawMessage(),
	})
}

func UpdateChunkFromInfo(info Info) message.ThreadUpdateChunk {
	return message.ThreadUpdateChunk{
		Data: message.ThreadUpdateData{
			Thread: message.ThreadUpdateInfo{
				ID:           info.ID,
				Name:         info.Name,
				CWD:          info.CWD,
				Phase:        info.Phase,
				LastMessage:  info.LastMessage,
				ErrorMessage: info.ErrorMessage,
				Model:        info.Model,
				Reasoning:    info.Reasoning,
				ServiceTier:  info.ServiceTier,
				State:        string(info.State),
				TokenUsage: message.TokenUsageInfo{
					Total:           info.TokenUsage.Total,
					LastStep:        info.TokenUsage.LastStep,
					LastTurn:        info.TokenUsage.LastTurn,
					ModelMaxTokens:  info.TokenUsage.ModelMaxTokens,
					MaxOutputTokens: info.TokenUsage.MaxOutputTokens,
					Prices:          info.TokenUsage.Prices,
				},
				ActiveCommand: info.ActiveCommand,
				Metadata:      info.Metadata,
			},
		},
	}
}

func UpdateChunkFromStore(store *Store, threadID string) (message.ThreadUpdateChunk, error) {
	info, err := store.GetThreadInfo(threadID)
	if err != nil {
		return message.ThreadUpdateChunk{}, err
	}
	return UpdateChunkFromInfo(info), nil
}

func YieldThreadUpdate(
	yield func(message.MessageChunk, error) bool,
	store *Store,
	threadID string,
) bool {
	chunk, err := UpdateChunkFromStore(store, threadID)
	if err != nil {
		return yield(nil, fmt.Errorf("load thread config: %w", err))
	}
	return yield(chunk, nil)
}
