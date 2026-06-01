package server

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/obot-platform/discobot/discobot/content"
	"github.com/obot-platform/discobot/discobot/content/components/app"
	"github.com/obot-platform/discobot/discobot/internal/command"
	"github.com/obot-platform/discobot/discobot/internal/config"
	"github.com/obot-platform/discobot/discobot/internal/state"
	datasync "github.com/obot-platform/discobot/discobot/internal/sync"
)

// Server owns Discobot HTTP routing and Datastar command handlers.
type Server struct {
	config          config.Config
	logger          *slog.Logger
	mu              sync.Mutex
	data            state.Data
	view            state.View
	devReloadNeeded bool
	subscribers     map[chan struct{}]struct{}
	commands        *command.Handler
	syncManager     *datasync.Manager
}

// New wires the Discobot server dependencies and route table.
func New(cfg config.Config, logger *slog.Logger) *Server {
	server := &Server{
		config:          cfg,
		logger:          logger,
		data:            state.DefaultData(),
		view:            state.DefaultView(),
		devReloadNeeded: cfg.DevReload,
		subscribers:     map[chan struct{}]struct{}{},
	}
	server.commands = command.New(server)
	if cfg.ServerBaseURL != "" {
		if syncManager, err := datasync.NewManager(cfg.ServerBaseURL, server, logger); err != nil {
			logger.Warn("failed to create discobot data sync manager", "error", err)
		} else {
			server.syncManager = syncManager
		}
	}
	return server
}

// Handler returns the HTTP route tree for Discobot.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(noStoreDynamicUI)
	r.Get("/", s.handleRoot)
	r.Get("/ui/stream", s.handleUIStream)
	r.Mount("/ui/commands", s.commands.Routes())
	r.Handle("/*", staticFileServer(s.config))
	return r
}

// ListenAndServe starts the Discobot HTTP server.
func (s *Server) ListenAndServe() error {
	addr := ":" + s.config.Port
	if s.syncManager != nil {
		go s.syncManager.Run(context.Background())
	}
	s.logger.Info("starting discobot", "addr", addr, "staticDir", s.config.StaticDir, "serverBaseURL", s.config.ServerBaseURL)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	templ.Handler(content.Root(s.snapshot())).ServeHTTP(w, r)
}

func (s *Server) handleUIStream(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	if s.consumeDevReload() {
		if err := sse.ExecuteScript(`window.location.reload()`); err != nil {
			s.logger.Warn("failed to send dev reload script", "error", err)
		}
		return
	}

	viewEvents, cancelView := s.subscribe()
	defer cancelView()

	if err := sse.MarshalAndPatchSignals(map[string]any{"streamOpen": true}); err != nil {
		s.logger.Warn("failed to patch Datastar stream signal", "error", err)
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case _, ok := <-viewEvents:
			if !ok {
				return
			}
			if err := sse.PatchElementTempl(app.AppShell(s.snapshot())); err != nil {
				s.logger.Warn("failed to patch app shell after view update", "error", err)
				return
			}
		}
	}
}

func (s *Server) snapshot() state.Shell {
	s.mu.Lock()
	defer s.mu.Unlock()
	return state.NewShell(s.data, s.view)
}

// SaveView publishes an updated copy of the server-owned view state.
func (s *Server) SaveView(update func(*state.View)) {
	s.mu.Lock()
	shell := state.NewShell(s.data, s.view)
	update(&shell.View)
	s.view = shell.View
	subscribers := make([]chan struct{}, 0, len(s.subscribers))
	for subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

// SaveData publishes an updated copy of the server-owned application data.
// Always preserve the clone/mutate/assign pattern here so callers cannot mutate
// snapshots that may still be read concurrently by renderers or stream handlers.
func (s *Server) SaveData(update func(*state.Data)) {
	s.mu.Lock()
	shell := state.NewShell(s.data, s.view)
	update(&shell.Data)
	s.data = shell.Data
	subscribers := make([]chan struct{}, 0, len(s.subscribers))
	for subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

// SaveShell publishes updated copies of server-owned application and view state.
func (s *Server) SaveShell(update func(*state.Data, *state.View)) {
	s.mu.Lock()
	shell := state.NewShell(s.data, s.view)
	update(&shell.Data, &shell.View)
	s.data = shell.Data
	s.view = shell.View
	subscribers := make([]chan struct{}, 0, len(s.subscribers))
	for subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func (s *Server) subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
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

func (s *Server) consumeDevReload() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.devReloadNeeded {
		return false
	}
	s.devReloadNeeded = false
	return true
}

func noStoreDynamicUI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/ui/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		next.ServeHTTP(w, r)
	})
}

func staticFileServer(cfg config.Config) http.Handler {
	staticFS := cfg.StaticFS
	if staticFS == nil {
		staticFS = http.Dir(cfg.StaticDir)
	}
	files := http.FileServer(staticFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Ext(r.URL.Path) == ".js" {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		if w.Header().Get("Cache-Control") == "" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
