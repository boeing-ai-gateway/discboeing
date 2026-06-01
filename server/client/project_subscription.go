package client

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"

	serverapi "github.com/obot-platform/discobot/server/api"
)

// ProjectThreadSubscription identifies one subscribed thread chat stream.
type ProjectThreadSubscription struct {
	SessionID string
	ThreadID  string
}

// ProjectSubscriptionStartedEvent is emitted after project-events are
// subscribed. When Resync is true, the websocket reconnected and the caller
// should rebuild any cache that depends on missed deltas.
type ProjectSubscriptionStartedEvent struct {
	ProjectID string
	Resync    bool
	Threads   []ProjectThreadSubscription
}

// ProjectThreadSubscriptionStartedEvent is emitted after one thread chat stream
// is subscribed. Replay history and live events are delivered on Events.
type ProjectThreadSubscriptionStartedEvent struct {
	ProjectID string
	SessionID string
	ThreadID  string
	Resync    bool
}

// ProjectSubscriptionOptions configures a managed project subscription.
type ProjectSubscriptionOptions struct {
	ProjectEvents  serverapi.ProjectEventsSubscriptionOptions
	EventBuffer    int
	ReconnectDelay time.Duration
}

// ProjectSubscription is a managed project-events subscription. It owns one
// websocket connection at a time, keeps requested thread chat subscriptions in
// memory, and restores them after reconnecting.
type ProjectSubscription struct {
	client    *Client
	projectID string
	options   ProjectSubscriptionOptions

	ctx    context.Context
	cancel context.CancelFunc
	events chan ProjectStreamEvent
	done   chan struct{}

	mu      sync.Mutex
	conn    *ProjectStreamConnection
	threads map[ProjectThreadSubscription]serverapi.ChatStreamSubscriptionOptions
}

// SubscribeProject subscribes to project-level events and returns a managed
// subscription. The returned subscription buffers events immediately, so callers
// can list the current project state and then drain Events without losing
// server updates that arrived during the list calls.
func (c *Client) SubscribeProject(ctx context.Context, projectID string, opts ProjectSubscriptionOptions) (*ProjectSubscription, error) {
	if opts.EventBuffer <= 0 {
		opts.EventBuffer = 128
	}
	if opts.ReconnectDelay <= 0 {
		opts.ReconnectDelay = time.Second
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := &ProjectSubscription{
		client:    c,
		projectID: projectID,
		options:   opts,
		ctx:       subCtx,
		cancel:    cancel,
		events:    make(chan ProjectStreamEvent, opts.EventBuffer),
		done:      make(chan struct{}),
		threads:   map[ProjectThreadSubscription]serverapi.ChatStreamSubscriptionOptions{},
	}

	ready := make(chan error, 1)
	go sub.run(ready)
	if err := <-ready; err != nil {
		cancel()
		return nil, err
	}
	return sub, nil
}

// Subscribe subscribes to project-level events for this project and returns a
// managed subscription.
func (c *ProjectClient) Subscribe(ctx context.Context, opts ProjectSubscriptionOptions) (*ProjectSubscription, error) {
	return c.client.SubscribeProject(ctx, c.projectID, opts)
}

// Events returns buffered project and subscribed thread events. The channel is
// closed when the subscription is closed or its context is canceled.
func (s *ProjectSubscription) Events() <-chan ProjectStreamEvent {
	return s.events
}

// Event returns buffered project and subscribed thread events. It is an alias
// for Events for call sites that read the subscription as one event stream.
func (s *ProjectSubscription) Event() <-chan ProjectStreamEvent {
	return s.events
}

// Done is closed when the managed subscription stops.
func (s *ProjectSubscription) Done() <-chan struct{} {
	return s.done
}

// SubscribeThread subscribes to replayed and live chat events for one session
// thread and records the subscription so it can be restored after a reconnect.
func (s *ProjectSubscription) SubscribeThread(ctx context.Context, sessionID, threadID string) error {
	key := ProjectThreadSubscription{SessionID: sessionID, ThreadID: threadID}
	chat := serverapi.ChatStreamSubscriptionOptions{SessionID: sessionID, ThreadID: threadID, Replay: true}

	s.mu.Lock()
	s.threads[key] = chat
	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return nil
	}
	return conn.Subscribe(ctx, serverapi.ProjectStreamOptions{Chat: &chat})
}

// UnsubscribeThread unsubscribes from live chat events for one session thread
// and removes it from the reconnect subscription set.
func (s *ProjectSubscription) UnsubscribeThread(ctx context.Context, sessionID, threadID string) error {
	key := ProjectThreadSubscription{SessionID: sessionID, ThreadID: threadID}
	chat := serverapi.ChatStreamSubscriptionOptions{SessionID: sessionID, ThreadID: threadID}

	s.mu.Lock()
	delete(s.threads, key)
	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return nil
	}
	return conn.Unsubscribe(ctx, serverapi.ProjectStreamOptions{Chat: &chat})
}

