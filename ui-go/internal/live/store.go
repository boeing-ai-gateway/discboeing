package live

import (
	"context"
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
	return cache.subscribe()
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
	loadedSessions map[string]struct{}
	loadedThreads  map[string]map[string]struct{}
	snapshot       Snapshot
	subscribers    map[chan Event]struct{}
	idleTimer      *time.Timer
	watchCancel    context.CancelFunc
}

func newProjectCache(projectID string, client *api.Client, onIdle func(string, *projectCache)) *projectCache {
	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx
	return &projectCache{
		projectID:      projectID,
		client:         client,
		onIdle:         onIdle,
		loadedSessions: map[string]struct{}{},
		loadedThreads:  map[string]map[string]struct{}{},
		snapshot: Snapshot{
			SelectedProjectID: projectID,
			ThreadsBySession:  map[string][]api.Thread{},
		},
		subscribers: map[chan Event]struct{}{},
		watchCancel: cancel,
	}
}

func (c *projectCache) subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 1)
	c.mu.Lock()
	c.cancelIdleLocked()
	c.subscribers[ch] = struct{}{}
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
	return clone
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
