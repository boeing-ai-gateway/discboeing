package handler

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/oauth"
	"github.com/obot-platform/discobot/server/internal/service"
)

const (
	oauthCallbackPort = 1455
	oauthCallbackAddr = "127.0.0.1:1455"
)

type pendingOAuthAuth struct {
	provider     string
	domain       string
	state        string
	verifier     string
	projectID    string
	redirectURI  string
	credentialID string
	name         string
	description  string
	visibility   service.CredentialVisibility
	inactive     bool
	createdAt    time.Time
}

type completedOAuthAuth struct {
	status    string
	error     string
	createdAt time.Time
}

// OAuthCallbackServer handles localhost OAuth callbacks on 127.0.0.1:1455.
type OAuthCallbackServer struct {
	handler       *Handler
	server        *http.Server
	listener      net.Listener
	mu            sync.Mutex
	pending       map[string]*pendingOAuthAuth
	completed     map[string]*completedOAuthAuth
	running       bool
	cleanupTicker *time.Ticker
	cleanupDone   chan struct{}
}

func NewOAuthCallbackServer(h *Handler) *OAuthCallbackServer {
	return &OAuthCallbackServer{
		handler:   h,
		pending:   make(map[string]*pendingOAuthAuth),
		completed: make(map[string]*completedOAuthAuth),
	}
}

// Start attempts to start the callback server. Returns true if started successfully.
func (s *OAuthCallbackServer) Start() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return true
	}

	listener, err := net.Listen("tcp", oauthCallbackAddr)
	if err != nil {
		log.Printf("OAuth callback server: could not listen on %s: %v (manual code entry will be required)", oauthCallbackAddr, err)
		return false
	}

	s.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", s.handleCallback)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	s.cleanupTicker = time.NewTicker(1 * time.Minute)
	s.cleanupDone = make(chan struct{})
	go s.cleanupExpired()

	go func() {
		log.Printf("OAuth callback server listening on %s", oauthCallbackAddr)
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("OAuth callback server error: %v", err)
		}
	}()

	s.running = true
	return true
}

func (s *OAuthCallbackServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
		close(s.cleanupDone)
	}

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(ctx)
	}

	s.running = false
	log.Printf("OAuth callback server stopped")
}

func (s *OAuthCallbackServer) RegisterPendingCodex(state, verifier, projectID, redirectURI string) {
	s.registerPending(&pendingOAuthAuth{
		provider:    service.ProviderCodex,
		state:       state,
		verifier:    verifier,
		projectID:   projectID,
		redirectURI: redirectURI,
		createdAt:   time.Now(),
	})
}

func (s *OAuthCallbackServer) RegisterPendingGitHub(
	state, verifier, projectID, redirectURI, domain, credentialID, name, description string,
	visibility service.CredentialVisibility,
	inactive bool,
) {
	s.registerPending(&pendingOAuthAuth{
		provider:     service.ProviderGitHub,
		domain:       domain,
		state:        state,
		verifier:     verifier,
		projectID:    projectID,
		redirectURI:  redirectURI,
		credentialID: credentialID,
		name:         name,
		description:  description,
		visibility:   visibility,
		inactive:     inactive,
		createdAt:    time.Now(),
	})
}

func (s *OAuthCallbackServer) registerPending(auth *pendingOAuthAuth) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.completed, auth.state)
	s.pending[auth.state] = auth
}

func (s *OAuthCallbackServer) cleanupExpired() {
	for {
		select {
		case <-s.cleanupTicker.C:
			s.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for state, auth := range s.pending {
				if auth.createdAt.Before(cutoff) {
					delete(s.pending, state)
				}
			}
			for state, auth := range s.completed {
				if auth.createdAt.Before(cutoff) {
					delete(s.completed, state)
				}
			}
			s.mu.Unlock()
		case <-s.cleanupDone:
			return
		}
	}
}

func (s *OAuthCallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")

	if errorParam != "" {
		s.complete(state, "error", fmt.Sprintf("Authorization failed: %s - %s", errorParam, errorDesc))
		s.renderError(w, fmt.Sprintf("Authorization failed: %s - %s", errorParam, errorDesc))
		return
	}
	if code == "" {
		s.renderError(w, "No authorization code received")
		return
	}
	if state == "" {
		s.renderError(w, "No state parameter received")
		return
	}

	s.mu.Lock()
	auth, ok := s.pending[state]
	if ok {
		delete(s.pending, state)
	}
	s.mu.Unlock()

	if !ok {
		s.renderCodeForCopy(w, code)
		return
	}

	providerName := "OAuth"
	var err error
	switch auth.provider {
	case service.ProviderCodex:
		providerName = "OpenAI"
		err = s.completeCodex(r.Context(), auth, code)
	case service.ProviderGitHub:
		providerName = "GitHub"
		err = s.completeGitHub(r.Context(), auth, code)
	default:
		err = fmt.Errorf("unsupported OAuth provider: %s", auth.provider)
	}
	if err != nil {
		s.complete(state, "error", err.Error())
		s.renderError(w, err.Error())
		return
	}

	s.complete(state, "success", "")
	s.renderSuccess(w, providerName)
}

