package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	serverapi "github.com/obot-platform/discobot/server/api"
)

type projectSubscriptionTestServer struct {
	t      *testing.T
	server *httptest.Server
	conns  chan *projectSubscriptionTestConn
}

type projectSubscriptionTestConn struct {
	conn     *websocket.Conn
	requests chan projectStreamSocketRequest
	done     chan struct{}
}

func newProjectSubscriptionTestServer(t *testing.T) *projectSubscriptionTestServer {
	t.Helper()

	streamServer := &projectSubscriptionTestServer{
		t:     t,
		conns: make(chan *projectSubscriptionTestConn, 16),
	}
	streamServer.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}

		testConn := &projectSubscriptionTestConn{
			conn:     conn,
			requests: make(chan projectStreamSocketRequest, 16),
			done:     make(chan struct{}),
		}
		streamServer.conns <- testConn
		go testConn.readRequests()
	}))
	t.Cleanup(streamServer.server.Close)
	return streamServer
}

func (s *projectSubscriptionTestServer) client(t *testing.T) *Client {
	t.Helper()
	client, err := NewClient(s.server.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func (s *projectSubscriptionTestServer) nextConn(ctx context.Context, t *testing.T) *projectSubscriptionTestConn {
	t.Helper()
	select {
	case conn := <-s.conns:
		return conn
	case <-ctx.Done():
		t.Fatalf("timed out waiting for websocket connection: %v", ctx.Err())
		return nil
	}
}

func (c *projectSubscriptionTestConn) readRequests() {
	defer close(c.done)
	defer close(c.requests)
	defer func() { _ = c.conn.Close(websocket.StatusNormalClosure, "done") }()

	for {
		var req projectStreamSocketRequest
		if err := wsjson.Read(context.Background(), c.conn, &req); err != nil {
			return
		}
		c.requests <- req
	}
}

func (c *projectSubscriptionTestConn) nextRequest(ctx context.Context, t *testing.T) projectStreamSocketRequest {
	t.Helper()
	select {
	case req, ok := <-c.requests:
		if !ok {
			t.Fatal("websocket request channel closed")
		}
		return req
	case <-ctx.Done():
		t.Fatalf("timed out waiting for websocket request: %v", ctx.Err())
		return projectStreamSocketRequest{}
	}
}

func (c *projectSubscriptionTestConn) writeEvent(ctx context.Context, t *testing.T, event ProjectStreamEvent) {
	t.Helper()
	if err := wsjson.Write(ctx, c.conn, ProjectStreamSocketMessageJSON{Message: event}); err != nil {
		t.Fatalf("write websocket event: %v", err)
	}
}

func (c *projectSubscriptionTestConn) close(t *testing.T) {
	t.Helper()
	_ = c.conn.Close(websocket.StatusNormalClosure, "test closed")
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func readSubscriptionEvent[T any](ctx context.Context, t *testing.T, events <-chan ProjectStreamEvent) T {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("subscription events channel closed")
		}
		typed, ok := event.(T)
		if !ok {
			t.Fatalf("event type = %T, want %T", event, *new(T))
		}
		return typed
	case <-ctx.Done():
		t.Fatalf("timed out waiting for subscription event: %v", ctx.Err())
		return *new(T)
	}
}

func readMatchingSubscriptionEvent[T any](ctx context.Context, t *testing.T, events <-chan ProjectStreamEvent) T {
	t.Helper()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("subscription events channel closed")
			}
			if typed, ok := event.(T); ok {
				return typed
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for subscription event: %v", ctx.Err())
			return *new(T)
		}
	}
}

func TestProjectSubscriptionRunBuffersEventsBeforeProjectSubscribed(t *testing.T) {
	ctx := testContext(t)
	streamServer := newProjectSubscriptionTestServer(t)
	client := streamServer.client(t)

	subscribeErr := make(chan error, 1)
	var sub *ProjectSubscription
	go func() {
		var err error
		sub, err = client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{
			ProjectEvents: serverapi.ProjectEventsSubscriptionOptions{AfterID: "event-1"},
			EventBuffer:   4,
		})
		subscribeErr <- err
	}()

	conn := streamServer.nextConn(ctx, t)
	projectReq := conn.nextRequest(ctx, t)
	if projectReq.Type != "subscribe" || projectReq.Stream != string(serverapi.ProjectStreamTypeProjectEvents) || projectReq.AfterID != "event-1" {
		t.Fatalf("project subscribe request = %#v", projectReq)
	}

	conn.writeEvent(ctx, t, serverapi.ProjectEventsStreamMessage{
		Stream: serverapi.ProjectStreamTypeProjectEvents,
		Event:  serverapi.Connected,
		Data:   `{"projectId":"project-1"}`,
		ID:     "queued-event",
	})
	conn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeProjectEvents})

	if err := <-subscribeErr; err != nil {
		t.Fatalf("subscribe project: %v", err)
	}
	defer func() { _ = sub.Close() }()

	event := readSubscriptionEvent[serverapi.ProjectConnectedEvent](ctx, t, sub.Events())
	if event.ProjectID != "project-1" {
		t.Fatalf("queued event = %#v", event)
	}
}

