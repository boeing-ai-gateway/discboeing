// Package server implements the HTTP API server mode for agent-api.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/config"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/credentials"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/middleware"
)

const agentNotConfiguredCode = "AGENT_NOT_CONFIGURED"

type configureRequest struct {
	AgentCwd               string               `json:"agentCwd"`
	Model                  string               `json:"model"`
	WorkspaceOrigin        string               `json:"workspaceOrigin"`
	WorkspaceSource        string               `json:"workspaceSource"`
	WorkspaceSourceType    string               `json:"workspaceSourceType"`
	WorkspaceCommit        string               `json:"workspaceCommit"`
	WorkspaceTargetRef     string               `json:"workspaceTargetRef"`
	DataDir                string               `json:"dataDir"`
	ThreadsDir             string               `json:"threadsDir"`
	HooksEnabled           *bool                `json:"hooksEnabled"`
	SessionID              string               `json:"sessionId"`
	MCPOAuthRedirectBase   string               `json:"mcpOAuthRedirectBase"`
	DiscboeingServerURL      string               `json:"discboeingServerUrl"`
	DiscboeingProjectID      string               `json:"discboeingProjectId"`
	EnableGitControlSocket bool                 `json:"enableGitControlSocket"`
	Credentials            []credentials.EnvVar `json:"credentials,omitempty"`
	GitUserName            string               `json:"gitUserName,omitempty"`
	GitUserEmail           string               `json:"gitUserEmail,omitempty"`
}

type configureEvent struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type bootstrapState struct {
	mu      sync.RWMutex
	handler http.Handler
	cleanup func()
}

func (s *bootstrapState) configured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handler != nil
}

func (s *bootstrapState) set(handler http.Handler, cleanup func()) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.handler != nil {
		return false
	}
	s.handler = handler
	s.cleanup = cleanup
	return true
}

func (s *bootstrapState) close() {
	s.mu.Lock()
	cleanup := s.cleanup
	s.cleanup = nil
	s.handler = nil
	s.mu.Unlock()
	if cleanup != nil {
		cleanup()
	}
}

func (s *bootstrapState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	h := s.handler
	s.mu.RUnlock()
	if h == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "agent is waiting for configuration",
			"code":  agentNotConfiguredCode,
		})
		return
	}
	h.ServeHTTP(w, r)
}

func (s *bootstrapState) configure(base *config.Config, w http.ResponseWriter, r *http.Request) {
	if s.configured() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "agent is already configured"})
		return
	}

	var req configureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid configure request"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	emit := func(event configureEvent) {
		data, _ := json.Marshal(event)
		_, _ = fmt.Fprintf(w, "event: configure\ndata: %s\n\n", data)
		flusher.Flush()
	}

	emit(configureEvent{Status: "configuring", Message: "applying configuration"})
	cfg := applyConfigureRequest(base, req)
	emit(configureEvent{Status: "configuring", Message: "setting up workspace"})
	initialCreds := runtimeInitialCredentials{
		Credentials:  req.Credentials,
		GitUserName:  req.GitUserName,
		GitUserEmail: req.GitUserEmail,
	}
	if err := setupConfiguredWorkspace(r.Context(), cfg, initialCreds, func(message string) {
		emit(configureEvent{Status: "configuring", Message: message})
	}); err != nil {
		emit(configureEvent{Status: "error", Error: err.Error()})
		return
	}
	emit(configureEvent{Status: "configuring", Message: "starting agent runtime"})
	h, cleanup, runStartup, err := buildRuntimeHandler(cfg, initialCreds)
	if err != nil {
		emit(configureEvent{Status: "error", Error: err.Error()})
		return
	}
	if !s.set(h, cleanup) {
		cleanup()
		emit(configureEvent{Status: "error", Error: "agent is already configured"})
		return
	}
	runStartup(func(message string) {
		emit(configureEvent{Status: "configuring", Message: message})
	})
	emit(configureEvent{Status: "ready", Message: "agent API is ready"})
}

func applyConfigureRequest(base *config.Config, req configureRequest) *config.Config {
	cfg := *base
	if req.AgentCwd != "" {
		cfg.AgentCwd = req.AgentCwd
	}
	if req.Model != "" {
		cfg.Model = req.Model
	}
	if req.WorkspaceOrigin != "" {
		cfg.WorkspaceOrigin = req.WorkspaceOrigin
	}
	if req.WorkspaceSource != "" {
		cfg.WorkspaceSource = req.WorkspaceSource
	}
	if req.WorkspaceSourceType != "" {
		cfg.WorkspaceType = req.WorkspaceSourceType
	}
	if req.WorkspaceCommit != "" {
		cfg.WorkspaceCommit = req.WorkspaceCommit
	}
	if req.WorkspaceTargetRef != "" {
		cfg.WorkspaceRef = req.WorkspaceTargetRef
	}
	if req.DataDir != "" {
		cfg.DataDir = req.DataDir
	}
	if req.ThreadsDir != "" {
		cfg.ThreadsDir = req.ThreadsDir
	}
	if req.HooksEnabled != nil {
		cfg.HooksEnabled = *req.HooksEnabled
	}
	if req.SessionID != "" {
		cfg.SessionID = req.SessionID
	}
	if req.MCPOAuthRedirectBase != "" {
		cfg.MCPOAuthRedirectBase = req.MCPOAuthRedirectBase
	}
	if req.DiscboeingServerURL != "" {
		cfg.DiscboeingServerURL = req.DiscboeingServerURL
	}
	if req.DiscboeingProjectID != "" {
		cfg.DiscboeingProjectID = req.DiscboeingProjectID
	}
	cfg.EnableGitControlSocket = req.EnableGitControlSocket
	return &cfg
}

func runBootstrap(cfg *config.Config) {
	state := &bootstrapState{}
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     "waiting_for_config",
			"configured": state.configured(),
		})
	})
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !state.configured() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":      "agent is waiting for configuration",
				"code":       agentNotConfiguredCode,
				"configured": false,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     "ok",
			"configured": true,
		})
	})

	authed := chi.NewRouter()
	authed.Use(middleware.Auth(cfg.SecretHash, cfg.TrustKey))
	authed.Post("/configure", func(w http.ResponseWriter, r *http.Request) {
		state.configure(cfg, w, r)
	})
	authed.Mount("/", state)
	r.Mount("/", authed)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
	}

	go func() {
		log.Printf("agent-api: listening on :%d (waiting for config)", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("agent-api: server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("agent-api: shutting down...")
	state.close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("agent-api: shutdown error: %v", err)
	}
	log.Println("agent-api: stopped")
}
