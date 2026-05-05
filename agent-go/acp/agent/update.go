package agent

import (
	"encoding/json"

	"github.com/obot-platform/discobot/agent-go/acp/protocol"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func (m *sessionManager) applyUpdate(threadID string, notification protocol.SessionNotification) (message.MessageChunk, bool) {
	update := notification.Update.Variant()

	switch update := update.(type) {
	case protocol.SessionUpdateSessionInfoUpdate:
		return m.updateThreadSessionInfo(threadID, notification.SessionID, update.SessionInfoUpdate)
	case protocol.SessionUpdateConfigOptionUpdate:
		m.updateThreadConfigOptions(threadID, update.ConfigOptions)
	}
	return nil, false
}

func (m *sessionManager) updateThreadSessionInfo(threadID string, sessionID protocol.SessionID, update protocol.SessionInfoUpdate) (message.MessageChunk, bool) {
	session, stored := m.loadStoredSession(threadID)
	if state, ok := m.state.Get(threadID); ok {
		if state.SessionID != sessionID {
			return nil, false
		}
		session.SessionID = state.SessionID
	} else if stored {
		if session.SessionID != sessionID {
			return nil, false
		}
	} else {
		session = protocol.SessionInfo{
			Cwd:       m.cwd,
			SessionID: sessionID,
		}
	}

	if update.Meta != nil {
		session.Meta = update.Meta
	}
	if update.Title != nil {
		session.Title = update.Title
	}
	if update.UpdatedAt != nil {
		session.UpdatedAt = update.UpdatedAt
	}
	m.state.UpdateSessionID(threadID, sessionID)

	before, _ := m.store.LoadConfig(threadID)
	if err := m.saveThreadSession(threadID, session, nil); err != nil {
		return nil, false
	}
	after, err := m.store.LoadConfig(threadID)
	if err != nil || before.Name == after.Name {
		return nil, false
	}
	return thread.UpdateChunkFromConfig(threadID, after), true
}

func (m *sessionManager) updateThreadConfigOptions(threadID string, options []protocol.SessionConfigOption) {
	cfg, err := m.store.LoadConfig(threadID)
	if err != nil {
		return
	}
	cfg.Metadata.ACPSession.ConfigOptions = make([]json.RawMessage, 0, len(options))
	for _, option := range options {
		cfg.Metadata.ACPSession.ConfigOptions = append(cfg.Metadata.ACPSession.ConfigOptions, option.Raw())
	}
	_ = m.store.SaveConfig(threadID, cfg)
}
