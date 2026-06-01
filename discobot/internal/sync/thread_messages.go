package sync

import (
	serverapi "github.com/obot-platform/discobot/server/api"
	serviceclient "github.com/obot-platform/discobot/server/client"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

type threadMessage struct {
	sessionID string
	threadID  string
	event     serverapi.ChatStreamEventName
	message   serverapi.Message
	chunk     serverapi.MessageChunk
}

func threadMessageFromEvent(event serviceclient.ProjectStreamEvent) (threadMessage, bool) {
	chatEvent, ok := event.(serverapi.ChatStreamEvent)
	if !ok {
		return threadMessage{}, false
	}
	message := threadMessage{
		sessionID: chatEvent.SessionID,
		threadID:  chatEvent.ThreadID,
		event:     chatEvent.Event,
	}
	switch chatEvent.Event {
	case serverapi.HistoryStart, serverapi.HistoryEnd:
		return message, true
	case serverapi.HistoryMessage:
		if data, ok := chatEvent.Data.(serverapi.Message); ok {
			message.message = data
			return message, true
		}
	case serverapi.Chunk:
		if data, ok := chatEvent.Data.(serverapi.MessageChunk); ok {
			message.chunk = data
			return message, true
		}
	}
	return threadMessage{}, false
}

func applyThreadMessage(cache *state.ProjectData, message threadMessage) bool {
	sessionData, ok := cache.Session[message.sessionID]
	if !ok || sessionData.Thread == nil {
		return false
	}
	threadData, ok := sessionData.Thread[message.threadID]
	if !ok || threadData.Thread.ID == "" {
		return false
	}

	changed := true
	switch message.event {
	case serverapi.HistoryStart:
		threadData.PendingHistory = true
		threadData.PendingMessages = nil
		changed = false
	case serverapi.HistoryEnd:
		if !threadData.PendingHistory {
			return false
		}
		threadData.Messages = threadData.PendingMessages
		threadData.PendingHistory = false
		threadData.PendingMessages = nil
	case serverapi.HistoryMessage:
		if threadData.PendingHistory {
			threadData.PendingMessages = append(threadData.PendingMessages, message.message)
			changed = false
		} else {
			threadData.Messages = upsertMessage(threadData.Messages, message.message, "")
		}
	case serverapi.Chunk:
		if threadData.PendingHistory {
			threadData.PendingMessages = applyChunk(threadData.PendingMessages, message.chunk)
			changed = false
		} else {
			threadData.Messages = applyChunk(threadData.Messages, message.chunk)
		}
	default:
		return false
	}
	sessionData.Thread[message.threadID] = threadData
	cache.Session[message.sessionID] = sessionData
	return changed
}