// SubscribedThreads returns a snapshot of the thread chat subscriptions that
// will be restored if the websocket reconnects.
func (s *ProjectSubscription) SubscribedThreads() []ProjectThreadSubscription {
	s.mu.Lock()
	defer s.mu.Unlock()

	threads, _ := s.snapshotThreadSubscriptionsLocked()
	return threads
}

// Close stops the managed subscription and closes its current websocket.
func (s *ProjectSubscription) Close() error {
	s.cancel()
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func (s *ProjectSubscription) run(ready chan<- error) {
	defer close(s.done)
	defer close(s.events)

	firstConnect := true
	reconnected := false
	for {
		conn, err := s.client.OpenProjectStream(s.ctx, s.projectID)
		if err == nil {
			err = conn.Subscribe(s.ctx, serverapi.ProjectStreamOptions{ProjectEvents: &s.options.ProjectEvents})
		}

		// Subscribe to project events first and buffer anything that arrives before
		// the server acknowledges that project-events subscription. This lets the
		// caller list current state immediately after SubscribeProject returns.
		if err == nil {
			for subscribed := false; !subscribed; {
				select {
				case <-s.ctx.Done():
					err = s.ctx.Err()
					subscribed = true
				case event, ok := <-conn.Events():
					if !ok {
						err = conn.Err()
						if err == nil {
							err = websocket.CloseError{Code: websocket.StatusAbnormalClosure}
						}
						subscribed = true
						break
					}
					if event, ok := event.(serverapi.ProjectStreamSubscribedEvent); ok && event.Stream == serverapi.ProjectStreamTypeProjectEvents {
						subscribed = true
						break
					}
					if sendErr := s.sendStreamEvent(event, reconnected); sendErr != nil {
						err = sendErr
						subscribed = true
					}
				}
			}
		}

		var threadEvents []ProjectThreadSubscription
		if err == nil {
			s.mu.Lock()
			s.conn = conn
			var threadOptions []serverapi.ChatStreamSubscriptionOptions
			threadEvents, threadOptions = s.snapshotThreadSubscriptionsLocked()
			s.mu.Unlock()

			for _, chat := range threadOptions {
				if err = conn.Subscribe(s.ctx, serverapi.ProjectStreamOptions{Chat: &chat}); err != nil {
					break
				}
			}
		}

		if firstConnect {
			ready <- err
			firstConnect = false
		}
		if err == nil {
			if err = s.sendEvent(ProjectSubscriptionStartedEvent{ProjectID: s.projectID, Resync: reconnected, Threads: threadEvents}); err != nil {
				_ = conn.Close()
			}
		}
		if err != nil {
			if conn != nil {
				_ = conn.Close()
			}
			if s.ctx.Err() != nil {
				return
			}
			timer := time.NewTimer(s.options.ReconnectDelay)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				reconnected = true
				continue
			}
		}

		for event := range conn.Events() {
			if err := s.sendStreamEvent(event, reconnected); err != nil {
				return
			}
		}

		s.mu.Lock()
		if s.conn == conn {
			s.conn = nil
		}
		s.mu.Unlock()
		if s.ctx.Err() != nil {
			return
		}
		reconnected = true
	}
}

func (s *ProjectSubscription) sendEvent(event ProjectStreamEvent) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case s.events <- event:
		return nil
	}
}

func (s *ProjectSubscription) sendStreamEvent(event ProjectStreamEvent, resync bool) error {
	if subscribed, ok := event.(serverapi.ProjectStreamSubscribedEvent); ok && subscribed.Stream == serverapi.ProjectStreamTypeChat {
		return s.sendEvent(ProjectThreadSubscriptionStartedEvent{
			ProjectID: s.projectID,
			SessionID: subscribed.SessionID,
			ThreadID:  subscribed.ThreadID,
			Resync:    resync,
		})
	}
	return s.sendEvent(event)
}

func (s *ProjectSubscription) snapshotThreadSubscriptionsLocked() ([]ProjectThreadSubscription, []serverapi.ChatStreamSubscriptionOptions) {
	threads := make([]ProjectThreadSubscription, 0, len(s.threads))
	for thread := range s.threads {
		threads = append(threads, thread)
	}
	sort.Slice(threads, func(i, j int) bool {
		if threads[i].SessionID != threads[j].SessionID {
			return threads[i].SessionID < threads[j].SessionID
		}
		return threads[i].ThreadID < threads[j].ThreadID
	})

	options := make([]serverapi.ChatStreamSubscriptionOptions, 0, len(threads))
	for _, thread := range threads {
		options = append(options, s.threads[thread])
	}
	return threads, options
}
