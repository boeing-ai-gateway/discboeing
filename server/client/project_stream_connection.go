package client

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	serverapi "github.com/obot-platform/discobot/server/api"
)

// ProjectStreamConnection is one shared project websocket connection.
type ProjectStreamConnection struct {
	conn   *websocket.Conn
	events chan ProjectStreamEvent
	done   chan struct{}

	writeMu sync.Mutex
	closeMu sync.Mutex
	closed  bool
	err     error
}

// OpenProjectStream opens one project websocket connection without subscribing
// to any sub-streams. Use Subscribe and Unsubscribe to manage streams on the
// shared connection, and Events to receive all events from subscribed streams.
func (c *Client) OpenProjectStream(ctx context.Context, projectID string) (*ProjectStreamConnection, error) {
	conn, _, err := websocket.Dial(ctx, c.WebSocketURL(projectPath(projectID, "/ws")), nil)
	if err != nil {
		return nil, err
	}
	stream := &ProjectStreamConnection{
		conn:   conn,
		events: make(chan ProjectStreamEvent, 128),
		done:   make(chan struct{}),
	}
	go stream.readLoop(ctx)
	go func() {
		<-ctx.Done()
		_ = stream.Close()
	}()
	return stream, nil
}

// OpenStream opens one shared project websocket connection for this project.
func (c *ProjectClient) OpenStream(ctx context.Context) (*ProjectStreamConnection, error) {
	return c.client.OpenProjectStream(ctx, c.projectID)
}

// Events returns all typed events received on the project websocket. The channel
// is closed when the connection closes.
func (s *ProjectStreamConnection) Events() <-chan ProjectStreamEvent {
	return s.events
}

// Done is closed when the project websocket connection has stopped.
func (s *ProjectStreamConnection) Done() <-chan struct{} {
	return s.done
}

// Err returns the websocket read error that stopped the connection, if any.
func (s *ProjectStreamConnection) Err() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	return s.err
}

// Subscribe subscribes to one or more sub-streams on the shared websocket.
func (s *ProjectStreamConnection) Subscribe(ctx context.Context, opts serverapi.ProjectStreamOptions) error {
	var requests []projectStreamSocketRequest
	if opts.ProjectEvents != nil {
		requests = append(requests, projectStreamSocketRequest{
			Type:    "subscribe",
			Stream:  string(serverapi.ProjectStreamTypeProjectEvents),
			AfterID: opts.ProjectEvents.AfterID,
		})
	}
	if opts.Chat != nil {
		requests = append(requests, projectStreamSocketRequest{
			Type:        "subscribe",
			Stream:      string(serverapi.ProjectStreamTypeChat),
			SessionID:   opts.Chat.SessionID,
			ThreadID:    opts.Chat.ThreadID,
			Replay:      opts.Chat.Replay,
			LastEventID: opts.Chat.LastEventID,
		})
	}
	if opts.Service != nil {
		requests = append(requests, projectStreamSocketRequest{
			Type:      "subscribe",
			Stream:    string(serverapi.ProjectStreamTypeService),
			SessionID: opts.Service.SessionID,
			ServiceID: opts.Service.ServiceID,
		})
	}
	return s.writeRequests(ctx, requests)
}

// Unsubscribe unsubscribes from one or more sub-streams on the shared websocket.
func (s *ProjectStreamConnection) Unsubscribe(ctx context.Context, opts serverapi.ProjectStreamOptions) error {
	var requests []projectStreamSocketRequest
	if opts.ProjectEvents != nil {
		requests = append(requests, projectStreamSocketRequest{
			Type:    "unsubscribe",
			Stream:  string(serverapi.ProjectStreamTypeProjectEvents),
			AfterID: opts.ProjectEvents.AfterID,
		})
	}
	if opts.Chat != nil {
		requests = append(requests, projectStreamSocketRequest{
			Type:        "unsubscribe",
			Stream:      string(serverapi.ProjectStreamTypeChat),
			SessionID:   opts.Chat.SessionID,
			ThreadID:    opts.Chat.ThreadID,
			Replay:      opts.Chat.Replay,
			LastEventID: opts.Chat.LastEventID,
		})
	}
	if opts.Service != nil {
		requests = append(requests, projectStreamSocketRequest{
			Type:      "unsubscribe",
			Stream:    string(serverapi.ProjectStreamTypeService),
			SessionID: opts.Service.SessionID,
			ServiceID: opts.Service.ServiceID,
		})
	}
	return s.writeRequests(ctx, requests)
}

// Close closes the shared websocket connection.
func (s *ProjectStreamConnection) Close() error {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	s.closeMu.Unlock()
	return s.conn.Close(websocket.StatusNormalClosure, "done")
}

func (s *ProjectStreamConnection) readLoop(ctx context.Context) {
	defer close(s.done)
	defer close(s.events)
	defer func() { _ = s.conn.Close(websocket.StatusNormalClosure, "done") }()

	for {
		var msg ProjectStreamSocketMessageJSON
		if err := wsjson.Read(ctx, s.conn, &msg); err != nil {
			if ctx.Err() == nil && !errors.Is(err, net.ErrClosed) {
				s.setErr(err)
				s.sendEvent(ctx, serverapi.ProjectStreamErrorEvent{Error: err.Error()})
			}
			return
		}
		for _, event := range projectStreamEvents(msg.Message) {
			if !s.sendEvent(ctx, event) {
				return
			}
		}
	}
}

func (s *ProjectStreamConnection) writeRequests(ctx context.Context, reqs []projectStreamSocketRequest) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, req := range reqs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			if err := s.Err(); err != nil {
				return err
			}
			return net.ErrClosed
		default:
		}
		if err := wsjson.Write(ctx, s.conn, req); err != nil {
			_ = s.Close()
			return err
		}
	}
	return nil
}

func (s *ProjectStreamConnection) sendEvent(ctx context.Context, event ProjectStreamEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case s.events <- event:
		return true
	}
}

func (s *ProjectStreamConnection) setErr(err error) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	s.err = err
}
