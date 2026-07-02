package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	api "github.com/boeing-ai-gateway/discboeing/server/api"
	"github.com/boeing-ai-gateway/discboeing/server/internal/config"
	"github.com/boeing-ai-gateway/discboeing/server/internal/conntrack"
	"github.com/boeing-ai-gateway/discboeing/server/internal/events"
	"github.com/boeing-ai-gateway/discboeing/server/internal/git"
	"github.com/boeing-ai-gateway/discboeing/server/internal/jobs"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
	"github.com/boeing-ai-gateway/discboeing/server/internal/startup"
	"github.com/boeing-ai-gateway/discboeing/server/internal/store"
	"github.com/boeing-ai-gateway/discboeing/server/internal/terminal"
)

const (
	sessionCookieName      = "discboeing_session"
	stateCookieName        = "discboeing_oauth_state"
	nonceCookieName        = "discboeing_oidc_nonce"
	pkceVerifierCookieName = "discboeing_oidc_pkce_verifier"
	returnToCookieName     = "discboeing_oidc_return_to"
)

// Handler contains all HTTP handlers
type Handler struct {
	store               *store.Store
	cfg                 *config.Config
	authService         *service.AuthService
	credentialService   *service.CredentialService
	gitService          *service.GitService
	gitProvider         git.Provider
	sandboxService      *service.SandboxService
	sessionService      *service.SessionService
	chatService         *service.ChatService
	serviceBindManager  *service.LocalhostBindManager
	modelsService       *service.ModelsService
	workspaceService    *service.WorkspaceService
	projectService      *service.ProjectService
	preferenceService   *service.PreferenceService
	jobQueue            *jobs.Queue
	eventBroker         *events.Broker
	oauthCallbackServer *OAuthCallbackServer
	systemManager       *startup.SystemManager
	terminalManager     *terminal.Manager
	shutdownCtx         context.Context
	shutdownCancel      context.CancelFunc
	shutdownOnce        sync.Once
}

// New creates a new Handler with the required application services.
func New(s *store.Store, cfg *config.Config, gitProvider git.Provider, sandboxSvc *service.SandboxService, eventBroker *events.Broker, jobQueue *jobs.Queue, systemManager *startup.SystemManager, connectionTracker *conntrack.Tracker) *Handler {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	credSvc, err := service.NewCredentialService(s, cfg)
	if err != nil {
		// This should only fail if the encryption key is invalid
		panic("failed to create credential service: " + err.Error())
	}

	var gitSvc *service.GitService
	if gitProvider != nil {
		gitSvc = service.NewGitService(s, gitProvider)
	}

	// Finish wiring the sandbox service with handler-owned dependencies.
	if sandboxSvc != nil {
		sandboxSvc.SetCredentialService(credSvc)
	}

	// Create session service
	sessionSvc := service.NewSessionService(s, gitSvc, sandboxSvc, eventBroker, jobQueue)

	// Break circular dependency: SandboxService needs SessionInitializer (which is SessionService)
	if sandboxSvc != nil {
		sandboxSvc.SetSessionInitializer(sessionSvc)
		if gitSvc != nil {
			sandboxSvc.SetGitConfigProvider(gitSvc.GetUserConfig)
		}
	}

	// Create chat service
	chatSvc := service.NewChatService(s, cfg, sessionSvc, jobQueue, eventBroker, sandboxSvc, gitSvc)
	var serviceBindManager *service.LocalhostBindManager
	if sandboxSvc != nil {
		serviceBindManager = service.NewLocalhostBindManager(sandboxSvc, connectionTracker)
	}

	// Create remaining services
	workspaceSvc := service.NewWorkspaceService(s, gitProvider, sandboxSvc, eventBroker, jobQueue)
	projectSvc := service.NewProjectService(s, sandboxSvc)
	preferenceSvc := service.NewPreferenceService(s)

	modelsSvc := service.NewModelsService(credSvc)

	h := &Handler{
		store:              s,
		cfg:                cfg,
		authService:        service.NewAuthService(s, cfg),
		credentialService:  credSvc,
		gitService:         gitSvc,
		gitProvider:        gitProvider,
		sandboxService:     sandboxSvc,
		sessionService:     sessionSvc,
		chatService:        chatSvc,
		serviceBindManager: serviceBindManager,
		modelsService:      modelsSvc,
		workspaceService:   workspaceSvc,
		projectService:     projectSvc,
		preferenceService:  preferenceSvc,
		jobQueue:           jobQueue,
		eventBroker:        eventBroker,
		systemManager:      systemManager,
		terminalManager:    terminal.NewManager(),
		shutdownCtx:        shutdownCtx,
		shutdownCancel:     shutdownCancel,
	}

	// Create localhost OAuth callback server (will be started on first use)
	h.oauthCallbackServer = NewOAuthCallbackServer(h)

	return h
}

