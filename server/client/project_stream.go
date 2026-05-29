package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// ProjectStreamChat is the chat completion stream.
	ProjectStreamChat ProjectStreamType = "chat"
	// ProjectStreamService is a session service output stream.
	ProjectStreamService ProjectStreamType = "service"
	// ProjectStreamProjectEvents is the project-level events stream.
	ProjectStreamProjectEvents ProjectStreamType = "project-events"
)

const (
	// ChatStreamEventHistoryStart begins replaying persisted chat history.
	ChatStreamEventHistoryStart ChatStreamEventName = "history-start"
	// ChatStreamEventHistoryMessage is one persisted chat message.
	ChatStreamEventHistoryMessage ChatStreamEventName = "history-message"
	// ChatStreamEventHistoryEnd completes persisted chat history replay.
	ChatStreamEventHistoryEnd ChatStreamEventName = "history-end"
	// ChatStreamEventChunk is a live UI message stream chunk.
	ChatStreamEventChunk ChatStreamEventName = "chunk"
	// ChatStreamEventPing is a chat stream keepalive.
	ChatStreamEventPing ChatStreamEventName = "ping"
)

const (
	// ProjectEventConnected is emitted after subscribing to project events.
	ProjectEventConnected ProjectEventName = "connected"
	// ProjectEventSessionUpdated indicates a session changed.
	ProjectEventSessionUpdated ProjectEventName = "session_updated"
	// ProjectEventThreadUpdated indicates thread metadata or activity changed.
	ProjectEventThreadUpdated ProjectEventName = "thread_updated"
	// ProjectEventWorkspaceUpdated indicates workspace state changed.
	ProjectEventWorkspaceUpdated ProjectEventName = "workspace_updated"
	// ProjectEventStartupTaskUpdated indicates startup task progress changed.
	ProjectEventStartupTaskUpdated ProjectEventName = "startup_task_updated"
)

// ProjectStreamType identifies one websocket sub-stream.
type ProjectStreamType string

// ChatStreamEventName identifies chat stream event names.
type ChatStreamEventName string

// ProjectEventName identifies project-events stream event names.
type ProjectEventName string

// ProjectStreamOptions describes which project websocket streams to subscribe to.
type ProjectStreamOptions struct {
	ProjectEvents *ProjectEventsSubscriptionOptions
	Chat          *ChatStreamSubscriptionOptions
	Service       *ServiceStreamSubscriptionOptions
}

// ProjectEventsSubscriptionOptions configures the project-events stream.
type ProjectEventsSubscriptionOptions struct {
	AfterID string
}

// ChatStreamSubscriptionOptions configures one chat stream subscription.
type ChatStreamSubscriptionOptions struct {
	SessionID   string
	ThreadID    string
	Replay      bool
	LastEventID string
}

// ServiceStreamSubscriptionOptions configures one service output subscription.
type ServiceStreamSubscriptionOptions struct {
	SessionID string
	ServiceID string
}

// ProjectStreamEvent is a typed event received from the project websocket.
type ProjectStreamEvent interface {
	projectStreamEvent()
}

// ProjectStreamSubscribedEvent is emitted after the server accepts a subscription.
type ProjectStreamSubscribedEvent struct {
	Stream    ProjectStreamType
	SessionID string
	ThreadID  string
	ServiceID string
	Replay    bool
}

func (ProjectStreamSubscribedEvent) projectStreamEvent() {}

// ProjectStreamUnsubscribedEvent is emitted after the server unsubscribes.
type ProjectStreamUnsubscribedEvent struct {
	Stream    ProjectStreamType
	SessionID string
	ThreadID  string
	ServiceID string
}

func (ProjectStreamUnsubscribedEvent) projectStreamEvent() {}

// ProjectStreamCompleteEvent is emitted when a sub-stream completes.
type ProjectStreamCompleteEvent struct {
	Stream    ProjectStreamType
	SessionID string
	ThreadID  string
	ServiceID string
}

func (ProjectStreamCompleteEvent) projectStreamEvent() {}

