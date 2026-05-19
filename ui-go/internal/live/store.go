package live

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	api "github.com/obot-platform/discobot/server/api"
)

const (
	DefaultProjectID = "local"
	idleTTL          = 30 * time.Second
)

// Scope describes the backend data the current UI view needs live.
type Scope struct {
	ProjectID string
	SessionID string
	ThreadID  string
}

// Event records a backend live-data update.
type Event struct {
	Version uint64
}

// Snapshot is the in-memory backend read side. It intentionally keeps the
// Discobot API client object types instead of introducing ui-go-only models.
type Snapshot struct {
	Ready             bool
	Loading           bool
	Error             string
	SelectedProjectID string
	Projects          []api.Project
	Workspaces        []api.Workspace
	Models            []api.ModelInfo
	Sessions          []api.Session
	ThreadsBySession  map[string][]api.Thread
	MessagesByThread  map[string][]Message
}

// Message is the live store's simplified in-memory conversation message.
type Message struct {
	ID      string
	Role    string
	Content string
}

// Store owns scoped process-local backend caches fetched through the Discobot
// API client. UI streams subscribe to project caches on demand and release them
// when disconnected.
type Store struct {
	client *api.Client

	mu       sync.Mutex
	projects map[string]*projectCache
}

// New returns a backend live-data cache manager backed by client.
func New(client *api.Client) *Store {
	return &Store{client: client, projects: map[string]*projectCache{}}
}

// NormalizeScope fills in stable defaults for a live data request.
func NormalizeScope(scope Scope) Scope {
	if scope.ProjectID == "" {
		scope.ProjectID = DefaultProjectID
	}
	return scope
}

// Subscribe keeps the scoped project cache alive and returns update events until
// the returned cancel function is called.
func (s *Store) Subscribe(scope Scope) (<-chan Event, func()) {
	cache := s.project(NormalizeScope(scope))
	return cache.subscribe(NormalizeScope(scope))
}

// Snapshot returns a copy of the current live backend state for scope.
func (s *Store) Snapshot(scope Scope) Snapshot {
	return s.project(NormalizeScope(scope)).snapshotFor(scope)
}

// EnsureLoaded loads backend state needed for scope if it has not loaded yet.
func (s *Store) EnsureLoaded(ctx context.Context, scope Scope) error {
	return s.project(NormalizeScope(scope)).ensureLoaded(ctx, NormalizeScope(scope))
}

// Refresh reloads backend state needed for scope and notifies subscribers.
func (s *Store) Refresh(ctx context.Context, scope Scope) error {
	return s.project(NormalizeScope(scope)).refresh(ctx, NormalizeScope(scope), true)
}

func (s *Store) project(scope Scope) *projectCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	cache, ok := s.projects[scope.ProjectID]
	if ok {
		cache.cancelIdle()
		return cache
	}
	cache = newProjectCache(scope.ProjectID, s.client, func(projectID string, cache *projectCache) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.projects[projectID] == cache && cache.subscriberCount() == 0 {
			delete(s.projects, projectID)
		}
	})
	s.projects[scope.ProjectID] = cache
	return cache
}

type projectCache struct {
	projectID string
	client    *api.Client
	onIdle    func(string, *projectCache)

	mu             sync.Mutex
	version        uint64
	loading        bool
	projectLoaded  bool
	sessionsLoaded bool
	watchScope     Scope
	loadedSessions map[string]struct{}
	loadedThreads  map[string]map[string]struct{}
	activeMessages map[string]string
	snapshot       Snapshot
	subscribers    map[chan Event]struct{}
	idleTimer      *time.Timer
	watchCancel    context.CancelFunc
}

func newProjectCache(projectID string, client *api.Client, onIdle func(string, *projectCache)) *projectCache {
	return &projectCache{
		projectID:      projectID,
		client:         client,
		onIdle:         onIdle,
		loadedSessions: map[string]struct{}{},
		loadedThreads:  map[string]map[string]struct{}{},
		activeMessages: map[string]string{},
		snapshot: Snapshot{
			SelectedProjectID: projectID,
			ThreadsBySession:  map[string][]api.Thread{},
			MessagesByThread:  map[string][]Message{},
		},
		subscribers: map[chan Event]struct{}{},
	}
}