func (h *Handler) withShutdownContext(parent context.Context) (context.Context, context.CancelFunc) {
	if h.shutdownCtx == nil {
		return parent, func() {}
	}

	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(h.shutdownCtx, cancel)

	return ctx, func() {
		stop()
		cancel()
	}
}

// BeginShutdown closes long-lived HTTP connections owned by the handler.
func (h *Handler) BeginShutdown() {
	h.shutdownOnce.Do(func() {
		if h.shutdownCancel != nil {
			h.shutdownCancel()
		}
		if h.terminalManager != nil {
			h.terminalManager.Shutdown()
		}
	})
}

// JSON helper to write JSON responses
func (h *Handler) JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// Error helper to write error responses
func (h *Handler) Error(w http.ResponseWriter, status int, message string) {
	h.JSON(w, status, api.ErrorResponse{Error: message})
}

// DecodeJSON helper to decode request body
func (h *Handler) DecodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// JobQueue returns the handler's job queue.
// Used by main.go to wire up dispatcher notifications.
func (h *Handler) JobQueue() *jobs.Queue {
	return h.jobQueue
}

// SandboxService returns the handler's sandbox service.
func (h *Handler) SandboxService() *service.SandboxService {
	return h.sandboxService
}

// EventBroker returns the handler's event broker for SSE.
func (h *Handler) EventBroker() *events.Broker {
	return h.eventBroker
}

// Close cleans up handler resources
func (h *Handler) Close() {
	h.BeginShutdown()
	if h.oauthCallbackServer != nil {
		h.oauthCallbackServer.Stop()
	}
	if h.serviceBindManager != nil {
		h.serviceBindManager.Close()
	}
}

// setSessionCookie sets the session cookie
func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		SameSite: h.cfg.CookieSameSite(),
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	})
}

// clearSessionCookie clears the session cookie
func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		MaxAge:   -1,
	})
}

// getSessionToken gets the session token from cookie
func (h *Handler) getSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// setStateCookie sets the OAuth state cookie
func (h *Handler) setStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		SameSite: h.cfg.CookieSameSite(),
		MaxAge:   10 * 60, // 10 minutes
	})
}

// getStateCookie gets and clears the OAuth state cookie
func (h *Handler) getStateCookie(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return ""
	}
	// Clear the cookie
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		MaxAge:   -1,
	})
	return cookie.Value
}

// setNonceCookie sets the OIDC nonce cookie.
func (h *Handler) setNonceCookie(w http.ResponseWriter, nonce string) {
	http.SetCookie(w, &http.Cookie{
		Name:     nonceCookieName,
		Value:    nonce,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		SameSite: h.cfg.CookieSameSite(),
		MaxAge:   10 * 60,
	})
}

// getNonceCookie gets and clears the OIDC nonce cookie.
func (h *Handler) getNonceCookie(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(nonceCookieName)
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     nonceCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		MaxAge:   -1,
	})
	return cookie.Value
}

// setPKCEVerifierCookie sets the OIDC PKCE verifier cookie.
func (h *Handler) setPKCEVerifierCookie(w http.ResponseWriter, verifier string) {
	http.SetCookie(w, &http.Cookie{
		Name:     pkceVerifierCookieName,
		Value:    verifier,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		SameSite: h.cfg.CookieSameSite(),
		MaxAge:   10 * 60,
	})
}

// getPKCEVerifierCookie gets and clears the OIDC PKCE verifier cookie.
func (h *Handler) getPKCEVerifierCookie(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(pkceVerifierCookieName)
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     pkceVerifierCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		MaxAge:   -1,
	})
	return cookie.Value
}

// setReturnToCookie stores the UI return URL for the OIDC flow.
func (h *Handler) setReturnToCookie(w http.ResponseWriter, returnTo string) {
	http.SetCookie(w, &http.Cookie{
		Name:     returnToCookieName,
		Value:    returnTo,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		SameSite: h.cfg.CookieSameSite(),
		MaxAge:   10 * 60,
	})
}

// getReturnToCookie gets and clears the OIDC return URL cookie.
func (h *Handler) getReturnToCookie(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(returnToCookieName)
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     returnToCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookiesSecure(),
		MaxAge:   -1,
	})
	return cookie.Value
}
