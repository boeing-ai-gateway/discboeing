package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/obot-platform/discobot/ui-go2/content"
	"github.com/obot-platform/discobot/ui-go2/content/lib/components/app"
	"github.com/obot-platform/discobot/ui-go2/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go2/internal/config"
)

// Server owns ui-go2 HTTP routing and Datastar command handlers.
type Server struct {
	config config.Config
	logger *slog.Logger
	mu     sync.Mutex
	state  viewmodel.ShellSnapshot
}

// New wires the ui-go2 server dependencies and route table.
func New(cfg config.Config, logger *slog.Logger) *Server {
	return &Server{
		config: cfg,
		logger: logger,
		state:  viewmodel.DefaultShell(),
	}
}

// Handler returns the HTTP route tree for ui-go2.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(noStoreDynamicUI)
	r.Get("/", s.handleRoot)
	r.Post("/ui/commands/greet", s.handleGreet)
	r.Handle("/*", staticFileServer(s.config.StaticDir))
	return r
}

// ListenAndServe starts the ui-go2 HTTP server.
func (s *Server) ListenAndServe() error {
	addr := ":" + s.config.Port
	s.logger.Info("starting ui-go2", "addr", addr, "staticDir", s.config.StaticDir)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	templ.Handler(content.Root(s.snapshot())).ServeHTTP(w, r)
}

func (s *Server) handleGreet(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Subject string `json:"subject"`
	}
	if err := json.NewDecoder(r.Body).Decode(&signals); err != nil && r.Body != nil {
		s.logger.Debug("failed to decode Datastar signals", "error", err)
	}

	s.mu.Lock()
	if strings.TrimSpace(signals.Subject) != "" {
		s.state.Greeting.Subject = strings.TrimSpace(signals.Subject)
	}
	s.state.Greeting.Count++
	snapshot := s.state
	s.mu.Unlock()

	sse := datastar.NewSSE(w, r)
	if err := sse.PatchElementTempl(app.AppShell(snapshot)); err != nil {
		s.logger.Warn("failed to patch app shell", "error", err)
	}
}

func (s *Server) snapshot() viewmodel.ShellSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
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

func staticFileServer(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if w.Header().Get("Cache-Control") == "" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
