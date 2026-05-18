package state

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// ViewState is browser-session-scoped UI state. It is persisted independently
// from backend live data; the read model combines this state with live.Snapshot
// when rendering ShellSnapshot.
type ViewState = viewmodel.ShellSnapshot

// Event records a saved session view update.
type Event struct {
	SessionID string
	Version   uint64
}

// Store owns session-scoped frontend view state.
type Store struct {
	mu          sync.Mutex
	defaultView func(string) ViewState
	sessions    map[string]*Session
}

// Session is one browser session's logical frontend application state.
type Session struct {
	id        string
	mu        sync.Mutex
	viewBytes []byte
	version   uint64

	subscribers map[chan Event]struct{}
}

// New returns an empty session store.
func New() *Store {
	return NewWithDefault(defaultView)
}

// NewWithDefault returns a session store using defaultView for new sessions.
func NewWithDefault(defaultView func(string) ViewState) *Store {
	return &Store{
		defaultView: defaultView,
		sessions:    map[string]*Session{},
	}
}

// Session returns the session state for id, creating the default view on first use.
func (s *Store) Session(id string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[id]
	if session == nil {
		session = &Session{
			id:          id,
			viewBytes:   mustEncodeView(s.defaultView(id)),
			subscribers: map[chan Event]struct{}{},
		}
		s.sessions[id] = session
	}
	return session
}

// View returns a copy of the session's current frontend view state.
func (s *Session) View() ViewState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return mustDecodeView(s.viewBytes)
}

// Save mutates the session view state and publishes a save event for stream listeners.
func (s *Session) Save(update func(*ViewState)) Event {
	s.mu.Lock()
	view := mustDecodeView(s.viewBytes)
	update(&view)
	s.viewBytes = mustEncodeView(view)
	s.version++
	event := Event{SessionID: s.id, Version: s.version}
	subscribers := make([]chan Event, 0, len(s.subscribers))
	for subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	return event
}

// Subscribe returns a channel that receives session save events until cancel is called.
func (s *Session) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, cancel
}

// RecordStreamPatch updates stream patch metadata without publishing another event.
func (s *Session) RecordStreamPatch() viewmodel.AppSidebarSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := mustDecodeView(s.viewBytes)
	current, _ := parseUintLabel(view.Sidebar.StreamEvents)
	current++
	view.Sidebar.StreamEvents = fmt.Sprintf("%d", current)
	s.viewBytes = mustEncodeView(view)
	return view.Sidebar
}

func defaultView(string) ViewState {
	return ViewState{
		Header: viewmodel.HeaderSnapshot{
			ShowSessionToolbar: false,
			SessionTitle:       "Discobot",
			ShowRefreshButton:  true,
		},
		Sidebar: viewmodel.AppSidebarSnapshot{
			ShowRecentThreads:  false,
			ShowAllHeader:      true,
			GroupedByWorkspace: true,
			StreamEvents:       "0",
			Commands:           "0",
		},
		Workspace: viewmodel.SessionWorkspaceSnapshot{
			Title:          "No session selected",
			State:          "Loading",
			Message:        "Loading backend data…",
			ReserveSidebar: false,
			Composer: viewmodel.ConversationComposerSnapshot{
				Placeholder:     "Loading Discobot…",
				DisabledMessage: "Loading Discobot…",
				SubmitStatus:    "disabled",
				ModelLabel:      "Model",
			},
			Conversation: viewmodel.ConversationPaneSnapshot{
				Status:       "ready",
				ShowComposer: false,
			},
		},
	}
}

func parseUintLabel(value string) (uint64, bool) {
	var parsed uint64
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		parsed = parsed*10 + uint64(r-'0')
	}
	return parsed, true
}

func mustEncodeView(view ViewState) []byte {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(view); err != nil {
		panic(fmt.Sprintf("encode ui-go session view: %v", err))
	}
	return buf.Bytes()
}

func mustDecodeView(data []byte) ViewState {
	var view ViewState
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&view); err != nil {
		panic(fmt.Sprintf("decode ui-go session view: %v", err))
	}
	return view
}
