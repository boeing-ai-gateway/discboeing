package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content"
	"github.com/obot-platform/discobot/ui-go/content/lib/components/app"
	"github.com/obot-platform/discobot/ui-go/internal/command"
	"github.com/obot-platform/discobot/ui-go/internal/config"
	"github.com/obot-platform/discobot/ui-go/internal/live"
	"github.com/obot-platform/discobot/ui-go/internal/readmodel"
	uisession "github.com/obot-platform/discobot/ui-go/internal/session"
	"github.com/obot-platform/discobot/ui-go/internal/state"
)

// Server owns ui-go HTTP routing and long-lived stream handlers.
type Server struct {
	config   config.Config
	client   *api.Client
	store    *state.Store
	live     *live.Store
	commands *command.Handler
	logger   *slog.Logger
}

// New wires the ui-go server dependencies and route table.
func New(cfg config.Config, logger *slog.Logger) *Server {
	client, err := api.NewClient(cfg.APIBaseURL)
	if err != nil {
		logger.Error("failed to create Discobot API client", "baseURL", cfg.APIBaseURL, "error", err)
		client, _ = api.NewClient("http://127.0.0.1:3001")
	}
	store := state.New()
	liveStore := live.New(client)
	return &Server{
		config:   cfg,
		client:   client,
		store:    store,
		live:     liveStore,
		commands: command.New(store, liveStore, client, logger),
		logger:   logger,
	}
}

// Handler returns the HTTP route tree for ui-go.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(noStoreDynamicUI)
	r.Group(func(r chi.Router) {
		r.Use(s.ensureSession)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			sessionState, ok := s.session(r)
			if !ok {
				http.Error(w, "missing session", http.StatusBadRequest)
				return
			}
			sessionID, _ := uisession.ID(r.Context())
			view := sessionState.View()
			templ.Handler(content.Root(readmodel.BuildShellFromBackend(view, s.live.Snapshot(readmodel.LiveScopeFromView(view))), sessionID)).ServeHTTP(w, r)
		})
		r.Get("/ui/stream", s.handleUIStream)
		r.Post("/ui/commands/sidebar-refresh", s.commands.SidebarRefresh)
		r.Post("/ui/commands/sidebar/new-session", s.commands.SidebarNewSession)
		r.Post("/ui/commands/sidebar/select-session", s.commands.SidebarSelectSession)
		r.Post("/ui/commands/sidebar/select-thread", s.commands.SidebarSelectThread)
		r.Post("/ui/commands/sidebar/toggle-collapsed", s.commands.SidebarToggleCollapsed)
		r.Post("/ui/commands/sidebar/toggle-floating", s.commands.SidebarToggleFloating)
		r.Post("/ui/commands/sidebar/toggle-grouping", s.commands.SidebarToggleGrouping)
		r.Post("/ui/commands/sidebar/toggle-section", s.commands.SidebarToggleSection)
		r.Post("/ui/commands/sidebar/session-menu", s.commands.SidebarSessionMenu)
		r.Post("/ui/commands/sidebar/thread-menu", s.commands.SidebarThreadMenu)
		r.Post("/ui/commands/sidebar/workspace-menu", s.commands.SidebarWorkspaceMenu)
		r.Post("/ui/commands/sidebar/close-menu", s.commands.SidebarCloseMenu)
		r.Post("/ui/commands/sidebar/session-action", s.commands.SidebarSessionAction)
		r.Post("/ui/commands/sidebar/thread-action", s.commands.SidebarThreadAction)
		r.Post("/ui/commands/sidebar/workspace-action", s.commands.SidebarWorkspaceAction)
		r.Post("/ui/commands/sidebar/rename", s.commands.SidebarRename)
		r.Post("/ui/commands/sidebar/delete", s.commands.SidebarDelete)
		r.Post("/ui/commands/message/branch", s.commands.MessageBranch)
		r.Post("/ui/commands/prompt-queue/action", s.commands.PromptQueueAction)
		r.Post("/ui/commands/credentials/action", s.commands.CredentialsAction)
		r.Post("/ui/commands/credentials/env-var", s.commands.CredentialEnvVarAction)
		r.Post("/ui/commands/credentials/oauth-scopes", s.commands.CredentialOAuthScopesAction)
		r.Post("/ui/commands/credentials/oauth-wizard", s.commands.CredentialOAuthWizardAction)
		r.Post("/ui/commands/settings/action", s.commands.SettingsAction)
		r.Post("/ui/commands/composer-attachments", s.commands.ComposerAttachments)
		r.Post("/ui/commands/composer-attachment-remove", s.commands.ComposerAttachmentRemove)
		r.Post("/ui/commands/composer-model", s.commands.ComposerModel)
		r.Post("/ui/commands/composer-reasoning", s.commands.ComposerReasoning)
		r.Post("/ui/commands/composer-service-tier", s.commands.ComposerServiceTier)
		r.Post("/ui/commands/composer-schedule", s.commands.ComposerSchedule)
		r.Post("/ui/commands/composer-workspace", s.commands.ComposerWorkspace)
		r.Post("/ui/commands/composer-submit", s.commands.ComposerSubmit)
		r.Post("/ui/commands/composer-stop", s.commands.ComposerStop)
	})
	r.Handle("/*", staticFileServer(s.config.StaticDir))
	return r
}