func (s *OAuthCallbackServer) completeCodex(ctx context.Context, auth *pendingOAuthAuth, code string) error {
	provider := oauth.NewCodexProvider(s.handler.cfg.CodexClientID)
	tokenResp, err := provider.Exchange(ctx, code, auth.redirectURI, auth.verifier)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	oauthCred := &service.OAuthCredential{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    tokenResp.ExpiresAt,
		Scope:        tokenResp.Scope,
	}
	_, err = s.handler.credentialService.SetOAuthTokens(ctx, auth.projectID, service.ProviderCodex, "OpenAI Codex", oauthCred)
	if err != nil {
		return fmt.Errorf("failed to store credential: %w", err)
	}
	return nil
}

func (s *OAuthCallbackServer) completeGitHub(ctx context.Context, auth *pendingOAuthAuth, code string) error {
	provider := oauth.NewGitHubProvider(
		s.handler.cfg.GitHubOAuthClientID,
		s.handler.cfg.GitHubOAuthClientSecret,
		auth.domain,
		nil,
	)
	tokenResp, err := provider.Exchange(ctx, code, auth.redirectURI, auth.verifier)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	oauthCred := &service.OAuthCredential{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		Scope:       tokenResp.Scope,
	}
	_, err = s.handler.credentialService.SetOAuthTokensWithMetadata(
		ctx,
		auth.projectID,
		auth.credentialID,
		service.ProviderGitHub,
		auth.name,
		auth.description,
		auth.visibility,
		auth.inactive,
		oauthCred,
	)
	if err != nil {
		return fmt.Errorf("failed to store credential: %w", err)
	}
	return nil
}

func (s *OAuthCallbackServer) complete(state, status, errMsg string) {
	if state == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pending, state)
	s.completed[state] = &completedOAuthAuth{
		status:    status,
		error:     errMsg,
		createdAt: time.Now(),
	}
}

func (s *OAuthCallbackServer) Status(state string) (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if completed, ok := s.completed[state]; ok {
		return completed.status, completed.error
	}
	if _, ok := s.pending[state]; ok {
		return "pending", ""
	}
	return "pending", ""
}

func (s *OAuthCallbackServer) renderSuccess(w http.ResponseWriter, providerName string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authorization Successful</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f5f5; }
        .container { text-align: center; padding: 2rem; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #22c55e; margin-bottom: 1rem; }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h1>✓ Authorization Successful!</h1>
        <p>You can close this window and return to Discobot.</p>
        <p>Your %s credential has been saved.</p>
    </div>
</body>
</html>`, providerName)
}

func (s *OAuthCallbackServer) renderError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authorization Failed</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f5f5; }
        .container { text-align: center; padding: 2rem; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); max-width: 500px; }
        h1 { color: #ef4444; margin-bottom: 1rem; }
        p { color: #666; }
        .error { background: #fef2f2; color: #dc2626; padding: 1rem; border-radius: 4px; margin: 1rem 0; word-break: break-word; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Authorization Failed</h1>
        <div class="error">%s</div>
        <p>Please close this window and try again.</p>
    </div>
</body>
</html>`, message)
}

func (s *OAuthCallbackServer) renderCodeForCopy(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authorization Code</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f5f5; }
        .container { text-align: center; padding: 2rem; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); max-width: 500px; }
        h1 { color: #22c55e; margin-bottom: 1rem; }
        p { color: #666; }
        .code { background: #f3f4f6; padding: 1rem; border-radius: 4px; margin: 1rem 0; font-family: monospace; word-break: break-all; }
        button { background: #3b82f6; color: white; border: none; padding: 0.75rem 1.5rem; border-radius: 4px; cursor: pointer; font-size: 1rem; }
        button:hover { background: #2563eb; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Authorization Code Received</h1>
        <p>Copy this code and paste it back into Discobot:</p>
        <div class="code" id="code">%s</div>
        <button onclick="navigator.clipboard.writeText(document.getElementById('code').textContent)">Copy Code</button>
    </div>
</body>
</html>`, code)
}