func (c *projectCache) subscribe(scope Scope) (<-chan Event, func()) {
	ch := make(chan Event, 1)
	c.mu.Lock()
	c.cancelIdleLocked()
	c.subscribers[ch] = struct{}{}
	c.ensureWatchLocked(NormalizeScope(scope))
	c.mu.Unlock()
	cancel := func() {
		c.mu.Lock()
		if _, ok := c.subscribers[ch]; ok {
			delete(c.subscribers, ch)
			close(ch)
		}
		if len(c.subscribers) == 0 {
			c.startIdleLocked()
		}
		c.mu.Unlock()
	}
	return ch, cancel
}

func (c *projectCache) ensureLoaded(ctx context.Context, scope Scope) error {
	c.mu.Lock()
	ready := c.readyForLocked(scope)
	loading := c.loading
	c.mu.Unlock()
	if ready || loading {
		return nil
	}
	return c.refresh(ctx, scope, false)
}

func (c *projectCache) refresh(ctx context.Context, scope Scope, force bool) error {
	c.mu.Lock()
	c.ensureWatchLocked(NormalizeScope(scope))
	c.mu.Unlock()

	c.setLoading(true)

	c.mu.Lock()
	next := cloneSnapshot(c.snapshot)
	projectLoaded := c.projectLoaded
	sessionsLoaded := c.sessionsLoaded
	_, sessionLoaded := c.loadedSessions[scope.SessionID]
	threadLoaded := false
	if scope.SessionID != "" && scope.ThreadID != "" {
		_, threadLoaded = c.loadedThreads[scope.SessionID][scope.ThreadID]
	}
	c.mu.Unlock()

	if force || !projectLoaded {
		project, err := c.client.Projects.Get(ctx, scope.ProjectID)
		if err != nil {
			c.setError(err)
			return err
		}
		next.Projects = []api.Project{*project}
		next.SelectedProjectID = scope.ProjectID

		workspaces, err := c.client.Workspaces.List(ctx, scope.ProjectID)
		if err != nil {
			c.setError(err)
			return err
		}
		next.Workspaces = append([]api.Workspace(nil), workspaces...)

		models, err := c.client.Models.List(ctx, scope.ProjectID)
		if err != nil {
			c.setError(err)
			return err
		}
		next.Models = append([]api.ModelInfo(nil), models...)
		projectLoaded = true
	}

	if force || !sessionsLoaded {
		sessions, err := c.client.Sessions.List(ctx, scope.ProjectID)
		if err != nil {
			c.setError(err)
			return err
		}
		next.Sessions = append([]api.Session(nil), sessions...)
		sessionsLoaded = true
	}

	if scope.SessionID != "" && (force || !sessionLoaded) {
		threads, err := c.client.Sessions.ListThreads(ctx, scope.ProjectID, scope.SessionID)
		if err != nil {
			c.setError(err)
			return err
		}
		next.ThreadsBySession[scope.SessionID] = append([]api.Thread(nil), threads...)
		sessionLoaded = true
	}

	if scope.SessionID != "" && scope.ThreadID != "" && (force || !threadLoaded) {
		thread, err := c.client.Sessions.GetThread(ctx, scope.ProjectID, scope.SessionID, scope.ThreadID)
		if err == nil {
			next.ThreadsBySession[scope.SessionID] = upsertThread(next.ThreadsBySession[scope.SessionID], *thread)
		}
		threadLoaded = true
	}

	next.Ready = true
	next.Loading = false
	next.Error = ""

	c.mu.Lock()
	c.loading = false
	c.projectLoaded = projectLoaded
	c.sessionsLoaded = sessionsLoaded
	if scope.SessionID != "" && sessionLoaded {
		c.loadedSessions[scope.SessionID] = struct{}{}
	}
	if scope.SessionID != "" && scope.ThreadID != "" && threadLoaded {
		if c.loadedThreads[scope.SessionID] == nil {
			c.loadedThreads[scope.SessionID] = map[string]struct{}{}
		}
		c.loadedThreads[scope.SessionID][scope.ThreadID] = struct{}{}
	}
	c.snapshot = next
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
	return nil
}

