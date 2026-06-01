package sync

import (
	"context"

	serviceclient "github.com/obot-platform/discobot/server/client"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func (p projectEventProcessor) processThreadEvent(ctx context.Context, event serviceclient.ProjectStreamEvent) bool {
	message, ok := threadMessageFromEvent(event)
	if !ok {
		return false
	}
	cache, ok := cloneThreadMessageState(p.runtime.cache, message.sessionID, message.threadID)
	if !ok {
		return true
	}
	if applyThreadMessage(&cache, message) {
		p.runtime.cache = cache
		p.manager.publishProjectThread(ctx, p.runtime.project, p.runtime.cache, message.sessionID, message.threadID)
		return true
	}
	p.runtime.cache = cache
	return true
}

func cloneThreadMessageState(cache state.ProjectData, sessionID string, threadID string) (state.ProjectData, bool) {
	sessionData, ok := cache.Session[sessionID]
	if !ok || sessionData.Thread == nil {
		return cache, false
	}
	threadData, ok := sessionData.Thread[threadID]
	if !ok || threadData.Thread.ID == "" {
		return cache, false
	}

	sessions := make(map[string]state.SessionData, len(cache.Session))
	for id, data := range cache.Session {
		sessions[id] = data
	}
	cache.Session = sessions

	threads := make(map[string]state.ThreadData, len(sessionData.Thread))
	for id, data := range sessionData.Thread {
		threads[id] = data
	}
	sessionData.Thread = threads

	threadData.Messages = cloneMessagesForUpdate(threadData.Messages)
	threadData.PendingMessages = cloneMessagesForUpdate(threadData.PendingMessages)
	sessionData.Thread[threadID] = threadData
	cache.Session[sessionID] = sessionData
	return cache, true
}