// ProjectStreamErrorEvent is emitted when the websocket or a sub-stream fails.
type ProjectStreamErrorEvent struct {
	Stream    ProjectStreamType
	SessionID string
	ThreadID  string
	ServiceID string
	Error     string
}

func (ProjectStreamErrorEvent) projectStreamEvent() {}

// ChatStreamEvent is a typed chat stream event with raw JSON data.
type ChatStreamEvent struct {
	SessionID string
	ThreadID  string
	Event     ChatStreamEventName
	Data      json.RawMessage
	ID        string
}

func (ChatStreamEvent) projectStreamEvent() {}

// ServiceOutputEvent is a typed service output stream event.
type ServiceOutputEvent struct {
	SessionID string
	ServiceID string
	Data      string
	ID        string
}

func (ServiceOutputEvent) projectStreamEvent() {}

// ProjectEventBase contains metadata shared by persisted project events.
type ProjectEventBase struct {
	ID        string
	Seq       int64
	Type      ProjectEventName
	Timestamp time.Time
	RawData   json.RawMessage
}

// ProjectConnectedEvent is emitted immediately after project-events subscribe.
type ProjectConnectedEvent struct {
	ProjectID string
}

func (ProjectConnectedEvent) projectStreamEvent() {}

// SessionUpdatedData is the payload for session_updated project events.
type SessionUpdatedData struct {
	SessionID     string `json:"sessionId"`
	SandboxStatus string `json:"sandboxStatus"`
	CommitStatus  string `json:"commitStatus,omitempty"`
}

// SessionUpdatedEvent is a typed session_updated project event.
type SessionUpdatedEvent struct {
	ProjectEventBase
	Data SessionUpdatedData
}

func (SessionUpdatedEvent) projectStreamEvent() {}

// ThreadUpdatedData is the payload for thread_updated project events.
type ThreadUpdatedData struct {
	SessionID string `json:"sessionId"`
	ThreadID  string `json:"threadId,omitempty"`
	Name      string `json:"name,omitempty"`
}

// ThreadUpdatedEvent is a typed thread_updated project event.
type ThreadUpdatedEvent struct {
	ProjectEventBase
	Data ThreadUpdatedData
}

func (ThreadUpdatedEvent) projectStreamEvent() {}

// WorkspaceUpdatedData is the payload for workspace_updated project events.
type WorkspaceUpdatedData struct {
	WorkspaceID string `json:"workspaceId"`
	Status      string `json:"status"`
}

// WorkspaceUpdatedEvent is a typed workspace_updated project event.
type WorkspaceUpdatedEvent struct {
	ProjectEventBase
	Data WorkspaceUpdatedData
}

func (WorkspaceUpdatedEvent) projectStreamEvent() {}

// StartupTaskUpdatedEvent is a typed startup_task_updated project event.
type StartupTaskUpdatedEvent struct {
	ProjectEventBase
	Data StartupTask
}

func (StartupTaskUpdatedEvent) projectStreamEvent() {}

// UnknownProjectEvent preserves project events unknown to this client version.
type UnknownProjectEvent struct {
	ProjectEventBase
	Data json.RawMessage
}

func (UnknownProjectEvent) projectStreamEvent() {}

type projectStreamSocketRequest struct {
	Type        string `json:"type"`
	Stream      string `json:"stream"`
	SessionID   string `json:"sessionId,omitempty"`
	ThreadID    string `json:"threadId,omitempty"`
	ServiceID   string `json:"serviceId,omitempty"`
	Replay      bool   `json:"replay,omitempty"`
	LastEventID string `json:"lastEventId,omitempty"`
	AfterID     string `json:"afterId,omitempty"`
}

type projectStreamSocketMessage interface {
	projectStreamSocketMessageType() string
}

type projectStreamSocketMessageJSON struct {
	Message projectStreamSocketMessage
}

type projectStreamSubscribedMessage struct {
	Stream    ProjectStreamType `json:"stream"`
	SessionID string            `json:"sessionId,omitempty"`
	ThreadID  string            `json:"threadId,omitempty"`
	ServiceID string            `json:"serviceId,omitempty"`
	Replay    bool              `json:"replay,omitempty"`
}

