package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/obot-platform/discobot/server/internal/events"
	"github.com/obot-platform/discobot/server/internal/service"
)

type projectStreamSubscriptionRequest struct {
	Type        string `json:"type"`
	Stream      string `json:"stream"`
	SessionID   string `json:"sessionId,omitempty"`
	ThreadID    string `json:"threadId,omitempty"`
	ServiceID   string `json:"serviceId,omitempty"`
	Replay      bool   `json:"replay,omitempty"`
	LastEventID string `json:"lastEventId,omitempty"`
	AfterID     string `json:"afterId,omitempty"`
}

type projectStreamSocketMessage struct {
	Type      string `json:"type"`
	Stream    string `json:"stream,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	ThreadID  string `json:"threadId,omitempty"`
	ServiceID string `json:"serviceId,omitempty"`
	Event     string `json:"event,omitempty"`
	Data      string `json:"data,omitempty"`
	ID        string `json:"id,omitempty"`
	Error     string `json:"error,omitempty"`
	Replay    bool   `json:"replay,omitempty"`
}

type projectStreamSubscriptionKey struct {
	stream    string
	sessionID string
	threadID  string
	serviceID string
}

func subscriptionKey(req projectStreamSubscriptionRequest) projectStreamSubscriptionKey {
	return projectStreamSubscriptionKey{
		stream:    req.Stream,
		sessionID: req.SessionID,
		threadID:  req.ThreadID,
		serviceID: req.ServiceID,
	}
}

type ProjectStreamSocket struct {
	chatService *service.ChatService
	eventBroker *events.Broker
	projectID   string
	conn        *websocket.Conn
	ctx         context.Context
	cancel      context.CancelFunc
	outgoing    chan projectStreamSocketMessage

	subscriptionsMu sync.Mutex
	subscriptions   map[projectStreamSubscriptionKey]context.CancelFunc
}

func NewProjectStreamSocket(
	ctx context.Context,
	cancel context.CancelFunc,
	conn *websocket.Conn,
	projectID string,
	chatService *service.ChatService,
	eventBroker *events.Broker,
) *ProjectStreamSocket {
	return &ProjectStreamSocket{
		chatService:   chatService,
		eventBroker:   eventBroker,
		projectID:     projectID,
		conn:          conn,
		ctx:           ctx,
		cancel:        cancel,
		outgoing:      make(chan projectStreamSocketMessage, 128),
		subscriptions: make(map[projectStreamSubscriptionKey]context.CancelFunc),
	}
}

func (s *ProjectStreamSocket) Run() {
	defer s.cancelAllSubscriptions()

	writerDone := make(chan struct{})
	go s.runWriter(writerDone)

	s.runReader()

	s.cancel()
	<-writerDone
}

func (s *ProjectStreamSocket) runWriter(done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case message, ok := <-s.outgoing:
			if !ok {
				return
			}
			if err := wsjson.Write(s.ctx, s.conn, message); err != nil {
				s.cancel()
				return
			}
		}
	}
}

func (s *ProjectStreamSocket) runReader() {
	for {
		var req projectStreamSubscriptionRequest
		if err := wsjson.Read(s.ctx, s.conn, &req); err != nil {
			status := websocket.CloseStatus(err)
			if status != websocket.StatusNormalClosure &&
				status != websocket.StatusGoingAway &&
				status != websocket.StatusAbnormalClosure &&
				!errors.Is(err, net.ErrClosed) &&
				s.ctx.Err() == nil {
				log.Printf("chat websocket read error: %v", err)
			}
			return
		}

		s.handleRequest(req)
	}
}

func (s *ProjectStreamSocket) handleRequest(req projectStreamSubscriptionRequest) {
	key := subscriptionKey(req)

	switch req.Type {
	case "subscribe":
		s.handleSubscribe(req)
	case "unsubscribe":
		s.cancelSubscription(key)
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:      "unsubscribed",
			Stream:    req.Stream,
			SessionID: req.SessionID,
			ThreadID:  req.ThreadID,
			ServiceID: req.ServiceID,
		})
	default:
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:   "error",
			Stream: req.Stream,
			Error:  fmt.Sprintf("unsupported message type %q", req.Type),
		})
	}
}

func (s *ProjectStreamSocket) handleSubscribe(req projectStreamSubscriptionRequest) {
	switch req.Stream {
	case "chat":
		s.startChatSubscription(req)
	case "service":
		s.startServiceSubscription(req)
	case "project-events":
		s.startProjectEventsSubscription(req)
	default:
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:   "error",
			Stream: req.Stream,
			Error:  fmt.Sprintf("unsupported stream %q", req.Stream),
		})
	}
}

func (s *ProjectStreamSocket) removeSubscription(key projectStreamSubscriptionKey) {
	s.subscriptionsMu.Lock()
	defer s.subscriptionsMu.Unlock()
	delete(s.subscriptions, key)
}

func (s *ProjectStreamSocket) cancelSubscription(key projectStreamSubscriptionKey) {
	s.subscriptionsMu.Lock()
	cancelFn, ok := s.subscriptions[key]
	if ok {
		delete(s.subscriptions, key)
	}
	s.subscriptionsMu.Unlock()
	if ok {
		cancelFn()
	}
}

func (s *ProjectStreamSocket) cancelAllSubscriptions() {
	s.subscriptionsMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.subscriptions))
	for key, cancelFn := range s.subscriptions {
		cancels = append(cancels, cancelFn)
		delete(s.subscriptions, key)
	}
	s.subscriptionsMu.Unlock()
	for _, cancelFn := range cancels {
		cancelFn()
	}
}

func (s *ProjectStreamSocket) writeMessage(message projectStreamSocketMessage) bool {
	select {
	case <-s.ctx.Done():
		return false
	case s.outgoing <- message:
		return true
	}
}

func (s *ProjectStreamSocket) trackSubscription(key projectStreamSubscriptionKey, cancel context.CancelFunc) {
	s.subscriptionsMu.Lock()
	s.subscriptions[key] = cancel
	s.subscriptionsMu.Unlock()
}

func (s *ProjectStreamSocket) startChatSubscription(req projectStreamSubscriptionRequest) {
	if req.SessionID == "" {
		_ = s.writeMessage(projectStreamSocketMessage{Type: "error", Stream: "chat", Error: "sessionId is required"})
		return
	}
	if req.ThreadID == "" {
		_ = s.writeMessage(projectStreamSocketMessage{Type: "error", Stream: "chat", SessionID: req.SessionID, Error: "threadId is required"})
		return
	}

	if _, err := s.chatService.GetSession(s.ctx, s.projectID, req.SessionID); err != nil {
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:      "error",
			Stream:    "chat",
			SessionID: req.SessionID,
			ThreadID:  req.ThreadID,
			Error:     err.Error(),
		})
		return
	}

	key := subscriptionKey(req)
	s.cancelSubscription(key)

	lastEventID := ""
	if req.Replay {
		lastEventID = req.LastEventID
	}

	streamCtx, streamCancel := context.WithCancel(s.ctx)
	sseCh, err := s.chatService.GetStream(streamCtx, s.projectID, req.SessionID, req.ThreadID, lastEventID)
	if err != nil {
		streamCancel()
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:      "error",
			Stream:    "chat",
			SessionID: req.SessionID,
			ThreadID:  req.ThreadID,
			Error:     err.Error(),
		})
		return
	}

	s.trackSubscription(key, streamCancel)

	if !s.writeMessage(projectStreamSocketMessage{
		Type:      "subscribed",
		Stream:    "chat",
		SessionID: req.SessionID,
		ThreadID:  req.ThreadID,
		Replay:    req.Replay,
	}) {
		s.cancelSubscription(key)
		return
	}

	go func() {
		defer func() {
			streamCancel()
			s.removeSubscription(key)
			_ = s.writeMessage(projectStreamSocketMessage{
				Type:      "complete",
				Stream:    "chat",
				SessionID: req.SessionID,
				ThreadID:  req.ThreadID,
			})
		}()

		for {
			select {
			case <-streamCtx.Done():
				return
			case line, ok := <-sseCh:
				if !ok {
					return
				}
				if line.Done {
					continue
				}
				if !s.writeMessage(projectStreamSocketMessage{
					Type:      "event",
					Stream:    "chat",
					SessionID: req.SessionID,
					ThreadID:  req.ThreadID,
					Event:     line.Event,
					Data:      line.Data,
					ID:        line.ID,
				}) {
					return
				}
			}
		}
	}()
}

func (s *ProjectStreamSocket) startServiceSubscription(req projectStreamSubscriptionRequest) {
	if req.SessionID == "" {
		_ = s.writeMessage(projectStreamSocketMessage{Type: "error", Stream: "service", Error: "sessionId is required"})
		return
	}
	if req.ServiceID == "" {
		_ = s.writeMessage(projectStreamSocketMessage{Type: "error", Stream: "service", SessionID: req.SessionID, Error: "serviceId is required"})
		return
	}

	if _, err := s.chatService.GetSession(s.ctx, s.projectID, req.SessionID); err != nil {
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:      "error",
			Stream:    "service",
			SessionID: req.SessionID,
			ServiceID: req.ServiceID,
			Error:     err.Error(),
		})
		return
	}

	key := subscriptionKey(req)
	s.cancelSubscription(key)

	streamCtx, streamCancel := context.WithCancel(s.ctx)
	sseCh, err := s.chatService.GetServiceOutput(streamCtx, s.projectID, req.SessionID, req.ServiceID)
	if err != nil {
		streamCancel()
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:      "error",
			Stream:    "service",
			SessionID: req.SessionID,
			ServiceID: req.ServiceID,
			Error:     err.Error(),
		})
		return
	}

	s.trackSubscription(key, streamCancel)

	if !s.writeMessage(projectStreamSocketMessage{
		Type:      "subscribed",
		Stream:    "service",
		SessionID: req.SessionID,
		ServiceID: req.ServiceID,
	}) {
		s.cancelSubscription(key)
		return
	}

	go func() {
		defer func() {
			streamCancel()
			s.removeSubscription(key)
			_ = s.writeMessage(projectStreamSocketMessage{
				Type:      "complete",
				Stream:    "service",
				SessionID: req.SessionID,
				ServiceID: req.ServiceID,
			})
		}()

		for {
			select {
			case <-streamCtx.Done():
				return
			case line, ok := <-sseCh:
				if !ok {
					_ = s.writeMessage(projectStreamSocketMessage{
						Type:      "event",
						Stream:    "service",
						SessionID: req.SessionID,
						ServiceID: req.ServiceID,
						Data:      "[DONE]",
					})
					return
				}
				if line.Done {
					_ = s.writeMessage(projectStreamSocketMessage{
						Type:      "event",
						Stream:    "service",
						SessionID: req.SessionID,
						ServiceID: req.ServiceID,
						Data:      "[DONE]",
					})
					return
				}
				if !s.writeMessage(projectStreamSocketMessage{
					Type:      "event",
					Stream:    "service",
					SessionID: req.SessionID,
					ServiceID: req.ServiceID,
					Data:      line.Data,
					ID:        line.ID,
				}) {
					return
				}
			}
		}
	}()
}

func (s *ProjectStreamSocket) startProjectEventsSubscription(req projectStreamSubscriptionRequest) {
	key := subscriptionKey(req)
	s.cancelSubscription(key)

	streamCtx, streamCancel := context.WithCancel(s.ctx)
	sub := s.eventBroker.Subscribe(s.projectID)
	sentEventIDs := map[string]bool{}

	s.trackSubscription(key, streamCancel)

	if !s.writeMessage(projectStreamSocketMessage{
		Type:   "subscribed",
		Stream: "project-events",
	}) {
		s.cancelSubscription(key)
		return
	}

	if !s.writeMessage(projectStreamSocketMessage{
		Type:   "event",
		Stream: "project-events",
		Event:  "connected",
		Data:   fmt.Sprintf(`{"projectId":%q}`, s.projectID),
	}) {
		s.cancelSubscription(key)
		return
	}

	history, err := s.projectEventHistory(streamCtx, req.AfterID, req.Replay)
	if err != nil {
		_ = s.writeMessage(projectStreamSocketMessage{
			Type:   "error",
			Stream: "project-events",
			Error:  "failed to get historical events",
		})
		s.cancelSubscription(key)
		return
	}
	for _, event := range history {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		sentEventIDs[event.ID] = true
		if !s.writeMessage(projectStreamSocketMessage{
			Type:   "event",
			Stream: "project-events",
			Event:  string(event.Type),
			Data:   string(data),
			ID:     event.ID,
		}) {
			s.cancelSubscription(key)
			return
		}
	}

	go func() {
		defer func() {
			streamCancel()
			s.eventBroker.Unsubscribe(sub)
			s.removeSubscription(key)
		}()

		for {
			select {
			case <-streamCtx.Done():
				return
			case event, ok := <-sub.Events:
				if !ok {
					return
				}
				if sentEventIDs[event.ID] {
					delete(sentEventIDs, event.ID)
					continue
				}
				data, err := json.Marshal(event)
				if err != nil {
					continue
				}
				if !s.writeMessage(projectStreamSocketMessage{
					Type:   "event",
					Stream: "project-events",
					Event:  string(event.Type),
					Data:   string(data),
					ID:     event.ID,
				}) {
					return
				}
			}
		}
	}()
}

func (s *ProjectStreamSocket) projectEventHistory(ctx context.Context, afterID string, replay bool) ([]*events.Event, error) {
	if afterID != "" {
		return s.eventBroker.GetEventsAfterID(ctx, s.projectID, afterID)
	}
	if !replay {
		return nil, nil
	}
	return s.eventBroker.GetEventsSince(ctx, s.projectID, time.Time{})
}
