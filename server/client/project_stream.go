package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	serverapi "github.com/boeing-ai-gateway/discboeing/server/api"
)

// ProjectStreamEvent is a typed event received from the project websocket.
type ProjectStreamEvent any

type ProjectHistoryEvent struct {
	Event string
	Data  any
}

type projectStreamSocketRequest struct {
	Type      string              `json:"type"`
	Stream    string              `json:"stream"`
	SessionID serverapi.SessionID `json:"sessionId,omitempty"`
	ThreadID  serverapi.ThreadID  `json:"threadId,omitempty"`
	ServiceID serverapi.ServiceID `json:"serviceId,omitempty"`
}

// WebSocketURL resolves a server websocket path against the client's base URL.
func (c *Client) WebSocketURL(path string) string {
	u, err := url.Parse(c.Server)
	if err != nil {
		return path
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = ""
	return u.String()
}

func projectPath(projectID, suffix string) string {
	return "/api/projects/" + url.PathEscape(projectID) + suffix
}

func projectStreamEvents(msg any) []ProjectStreamEvent {
	switch m := msg.(type) {
	case serverapi.ProjectStreamSubscribedEvent:
		return []ProjectStreamEvent{m}
	case serverapi.ProjectStreamUnsubscribedEvent:
		return []ProjectStreamEvent{m}
	case serverapi.ProjectStreamCompleteEvent:
		return []ProjectStreamEvent{m}
	case serverapi.ProjectStreamErrorEvent:
		return []ProjectStreamEvent{m}
	case serverapi.ProjectEventsStreamMessage:
		event, err := parseProjectEventMessage(m.Event, m.Data)
		if err != nil {
			return []ProjectStreamEvent{serverapi.ProjectStreamErrorEvent{Stream: serverapi.ProjectStreamTypeProjectEvents, Error: err.Error()}}
		}
		return []ProjectStreamEvent{event}
	case serverapi.ChatStreamMessage:
		event := serverapi.ChatStreamEvent{SessionID: m.SessionID, ThreadID: m.ThreadID, Event: m.Event, ID: m.ID}
		switch m.Event {
		case serverapi.ChatStreamEventNameHistoryMessage:
			var message serverapi.Message
			if err := json.Unmarshal([]byte(m.Data), &message); err != nil {
				return []ProjectStreamEvent{serverapi.ProjectStreamErrorEvent{Stream: serverapi.ProjectStreamTypeChat, Error: fmt.Sprintf("decode history message: %v", err)}}
			}
			event.Data = message
		case serverapi.ChatStreamEventNameChunk:
			chunk, err := serverapi.UnmarshalMessageChunk([]byte(m.Data))
			if err != nil {
				return []ProjectStreamEvent{serverapi.ProjectStreamErrorEvent{Stream: serverapi.ProjectStreamTypeChat, Error: fmt.Sprintf("decode chat chunk: %v", err)}}
			}
			event.Data = chunk
		}
		return []ProjectStreamEvent{event}
	case serverapi.ServiceOutputEvent:
		return []ProjectStreamEvent{m}
	case UnknownProjectStreamSocketMessage:
		return []ProjectStreamEvent{m}
	}
	return nil
}

func parseProjectEventMessage(eventName serverapi.ProjectEventName, data json.RawMessage) (ProjectStreamEvent, error) {
	switch string(eventName) {
	case "history-start", "history-end":
		return ProjectHistoryEvent{Event: string(eventName)}, nil
	}

	if eventName == serverapi.ProjectEventNameConnected {
		var connected struct {
			ProjectID string `json:"projectId"`
		}
		if err := json.Unmarshal(data, &connected); err != nil {
			return nil, fmt.Errorf("decode connected event: %w", err)
		}
		return serverapi.ProjectConnectedEvent{ProjectID: connected.ProjectID}, nil
	}

	var envelope struct {
		ID        string                     `json:"id"`
		Seq       int64                      `json:"seq"`
		Type      serverapi.ProjectEventName `json:"type"`
		Timestamp time.Time                  `json:"timestamp"`
		Data      json.RawMessage            `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode project event %q: %w", eventName, err)
	}
	switch envelope.Type {
	case serverapi.ProjectEventNameSessionUpdated:
		var payload serverapi.Session
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode session_updated data: %w", err)
		}
		return serverapi.SessionUpdatedEvent{ID: envelope.ID, Seq: envelope.Seq, Type: envelope.Type, Timestamp: envelope.Timestamp, Data: payload}, nil
	case serverapi.ProjectEventNameThreadUpdated:
		var payload serverapi.ThreadUpdatedData
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode thread_updated data: %w", err)
		}
		return serverapi.ThreadUpdatedEvent{ID: envelope.ID, Seq: envelope.Seq, Type: envelope.Type, Timestamp: envelope.Timestamp, Data: payload}, nil
	case serverapi.ProjectEventNameWorkspaceUpdated:
		var payload serverapi.Workspace
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode workspace_updated data: %w", err)
		}
		return serverapi.WorkspaceUpdatedEvent{ID: envelope.ID, Seq: envelope.Seq, Type: envelope.Type, Timestamp: envelope.Timestamp, Data: payload}, nil
	case serverapi.ProjectEventNameStartupTaskUpdated:
		var payload serverapi.StartupTask
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode startup_task_updated data: %w", err)
		}
		return serverapi.StartupTaskUpdatedEvent{ID: envelope.ID, Seq: envelope.Seq, Type: envelope.Type, Timestamp: envelope.Timestamp, Data: payload}, nil
	default:
		return serverapi.UnknownProjectEvent{ID: envelope.ID, Seq: envelope.Seq, Type: envelope.Type, Timestamp: envelope.Timestamp, Data: envelope.Data}, nil
	}
}