func (projectStreamSubscribedMessage) projectStreamSocketMessageType() string { return "subscribed" }

type projectStreamUnsubscribedMessage struct {
	Stream    ProjectStreamType `json:"stream"`
	SessionID string            `json:"sessionId,omitempty"`
	ThreadID  string            `json:"threadId,omitempty"`
	ServiceID string            `json:"serviceId,omitempty"`
}

func (projectStreamUnsubscribedMessage) projectStreamSocketMessageType() string {
	return "unsubscribed"
}

type projectStreamCompleteMessage struct {
	Stream    ProjectStreamType `json:"stream"`
	SessionID string            `json:"sessionId,omitempty"`
	ThreadID  string            `json:"threadId,omitempty"`
	ServiceID string            `json:"serviceId,omitempty"`
}

func (projectStreamCompleteMessage) projectStreamSocketMessageType() string { return "complete" }

type projectStreamErrorMessage struct {
	Stream    ProjectStreamType `json:"stream,omitempty"`
	SessionID string            `json:"sessionId,omitempty"`
	ThreadID  string            `json:"threadId,omitempty"`
	ServiceID string            `json:"serviceId,omitempty"`
	Error     string            `json:"error"`
}

func (projectStreamErrorMessage) projectStreamSocketMessageType() string { return "error" }

type projectEventsStreamMessage struct {
	Event ProjectEventName `json:"event"`
	Data  string           `json:"data,omitempty"`
	ID    string           `json:"id,omitempty"`
}

func (projectEventsStreamMessage) projectStreamSocketMessageType() string { return "event" }

type chatStreamMessage struct {
	SessionID string              `json:"sessionId"`
	ThreadID  string              `json:"threadId"`
	Event     ChatStreamEventName `json:"event"`
	Data      string              `json:"data,omitempty"`
	ID        string              `json:"id,omitempty"`
}

func (chatStreamMessage) projectStreamSocketMessageType() string { return "event" }

type serviceStreamMessage struct {
	SessionID string `json:"sessionId"`
	ServiceID string `json:"serviceId"`
	Data      string `json:"data,omitempty"`
	ID        string `json:"id,omitempty"`
}

func (serviceStreamMessage) projectStreamSocketMessageType() string { return "event" }

type unknownProjectStreamSocketMessage struct {
	Type   string            `json:"type"`
	Stream ProjectStreamType `json:"stream,omitempty"`
	Raw    json.RawMessage   `json:"-"`
}

func (unknownProjectStreamSocketMessage) projectStreamSocketMessageType() string { return "unknown" }

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

// WatchProjectStream subscribes to project websocket streams and returns typed
// events until ctx is canceled, the websocket closes, or an unrecoverable error
// occurs. Transport and server errors are delivered as ProjectStreamErrorEvent.
func (c *Client) WatchProjectStream(ctx context.Context, projectID string, opts ProjectStreamOptions) <-chan ProjectStreamEvent {
	ch := make(chan ProjectStreamEvent)
	go c.watchProjectStream(ctx, projectID, opts, ch)
	return ch
}

func (c *Client) watchProjectStream(ctx context.Context, projectID string, opts ProjectStreamOptions, ch chan<- ProjectStreamEvent) {
	defer close(ch)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.WebSocketURL(projectPath(projectID, "/ws")), nil)
	if err != nil {
		sendProjectStreamEvent(ctx, ch, ProjectStreamErrorEvent{Error: err.Error()})
		return
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for _, req := range projectStreamSubscribeRequests(opts) {
		if err := conn.WriteJSON(req); err != nil {
			sendProjectStreamEvent(ctx, ch, ProjectStreamErrorEvent{Stream: ProjectStreamType(req.Stream), SessionID: req.SessionID, ThreadID: req.ThreadID, ServiceID: req.ServiceID, Error: err.Error()})
			return
		}
	}

	for {
		var msg projectStreamSocketMessageJSON
		if err := conn.ReadJSON(&msg); err != nil {
			if ctx.Err() != nil {
				return
			}
			sendProjectStreamEvent(ctx, ch, ProjectStreamErrorEvent{Error: err.Error()})
			return
		}
		for _, event := range projectStreamEvents(msg.Message) {
			if !sendProjectStreamEvent(ctx, ch, event) {
				return
			}
		}
	}
}