func TestProjectSubscriptionRunEmitsStartedEventAfterSubscribeAck(t *testing.T) {
	ctx := testContext(t)
	streamServer := newProjectSubscriptionTestServer(t)
	client := streamServer.client(t)

	subscribeErr := make(chan error, 1)
	var sub *ProjectSubscription
	go func() {
		var err error
		sub, err = client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{EventBuffer: 4})
		subscribeErr <- err
	}()

	conn := streamServer.nextConn(ctx, t)
	_ = conn.nextRequest(ctx, t)
	conn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeProjectEvents})
	if err := <-subscribeErr; err != nil {
		t.Fatalf("subscribe project: %v", err)
	}
	defer func() { _ = sub.Close() }()

	event := readSubscriptionEvent[ProjectSubscriptionStartedEvent](ctx, t, sub.Events())
	if event.ProjectID != "project-1" {
		t.Fatalf("started event ProjectID = %q, want project-1", event.ProjectID)
	}
	if event.Resync {
		t.Fatalf("started event Resync = true")
	}
	if len(event.Threads) != 0 {
		t.Fatalf("started event Threads = %#v, want empty", event.Threads)
	}
}

func TestProjectSubscriptionRunBuffersNonProjectAckBeforeProjectSubscribed(t *testing.T) {
	ctx := testContext(t)
	streamServer := newProjectSubscriptionTestServer(t)
	client := streamServer.client(t)

	subscribeErr := make(chan error, 1)
	var sub *ProjectSubscription
	go func() {
		var err error
		sub, err = client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{EventBuffer: 4})
		subscribeErr <- err
	}()

	conn := streamServer.nextConn(ctx, t)
	_ = conn.nextRequest(ctx, t)
	conn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeChat, SessionID: "session-1", ThreadID: "thread-1"})
	conn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeProjectEvents})

	if err := <-subscribeErr; err != nil {
		t.Fatalf("subscribe project: %v", err)
	}
	defer func() { _ = sub.Close() }()

	event := readSubscriptionEvent[ProjectThreadSubscriptionStartedEvent](ctx, t, sub.Events())
	if event.ProjectID != "project-1" || event.SessionID != "session-1" || event.ThreadID != "thread-1" {
		t.Fatalf("buffered non-project ack = %#v", event)
	}
}

func TestProjectSubscriptionRunFailsInitialSubscribeWhenConnectionClosesBeforeAck(t *testing.T) {
	ctx := testContext(t)
	streamServer := newProjectSubscriptionTestServer(t)
	client := streamServer.client(t)

	subscribeErr := make(chan error, 1)
	go func() {
		_, err := client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{ReconnectDelay: time.Millisecond})
		subscribeErr <- err
	}()

	conn := streamServer.nextConn(ctx, t)
	_ = conn.nextRequest(ctx, t)
	conn.close(t)

	select {
	case err := <-subscribeErr:
		if err == nil {
			t.Fatal("expected initial subscribe error")
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for subscribe error: %v", ctx.Err())
	}
}

func TestProjectSubscriptionRunReturnsInitialDialError(t *testing.T) {
	ctx := testContext(t)
	closedServer := httptest.NewServer(http.NotFoundHandler())
	closedServer.Close()
	client, err := NewClient(closedServer.URL)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{ReconnectDelay: time.Millisecond})
	if err == nil {
		t.Fatal("expected initial dial error")
	}
}