func (c *projectCache) snapshotFor(scope Scope) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot := cloneSnapshot(c.snapshot)
	if scope.SessionID != "" {
		if threads, ok := c.snapshot.ThreadsBySession[scope.SessionID]; ok {
			snapshot.ThreadsBySession = map[string][]api.Thread{scope.SessionID: append([]api.Thread(nil), threads...)}
		}
	}
	if scope.SessionID != "" && scope.ThreadID != "" {
		key := threadKey(scope.SessionID, scope.ThreadID)
		if messages, ok := c.snapshot.MessagesByThread[key]; ok {
			snapshot.MessagesByThread = map[string][]Message{key: append([]Message(nil), messages...)}
		}
	}
	return snapshot
}

func (c *projectCache) setLoading(loading bool) {
	c.mu.Lock()
	c.loading = loading
	c.snapshot.Loading = loading
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func (c *projectCache) setError(err error) {
	c.mu.Lock()
	c.loading = false
	c.snapshot.Loading = false
	c.snapshot.Error = err.Error()
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func (c *projectCache) readyForLocked(scope Scope) bool {
	if !c.snapshot.Ready || !c.projectLoaded || !c.sessionsLoaded {
		return false
	}
	if scope.SessionID != "" {
		if _, ok := c.loadedSessions[scope.SessionID]; !ok {
			return false
		}
	}
	if scope.SessionID != "" && scope.ThreadID != "" {
		if _, ok := c.loadedThreads[scope.SessionID][scope.ThreadID]; !ok {
			return false
		}
	}
	return true
}

func (c *projectCache) nextEventLocked() Event {
	c.version++
	return Event{Version: c.version}
}

func (c *projectCache) subscriberListLocked() []chan Event {
	subscribers := make([]chan Event, 0, len(c.subscribers))
	for subscriber := range c.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	return subscribers
}

func (c *projectCache) subscriberCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.subscribers)
}

func (c *projectCache) publish(event Event, subscribers []chan Event) {
	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (c *projectCache) cancelIdle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelIdleLocked()
}

func (c *projectCache) cancelIdleLocked() {
	if c.idleTimer != nil {
		c.idleTimer.Stop()
		c.idleTimer = nil
	}
}

func (c *projectCache) startIdleLocked() {
	c.cancelIdleLocked()
	c.idleTimer = time.AfterFunc(idleTTL, func() {
		c.mu.Lock()
		if len(c.subscribers) != 0 {
			c.mu.Unlock()
			return
		}
		if c.watchCancel != nil {
			c.watchCancel()
		}
		c.mu.Unlock()
		c.onIdle(c.projectID, c)
	})
}

func (c *projectCache) ensureWatchLocked(scope Scope) {
	if c.watchCancel != nil && c.watchScope == scope {
		return
	}
	if c.watchCancel != nil {
		c.watchCancel()
		c.watchCancel = nil
	}
	c.watchScope = scope
	ctx, cancel := context.WithCancel(context.Background())
	c.watchCancel = cancel
	go c.watch(ctx, scope)
}