func projectStreamSubscribeRequests(opts ProjectStreamOptions) []projectStreamSocketRequest {
	var reqs []projectStreamSocketRequest
	if opts.ProjectEvents != nil {
		reqs = append(reqs, projectStreamSocketRequest{Type: "subscribe", Stream: string(ProjectStreamProjectEvents), AfterID: opts.ProjectEvents.AfterID})
	}
	if opts.Chat != nil {
		reqs = append(reqs, projectStreamSocketRequest{Type: "subscribe", Stream: string(ProjectStreamChat), SessionID: opts.Chat.SessionID, ThreadID: opts.Chat.ThreadID, Replay: opts.Chat.Replay, LastEventID: opts.Chat.LastEventID})
	}
	if opts.Service != nil {
		reqs = append(reqs, projectStreamSocketRequest{Type: "subscribe", Stream: string(ProjectStreamService), SessionID: opts.Service.SessionID, ServiceID: opts.Service.ServiceID})
	}
	return reqs
}

func sendProjectStreamEvent(ctx context.Context, ch chan<- ProjectStreamEvent, event ProjectStreamEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- event:
		return true
	}
}

func projectStreamEvents(msg projectStreamSocketMessage) []ProjectStreamEvent {
	switch m := msg.(type) {
	case projectStreamSubscribedMessage:
		return []ProjectStreamEvent{ProjectStreamSubscribedEvent(m)}
	case projectStreamUnsubscribedMessage:
		return []ProjectStreamEvent{ProjectStreamUnsubscribedEvent(m)}
	case projectStreamCompleteMessage:
		return []ProjectStreamEvent{ProjectStreamCompleteEvent(m)}
	case projectStreamErrorMessage:
		return []ProjectStreamEvent{ProjectStreamErrorEvent(m)}
	case projectEventsStreamMessage:
		event, err := parseProjectEventMessage(m.Event, m.Data)
		if err != nil {
			return []ProjectStreamEvent{ProjectStreamErrorEvent{Stream: ProjectStreamProjectEvents, Error: err.Error()}}
		}
		return []ProjectStreamEvent{event}
	case chatStreamMessage:
		return []ProjectStreamEvent{ChatStreamEvent{SessionID: m.SessionID, ThreadID: m.ThreadID, Event: m.Event, Data: json.RawMessage(m.Data), ID: m.ID}}
	case serviceStreamMessage:
		return []ProjectStreamEvent{ServiceOutputEvent(m)}
	}
	return nil
}

func (m projectStreamSocketMessageJSON) MarshalJSON() ([]byte, error) {
	if m.Message == nil {
		return []byte("null"), nil
	}
	t := m.Message.projectStreamSocketMessageType()
	switch msg := m.Message.(type) {
	case projectStreamSubscribedMessage:
		return json.Marshal(struct {
			Type string `json:"type"`
			projectStreamSubscribedMessage
		}{t, msg})
	case projectStreamUnsubscribedMessage:
		return json.Marshal(struct {
			Type string `json:"type"`
			projectStreamUnsubscribedMessage
		}{t, msg})
	case projectStreamCompleteMessage:
		return json.Marshal(struct {
			Type string `json:"type"`
			projectStreamCompleteMessage
		}{t, msg})
	case projectStreamErrorMessage:
		return json.Marshal(struct {
			Type string `json:"type"`
			projectStreamErrorMessage
		}{t, msg})
	case projectEventsStreamMessage:
		return json.Marshal(struct {
			Type   string            `json:"type"`
			Stream ProjectStreamType `json:"stream"`
			projectEventsStreamMessage
		}{t, ProjectStreamProjectEvents, msg})
	case chatStreamMessage:
		return json.Marshal(struct {
			Type   string            `json:"type"`
			Stream ProjectStreamType `json:"stream"`
			chatStreamMessage
		}{t, ProjectStreamChat, msg})
	case serviceStreamMessage:
		return json.Marshal(struct {
			Type   string            `json:"type"`
			Stream ProjectStreamType `json:"stream"`
			serviceStreamMessage
		}{t, ProjectStreamService, msg})
	case unknownProjectStreamSocketMessage:
		if len(msg.Raw) > 0 {
			return msg.Raw, nil
		}
		return json.Marshal(struct {
			Type   string            `json:"type"`
			Stream ProjectStreamType `json:"stream,omitempty"`
		}{msg.Type, msg.Stream})
	default:
		return nil, fmt.Errorf("unknown project stream socket message type: %T", m.Message)
	}
}