// ListenAndServe starts the ui-go HTTP server.
func (s *Server) ListenAndServe() error {
	addr := ":" + s.config.Port
	s.logger.Info("starting ui-go", "addr", addr, "staticDir", s.config.StaticDir)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleUIStream(w http.ResponseWriter, r *http.Request) {
	sessionState, ok := s.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)
	scope := readmodel.LiveScopeFromView(sessionState.View())
	if err := sse.MarshalAndPatchSignals(map[string]any{"streamOpen": true}); err != nil {
		s.logger.Warn("failed to patch Datastar stream signal", "error", err)
		return
	}
	if err := sse.PatchElementTempl(app.AppSidebar(readmodel.BuildShellFromBackend(sessionState.View(), s.live.Snapshot(scope)).Sidebar)); err != nil {
		s.logger.Warn("failed to patch Datastar sidebar", "error", err)
		return
	}

	viewEvents, cancelView := sessionState.Subscribe()
	defer cancelView()
	liveEvents, cancelLive := s.live.Subscribe(scope)
	defer func() {
		cancelLive()
	}()
	go func() {
		if err := s.live.EnsureLoaded(r.Context(), scope); err != nil {
			s.logger.Warn("failed to load live backend data", "error", err)
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-viewEvents:
			if !ok {
				return
			}
			if event.SessionID == "" {
				continue
			}
			nextScope := readmodel.LiveScopeFromView(sessionState.View())
			if nextScope != scope {
				cancelLive()
				scope = nextScope
				liveEvents, cancelLive = s.live.Subscribe(scope)
				go func(scope live.Scope) {
					if err := s.live.EnsureLoaded(r.Context(), scope); err != nil {
						s.logger.Warn("failed to load scoped live backend data", "error", err)
					}
				}(scope)
			}
			sessionState.RecordStreamPatch()
			if err := sse.PatchElementTempl(app.AppShell(readmodel.BuildShellFromBackend(sessionState.View(), s.live.Snapshot(scope)))); err != nil {
				s.logger.Warn("failed to patch Datastar app shell", "error", err)
				return
			}
		case _, ok := <-liveEvents:
			if !ok {
				return
			}
			sessionState.RecordStreamPatch()
			if err := sse.PatchElementTempl(app.AppShell(readmodel.BuildShellFromBackend(sessionState.View(), s.live.Snapshot(scope)))); err != nil {
				s.logger.Warn("failed to patch Datastar app shell from live data", "error", err)
				return
			}
		}
	}
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
		setStaticCacheHeaders(w, r.URL.Path)
		files.ServeHTTP(w, r)
	})
}

func setStaticCacheHeaders(w http.ResponseWriter, path string) {
	if w.Header().Get("Cache-Control") != "" {
		return
	}
	if strings.HasPrefix(path, "/assets/chunks/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

func (s *Server) ensureSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ""
		if cookie, err := r.Cookie(uisession.CookieName); err == nil {
			id = cookie.Value
		}
		if id == "" {
			id = r.URL.Query().Get(uisession.QueryParam)
		}
		if id == "" {
			id = r.Header.Get(uisession.HeaderName)
		}
		if id == "" {
			var err error
			id, err = newSessionID()
			if err != nil {
				s.logger.Error("failed to generate ui-go session ID", "error", err)
				http.Error(w, "failed to create session", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     uisession.CookieName,
				Value:    id,
				Path:     "/",
				MaxAge:   60 * 60 * 24 * 30,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}
		if !validSessionID(id) {
			http.Error(w, "invalid session", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r.WithContext(uisession.WithID(r.Context(), id)))
	})
}

func (s *Server) session(r *http.Request) (*state.Session, bool) {
	id, ok := uisession.ID(r.Context())
	if !ok {
		return nil, false
	}
	return s.store.Session(id), true
}

func newSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func validSessionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