func TestProjectSubscriptionRunReconnectsAndRestoresThreadSubscriptions(t *testing.T) {
	ctx := testContext(t)
	streamServer := newProjectSubscriptionTestServer(t)
	client := streamServer.client(t)

	subscribeErr := make(chan error, 1)
	var sub *ProjectSubscription
	go func() {
		var err error
		sub, err = client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{
			EventBuffer:    8,
			ReconnectDelay: time.Millisecond,
		})
		subscribeErr <- err
	}()

	firstConn := streamServer.nextConn(ctx, t)
	_ = firstConn.nextRequest(ctx, t)
	firstConn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeProjectEvents})
	if err := <-subscribeErr; err != nil {
		t.Fatalf("subscribe project: %v", err)
	}
	defer func() { _ = sub.Close() }()

	if err := sub.SubscribeThread(ctx, "session-b", "thread-b"); err != nil {
		t.Fatalf("subscribe thread b: %v", err)
	}
	if err := sub.SubscribeThread(ctx, "session-a", "thread-a"); err != nil {
		t.Fatalf("subscribe thread a: %v", err)
	}
	_ = firstConn.nextRequest(ctx, t)
	_ = firstConn.nextRequest(ctx, t)
	firstConn.close(t)

	secondConn := streamServer.nextConn(ctx, t)
	projectReq := secondConn.nextRequest(ctx, t)
	if projectReq.Type != "subscribe" || projectReq.Stream != string(serverapi.ProjectStreamTypeProjectEvents) {
		t.Fatalf("reconnect project subscribe request = %#v", projectReq)
	}
	secondConn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeProjectEvents})

	firstThreadReq := secondConn.nextRequest(ctx, t)
	secondThreadReq := secondConn.nextRequest(ctx, t)
	wantThreadReqs := []projectStreamSocketRequest{
		{Type: "subscribe", Stream: string(serverapi.ProjectStreamTypeChat), SessionID: "session-a", ThreadID: "thread-a", Replay: true},
		{Type: "subscribe", Stream: string(serverapi.ProjectStreamTypeChat), SessionID: "session-b", ThreadID: "thread-b", Replay: true},
	}
	gotThreadReqs := []projectStreamSocketRequest{firstThreadReq, secondThreadReq}
	if !reflect.DeepEqual(gotThreadReqs, wantThreadReqs) {
		t.Fatalf("restored thread requests = %#v, want %#v", gotThreadReqs, wantThreadReqs)
	}

	var resubscribed ProjectSubscriptionStartedEvent
	for {
		event := readSubscriptionEvent[ProjectStreamEvent](ctx, t, sub.Events())
		if typed, ok := event.(ProjectSubscriptionStartedEvent); ok && typed.Resync {
			resubscribed = typed
			break
		}
	}
	wantThreads := []ProjectThreadSubscription{
		{SessionID: "session-a", ThreadID: "thread-a"},
		{SessionID: "session-b", ThreadID: "thread-b"},
	}
	if !reflect.DeepEqual(resubscribed.Threads, wantThreads) {
		t.Fatalf("resubscribed threads = %#v, want %#v", resubscribed.Threads, wantThreads)
	}
}

func TestProjectSubscriptionRunForwardsEventsAfterSubscribeAck(t *testing.T) {
	ctx := testContext(t)
	streamServer := newProjectSubscriptionTestServer(t)
	client := streamServer.client(t)

	subscribeErr := make(chan error, 1)
	var sub *ProjectSubscription
	go func() {
		var err error
		sub, err = client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{EventBuffer: 4})
		subscribeErr <- err
	}()

	conn := streamServer.nextConn(ctx, t)
	_ = conn.nextRequest(ctx, t)
	conn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeProjectEvents})
	if err := <-subscribeErr; err != nil {
		t.Fatalf("subscribe project: %v", err)
	}
	defer func() { _ = sub.Close() }()

	conn.writeEvent(ctx, t, serverapi.ChatStreamMessage{
		Stream:    serverapi.ProjectStreamTypeChat,
		SessionID: "session-1",
		ThreadID:  "thread-1",
		Event:     serverapi.ChatStreamEventName("message"),
		ID:        "chat-event",
	})

	event := readMatchingSubscriptionEvent[serverapi.ChatStreamEvent](ctx, t, sub.Events())
	if event.ID != "chat-event" || event.SessionID != "session-1" || event.ThreadID != "thread-1" {
		t.Fatalf("forwarded chat event = %#v", event)
	}
}

func TestProjectSubscriptionRunCloseStopsAndClosesEvents(t *testing.T) {
	ctx := testContext(t)
	streamServer := newProjectSubscriptionTestServer(t)
	client := streamServer.client(t)

	subscribeErr := make(chan error, 1)
	var sub *ProjectSubscription
	go func() {
		var err error
		sub, err = client.SubscribeProject(ctx, "project-1", ProjectSubscriptionOptions{EventBuffer: 1})
		subscribeErr <- err
	}()

	conn := streamServer.nextConn(ctx, t)
	_ = conn.nextRequest(ctx, t)
	conn.writeEvent(ctx, t, serverapi.ProjectStreamSubscribedEvent{Stream: serverapi.ProjectStreamTypeProjectEvents})
	if err := <-subscribeErr; err != nil {
		t.Fatalf("subscribe project: %v", err)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("close subscription: %v", err)
	}
	select {
	case <-sub.Done():
	case <-ctx.Done():
		t.Fatalf("timed out waiting for subscription done: %v", ctx.Err())
	}
	for {
		select {
		case _, ok := <-sub.Events():
			if !ok {
				return
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for events channel close: %v", ctx.Err())
		}
	}
}