func (m *projectStreamSocketMessageJSON) UnmarshalJSON(data []byte) error {
	var disc struct {
		Type   string            `json:"type"`
		Stream ProjectStreamType `json:"stream,omitempty"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return fmt.Errorf("unmarshal project stream message discriminator: %w", err)
	}

	switch disc.Type {
	case "subscribed":
		var msg projectStreamSubscribedMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		m.Message = msg
	case "unsubscribed":
		var msg projectStreamUnsubscribedMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		m.Message = msg
	case "complete":
		var msg projectStreamCompleteMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		m.Message = msg
	case "error":
		var msg projectStreamErrorMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		m.Message = msg
	case "event":
		switch disc.Stream {
		case ProjectStreamProjectEvents:
			var msg projectEventsStreamMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				return err
			}
			m.Message = msg
		case ProjectStreamChat:
			var msg chatStreamMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				return err
			}
			m.Message = msg
		case ProjectStreamService:
			var msg serviceStreamMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				return err
			}
			m.Message = msg
		default:
			m.Message = unknownProjectStreamSocketMessage{Type: disc.Type, Stream: disc.Stream, Raw: append(json.RawMessage(nil), data...)}
		}
	default:
		m.Message = unknownProjectStreamSocketMessage{Type: disc.Type, Stream: disc.Stream, Raw: append(json.RawMessage(nil), data...)}
	}
	return nil
}

func parseProjectEventMessage(eventName ProjectEventName, data string) (ProjectStreamEvent, error) {
	if eventName == ProjectEventConnected {
		var connected struct {
			ProjectID string `json:"projectId"`
		}
		if err := json.Unmarshal([]byte(data), &connected); err != nil {
			return nil, fmt.Errorf("decode connected event: %w", err)
		}
		return ProjectConnectedEvent{ProjectID: connected.ProjectID}, nil
	}

	var envelope struct {
		ID        string           `json:"id"`
		Seq       int64            `json:"seq"`
		Type      ProjectEventName `json:"type"`
		Timestamp time.Time        `json:"timestamp"`
		Data      json.RawMessage  `json:"data"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return nil, fmt.Errorf("decode project event %q: %w", eventName, err)
	}
	base := ProjectEventBase{ID: envelope.ID, Seq: envelope.Seq, Type: envelope.Type, Timestamp: envelope.Timestamp, RawData: envelope.Data}
	switch envelope.Type {
	case ProjectEventSessionUpdated:
		var payload SessionUpdatedData
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode session_updated data: %w", err)
		}
		return SessionUpdatedEvent{ProjectEventBase: base, Data: payload}, nil
	case ProjectEventThreadUpdated:
		var payload ThreadUpdatedData
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode thread_updated data: %w", err)
		}
		return ThreadUpdatedEvent{ProjectEventBase: base, Data: payload}, nil
	case ProjectEventWorkspaceUpdated:
		var payload WorkspaceUpdatedData
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode workspace_updated data: %w", err)
		}
		return WorkspaceUpdatedEvent{ProjectEventBase: base, Data: payload}, nil
	case ProjectEventStartupTaskUpdated:
		var payload StartupTask
		if err := json.Unmarshal(envelope.Data, &payload); err != nil {
			return nil, fmt.Errorf("decode startup_task_updated data: %w", err)
		}
		return StartupTaskUpdatedEvent{ProjectEventBase: base, Data: payload}, nil
	default:
		return UnknownProjectEvent{ProjectEventBase: base, Data: envelope.Data}, nil
	}
}
