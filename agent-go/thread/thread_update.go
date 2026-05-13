package thread

import (
	"fmt"

	"github.com/obot-platform/discobot/agent-go/message"
)

func UpdateChunkFromConfig(threadID string, cfg Config) message.ThreadUpdateChunk {
	return message.ThreadUpdateChunk{
		Data: message.ThreadUpdateData{
			Thread: message.ThreadUpdateInfo{
				ID:            threadID,
				Name:          cfg.Name,
				CWD:           cfg.CWD,
				LastMessage:   cfg.LastMessage,
				ErrorMessage:  cfg.ErrorMessage,
				Model:         cfg.Model,
				Reasoning:     string(cfg.Reasoning),
				ServiceTier:   cfg.ServiceTier,
				State:         string(cfg.LastTurnState),
				ActiveCommand: cfg.ActiveCommand,
				Metadata:      cfg.Metadata.RawMessage(),
			},
		},
	}
}

func YieldThreadUpdate(
	yield func(message.MessageChunk, error) bool,
	store *Store,
	threadID string,
) bool {
	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		return yield(nil, fmt.Errorf("load thread config: %w", err))
	}
	return yield(UpdateChunkFromConfig(threadID, cfg), nil)
}
