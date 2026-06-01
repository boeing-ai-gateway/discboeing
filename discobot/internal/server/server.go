package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/obot-platform/discobot/discobot/content"
	"github.com/obot-platform/discobot/discobot/content/components/app"
	"github.com/obot-platform/discobot/discobot/internal/command"
	"github.com/obot-platform/discobot/discobot/internal/config"
	"github.com/obot-platform/discobot/discobot/internal/state"
	datasync "github.com/obot-platform/discobot/discobot/internal/sync"
	serviceclient "github.com/obot-platform/discobot/server/client"
)

// Server owns Discobot HTTP routing and Datastar command handlers.
type Server struct {
	config      config.Config
	logger      *slog.Logger
	mu          sync.Mutex
	data        state.Data
	sessions    *sessionStore
	devReloadID string
	subscribers map[chan struct{}]struct{}
	commands    *command.Handler
	syncManager dataSyncManager
}

type sessionContextKey struct{}

type dataSyncManager interface {
	Run(context.Context)
}

// New wires the Discobot server dependencies and route table.
func New(cfg config.Config, logger *slog.Logger) *Server {
	server := &Server{
		config:      cfg,
		logger:      logger,
		data:        state.DefaultData(),
		sessions:    newSessionStore(cfg.SessionDir, logger),
		devReloadID: time.Now().UTC().Format(time.RFC3339Nano),
		subscribers: map[chan struct{}]struct{}{},
	}
	var client *serviceclient.Client
	if cfg.ServerBaseURL != "" {
		if strings.HasPrefix(cfg.ServerBaseURL, "file://") {
			if syncManager, err := datasync.NewFileManager(cfg.ServerBaseURL, server, logger); err != nil {
				logger.Warn("failed to create discobot file data sync manager", "error", err)
			} else {
				server.syncManager = syncManager
			}
		} else {
			if commandClient, err := serviceclient.NewClient(cfg.ServerBaseURL); err != nil {
				logger.Warn("failed to create discobot command API client", "error", err)
			} else {
				client = commandClient
			}
			if syncManager, err := datasync.NewManager(cfg.ServerBaseURL, server, logger); err != nil {
				logger.Warn("failed to create discobot data sync manager", "error", err)
			} else {
				server.syncManager = syncManager
			}
		}
	}
	server.commands = command.New(server, command.WithClient(client))
	return server
}

// Handler returns the HTTP route tree for Discobot.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(noStoreDynamicUI)
	r.Use(s.withSession)
	r.Get("/", s.handleRoot)
	r.Get("/ui/stream", s.handleUIStream)
	r.Mount("/ui/commands", s.commands.Routes())
	r.Handle("/*", staticFileServer(s.config))
	return r
}

// ListenAndServe starts the Discobot HTTP server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := ":" + s.config.Port
	if s.syncManager != nil {
		go s.syncManager.Run(ctx)
	}
	s.logger.Info("starting discobot", "addr", addr, "staticDir", s.config.StaticDir, "serverBaseURL", s.config.ServerBaseURL)
	server := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to shut down discobot server", "error", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	s.setDevReloadCookie(w)
	templ.Handler(content.Root(s.snapshot(r.Context()))).ServeHTTP(w, r)
}

func (s *Server) handleUIStream(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	if s.shouldDevReload(r) {
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
			if err := sse.PatchElementTempl(app.AppShell(s.snapshot(r.Context()))); err != nil {
				s.logger.Warn("failed to patch app shell after view update", "error", err)
				return
			}
		}
	}
}

func (s *Server) snapshot(ctx context.Context) state.Shell {
	s.mu.Lock()
	data := s.data
	s.mu.Unlock()
	return state.NewShell(data, s.sessions.view(sessionIDFromContext(ctx)))
}

// SaveView publishes an updated copy of the request session's view state.
func (s *Server) SaveView(ctx context.Context, update func(*state.View)) {
	sessionID := sessionIDFromContext(ctx)
	s.mu.Lock()
	data := s.data
	s.mu.Unlock()
	s.sessions.save(sessionID, func(view *state.View) {
		shell := state.NewShell(data, *view)
		update(&shell.View)
		*view = shell.View
	})
	s.publish()
}

// SaveData publishes an updated copy of the server-owned application data.
// Always preserve the clone/mutate/assign pattern here so callers cannot mutate
// snapshots that may still be read concurrently by renderers or stream handlers.
func (s *Server) SaveData(_ context.Context, update func(*state.Data)) {
	s.mu.Lock()
	shell := state.NewShell(s.data, state.DefaultView())
	update(&shell.Data)
	s.data = shell.Data
	s.mu.Unlock()
	s.publish()
}

// SaveShell publishes updated copies of server-owned application and view state.
func (s *Server) SaveShell(ctx context.Context, update func(*state.Data, *state.View)) {
	sessionID := sessionIDFromContext(ctx)
	s.mu.Lock()
	s.sessions.save(sessionID, func(view *state.View) {
		shell := state.NewShell(s.data, *view)
		update(&shell.Data, &shell.View)
		s.data = shell.Data
		*view = shell.View
	})
	s.mu.Unlock()
	s.publish()
}

func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/ui/") {
			sessionID := s.sessions.sessionID(w, r)
			r = r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sessionID))
		}
		next.ServeHTTP(w, r)
	})
}

func sessionIDFromContext(ctx context.Context) string {
	sessionID, ok := ctx.Value(sessionContextKey{}).(string)
	if !ok || sessionID == "" {
		panic("discobot session missing from request context")
	}
	return sessionID
}

func (s *Server) publish() {
	s.mu.Lock()
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

const devReloadCookie = "discobot_dev_reload_id"

func (s *Server) setDevReloadCookie(w http.ResponseWriter) {
	if !s.config.DevReload {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     devReloadCookie,
		Value:    s.devReloadID,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) shouldDevReload(r *http.Request) bool {
	if !s.config.DevReload {
		return false
	}
	cookie, err := r.Cookie(devReloadCookie)
	if err != nil {
		return true
	}
	return cookie.Value != s.devReloadID
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