func (c *projectCache) watch(ctx context.Context, scope Scope) {
	for {
		if err := c.watchOnce(ctx, scope); err != nil {
			if ctx.Err() != nil {
				return
			}
			c.setError(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (c *projectCache) watchOnce(ctx context.Context, scope Scope) error {
	opts := api.ProjectStreamOptions{ProjectEvents: &api.ProjectEventsSubscriptionOptions{}}
	if scope.SessionID != "" && scope.ThreadID != "" {
		opts.Chat = &api.ChatStreamSubscriptionOptions{
			SessionID: scope.SessionID,
			ThreadID:  scope.ThreadID,
			Replay:    true,
		}
	}

	for event := range c.client.Events.WatchProjectStream(ctx, scope.ProjectID, opts) {
		switch event := event.(type) {
		case api.ProjectStreamErrorEvent:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("project stream: %s", event.Error)
		case api.ProjectConnectedEvent:
			_ = c.refresh(ctx, scope, true)
		case api.SessionUpdatedEvent:
			c.handleSessionUpdated(ctx, event.Data)
		case api.ThreadUpdatedEvent:
			c.handleThreadUpdated(ctx, event.Data)
		case api.WorkspaceUpdatedEvent:
			_ = c.refresh(ctx, scope, true)
		case api.ChatStreamEvent:
			c.handleChatEvent(event.SessionID, event.ThreadID, event.Event, event.Data)
		}
	}
	return ctx.Err()
}

func (c *projectCache) handleSessionUpdated(ctx context.Context, data api.SessionUpdatedData) {
	if data.SessionID == "" {
		return
	}
	if data.SandboxStatus == "removed" {
		c.removeSession(data.SessionID)
		return
	}
	if session, err := c.client.Sessions.Get(ctx, c.projectID, data.SessionID); err == nil {
		c.upsertSession(*session)
	}
}

func (c *projectCache) handleThreadUpdated(ctx context.Context, data api.ThreadUpdatedData) {
	if data.SessionID == "" {
		return
	}
	if session, err := c.client.Sessions.Get(ctx, c.projectID, data.SessionID); err == nil {
		c.upsertSession(*session)
	}
	if data.ThreadID != "" {
		if thread, err := c.client.Sessions.GetThread(ctx, c.projectID, data.SessionID, data.ThreadID); err == nil {
			c.upsertThread(data.SessionID, *thread)
		}
		return
	}
	if threads, err := c.client.Sessions.ListThreads(ctx, c.projectID, data.SessionID); err == nil {
		c.setThreads(data.SessionID, threads)
	}
}

func (c *projectCache) upsertSession(session api.Session) {
	c.mu.Lock()
	c.snapshot.Sessions = upsertSession(c.snapshot.Sessions, session)
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func (c *projectCache) removeSession(sessionID string) {
	c.mu.Lock()
	next := c.snapshot.Sessions[:0]
	for _, session := range c.snapshot.Sessions {
		if session.ID != sessionID {
			next = append(next, session)
		}
	}
	c.snapshot.Sessions = next
	delete(c.snapshot.ThreadsBySession, sessionID)
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func (c *projectCache) setThreads(sessionID string, threads []api.Thread) {
	c.mu.Lock()
	c.snapshot.ThreadsBySession[sessionID] = append([]api.Thread(nil), threads...)
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func (c *projectCache) upsertThread(sessionID string, thread api.Thread) {
	c.mu.Lock()
	c.snapshot.ThreadsBySession[sessionID] = upsertThread(c.snapshot.ThreadsBySession[sessionID], thread)
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func (c *projectCache) handleChatEvent(sessionID string, threadID string, eventName api.ChatStreamEventName, data json.RawMessage) {
	key := threadKey(sessionID, threadID)
	switch eventName {
	case api.ChatStreamEventHistoryStart:
		c.setMessages(key, nil)
	case api.ChatStreamEventHistoryMessage:
		if msg, ok := parseMessage(data); ok {
			c.upsertMessage(key, msg)
		}
	case api.ChatStreamEventChunk:
		c.handleChunk(key, data)
	}
}

func (c *projectCache) handleChunk(key string, data json.RawMessage) {
	var raw struct {
		Type      string `json:"type"`
		MessageID string `json:"messageId"`
		Delta     string `json:"delta"`
		Data      struct {
			Message               any    `json:"message"`
			InsertBeforeMessageID string `json:"insertBeforeMessageId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	switch raw.Type {
	case "start":
		if raw.MessageID == "" {
			return
		}
		c.mu.Lock()
		c.activeMessages[key] = raw.MessageID
		c.snapshot.MessagesByThread[key] = upsertMessage(c.snapshot.MessagesByThread[key], Message{ID: raw.MessageID, Role: "assistant"})
		event := c.nextEventLocked()
		subscribers := c.subscriberListLocked()
		c.mu.Unlock()
		c.publish(event, subscribers)
	case "text-delta":
		c.mu.Lock()
		messageID := c.activeMessages[key]
		if messageID == "" {
			c.mu.Unlock()
			return
		}
		c.snapshot.MessagesByThread[key] = appendMessageContent(c.snapshot.MessagesByThread[key], messageID, raw.Delta)
		event := c.nextEventLocked()
		subscribers := c.subscriberListLocked()
		c.mu.Unlock()
		c.publish(event, subscribers)
	case "finish", "abort":
		c.mu.Lock()
		delete(c.activeMessages, key)
		c.mu.Unlock()
	case "data-user-message":
		if raw.Data.Message == nil {
			return
		}
		messageData, err := json.Marshal(raw.Data.Message)
		if err != nil {
			return
		}
		if msg, ok := parseMessage(messageData); ok {
			c.upsertMessage(key, msg)
		}
	}
}

func (c *projectCache) setMessages(key string, messages []Message) {
	c.mu.Lock()
	c.snapshot.MessagesByThread[key] = append([]Message(nil), messages...)
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func (c *projectCache) upsertMessage(key string, msg Message) {
	c.mu.Lock()
	c.snapshot.MessagesByThread[key] = upsertMessage(c.snapshot.MessagesByThread[key], msg)
	event := c.nextEventLocked()
	subscribers := c.subscriberListLocked()
	c.mu.Unlock()
	c.publish(event, subscribers)
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	clone := snapshot
	clone.Projects = append([]api.Project(nil), snapshot.Projects...)
	clone.Workspaces = append([]api.Workspace(nil), snapshot.Workspaces...)
	clone.Models = append([]api.ModelInfo(nil), snapshot.Models...)
	clone.Sessions = append([]api.Session(nil), snapshot.Sessions...)
	clone.ThreadsBySession = map[string][]api.Thread{}
	for sessionID, threads := range snapshot.ThreadsBySession {
		clone.ThreadsBySession[sessionID] = append([]api.Thread(nil), threads...)
	}
	clone.MessagesByThread = map[string][]Message{}
	for key, messages := range snapshot.MessagesByThread {
		clone.MessagesByThread[key] = append([]Message(nil), messages...)
	}
	return clone
}

func upsertSession(sessions []api.Session, session api.Session) []api.Session {
	for i := range sessions {
		if sessions[i].ID == session.ID {
			sessions[i] = session
			return sessions
		}
	}
	return append(sessions, session)
}

func upsertThread(threads []api.Thread, thread api.Thread) []api.Thread {
	for i := range threads {
		if threads[i].ID == thread.ID {
			threads[i] = thread
			return threads
		}
	}
	return append(threads, thread)
}

func threadKey(sessionID string, threadID string) string {
	return sessionID + ":" + threadID
}

func upsertMessage(messages []Message, msg Message) []Message {
	for i := range messages {
		if messages[i].ID == msg.ID {
			if msg.Content != "" || messages[i].Content == "" {
				messages[i] = msg
			}
			return messages
		}
	}
	return append(messages, msg)
}

func parseMessage(data json.RawMessage) (Message, bool) {
	var raw struct {
		ID    string `json:"id"`
		Role  string `json:"role"`
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || raw.ID == "" {
		return Message{}, false
	}
	var content strings.Builder
	content.WriteString(raw.Content)
	for _, part := range raw.Parts {
		if part.Type == "text" || part.Type == "reasoning" {
			content.WriteString(part.Text)
		}
	}
	return Message{ID: raw.ID, Role: raw.Role, Content: content.String()}, true
}

func appendMessageContent(messages []Message, messageID string, delta string) []Message {
	for i := range messages {
		if messages[i].ID == messageID {
			messages[i].Content += delta
			return messages
		}
	}
	return append(messages, Message{ID: messageID, Role: "assistant", Content: delta})
}
