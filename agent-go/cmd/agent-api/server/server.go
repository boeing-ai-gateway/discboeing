// Package server implements the HTTP API server mode for agent-api.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/browser"
	"github.com/obot-platform/discobot/agent-go/internal/config"
	controlfeatures "github.com/obot-platform/discobot/agent-go/internal/controlfeatures"
	controlsocket "github.com/obot-platform/discobot/agent-go/internal/controlsocket"
	"github.com/obot-platform/discobot/agent-go/internal/credentials"
	"github.com/obot-platform/discobot/agent-go/internal/handler"
	"github.com/obot-platform/discobot/agent-go/internal/hooks"
	"github.com/obot-platform/discobot/agent-go/internal/middleware"
	"github.com/obot-platform/discobot/agent-go/internal/processes"
	"github.com/obot-platform/discobot/agent-go/internal/routes"
	"github.com/obot-platform/discobot/agent-go/internal/services"
	"github.com/obot-platform/discobot/agent-go/internal/workspaceenv"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/promptqueue"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
	"github.com/obot-platform/discobot/agent-go/tools"

	// Embed the API browser HTML.
	staticFiles "github.com/obot-platform/discobot/agent-go/cmd/agent-api/static"
)

func visibleEnvSnapshot(workspaceRoot string, envSnapshot func() map[string]string) map[string]string {
	env := workspaceenv.FileSnapshot(workspaceRoot)
	if env == nil {
		env = map[string]string{}
	}
	if envSnapshot == nil {
		return env
	}
	maps.Copy(env, envSnapshot())
	return env
}

type credentialUseAuthorizerResolver struct {
	registry *providers.ProviderRegistry
}

func (r credentialUseAuthorizerResolver) ResolveAuthorizationModel(currentProviderID string) (credentials.AuthorizationModelRef, error) {
	ref, err := r.registry.ResolveModelInProvider(currentProviderID, "", providers.ModelTaskAuthorization, providers.ModelTaskChat)
	if err != nil {
		return credentials.AuthorizationModelRef{}, err
	}
	return credentials.AuthorizationModelRef{ProviderID: ref.ProviderID, ModelID: ref.ModelID}, nil
}

func (r credentialUseAuthorizerResolver) CompleteText(ctx context.Context, model credentials.AuthorizationModelRef, messages []message.Message, maxTokens *int) (string, error) {
	provider, err := r.registry.Get(model.ProviderID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for chunk, err := range provider.Complete(ctx, providers.CompleteRequest{
		Model:     providers.ModelRef{ProviderID: model.ProviderID, ModelID: model.ModelID},
		Messages:  messages,
		MaxTokens: maxTokens,
	}) {
		if err != nil {
			return "", err
		}
		switch delta := chunk.(type) {
		case message.TextDeltaChunk:
			b.WriteString(delta.Delta)
		}
	}
	return strings.TrimSpace(b.String()), nil
}

// Run starts the HTTP API server and blocks until SIGINT/SIGTERM.
func Run(cfg *config.Config) {
	// ── Credential manager ───────────────────────────────────────────────────
	credMgr := credentials.NewManager()

	// ── Provider registry ────────────────────────────────────────────────────
	// Providers are built on demand from current credentials when first needed.
	// No startup registration required — credentials arrive per-request via
	// X-Discobot-Credentials and are held in the credential manager.
	reg := providers.NewProviderRegistry(credMgr)

	// ── Thread store ─────────────────────────────────────────────────────────
	store := thread.NewStore(cfg.ThreadsDir)

	// ── Tools executor ───────────────────────────────────────────────────────
	// threadID is empty here; the executor's threadID is only used for naming
	// output spill files and is set per-turn by tools that need it.
	exec := tools.New(cfg.AgentCwd, cfg.DataDir, "")
	exec.SetThreadsDir(cfg.ThreadsDir)
	// Internal tool lookups may use request-scoped credentials even when they are
	// not agent-visible. Bash still receives only the visible snapshot below.
	exec.SetEnvLookup(func(key string) string {
		if cred := credMgr.Get(key); cred != nil {
			return cred.Value
		}
		return ""
	})
	browserMgr, err := browser.NewManager(cfg.SessionID, cfg.DataDir, cfg.Port)
	if err != nil {
		log.Fatalf("browser manager: %v", err)
	}
	browserMgr.SetStore(browser.NewStore(cfg.ThreadsDir))
	browserMgr.SetCurrentTurnLoader(store.LoadTurnState)
	exec.SetEnvForThread(browserMgr.EnvForThread)
	exec.SetEnvSnapshot(func() map[string]string {
		env := visibleEnvSnapshot(cfg.AgentCwd, credMgr.Snapshot)
		maps.Copy(env, browserMgr.Env())
		return env
	})
	authorizer := credentials.NewCredentialUseAuthorizer(
		credentialUseAuthorizerResolver{registry: reg},
		credMgr,
		sessionconfig.CredentialUseAuthorizerSystemPrompt(),
	)
	exec.SetCredentialUseAuthorizer(func(ctx context.Context, currentProviderID, toolCallID, command, description string, uses []tools.CredentialUseBinding) error {
		converted := make([]credentials.CredentialUseBinding, 0, len(uses))
		for _, use := range uses {
			converted = append(converted, credentials.CredentialUseBinding{
				CredentialID: use.CredentialID,
				UseID:        use.UseID,
				EnvVar:       use.EnvVar,
			})
		}
		return authorizer.Authorize(ctx, currentProviderID, toolCallID, command, description, converted)
	})
	exec.SetCredentialUseEnv(func(uses []tools.CredentialUseBinding) (map[string]string, error) {
		env := make(map[string]string, len(uses))
		for _, use := range uses {
			cred := credMgr.SessionCredential(use.CredentialID)
			if cred == nil {
				return nil, fmt.Errorf("credential id %s is not available in this session", use.CredentialID)
			}
			if !cred.AgentVisible {
				return nil, fmt.Errorf("credential id %s is not visible to the agent in this session", use.CredentialID)
			}
			if cred.EnvVar != use.EnvVar {
				return nil, fmt.Errorf("credential id %s is not authorized for environment variable %s", use.CredentialID, use.EnvVar)
			}
			if existing, ok := env[use.EnvVar]; ok && existing != cred.Value {
				return nil, fmt.Errorf("multiple credential uses target environment variable %s", use.EnvVar)
			}
			env[use.EnvVar] = cred.Value
		}
		return env, nil
	})

	// ── DefaultAgent ─────────────────────────────────────────────────────────
	mcpCfg := agentimpl.NewMCPConfig(
		cfg.MCPOAuthRedirectBase,
		cfg.SessionID,
		cfg.DiscobotServerURL,
		cfg.DiscobotProjectID,
	)
	a := agentimpl.NewDefaultAgent(store, reg, exec, cfg.AgentCwd, mcpCfg)

	// ── ConversationManager ────────────────────────────────────────────────────
	conversations := agent.NewConversationManager(a)
	processMgr := processes.NewManager(cfg.AgentCwd)

	queueStore := promptqueue.NewStore(cfg.ThreadsDir)
	promptQueue := promptqueue.NewManager(queueStore, conversations, nil)
	promptQueue.SetChangeFunc(func(threadID string, queue []promptqueue.Prompt) {
		info, err := a.GetThreadInfo(threadID)
		if err != nil {
			return
		}
		chunk := threadUpdateChunk(conversations, info, queue)
		emitThreadUpdate(conversations, threadID, chunk)
	})

	// ── Hook manager ─────────────────────────────────────────────────────────
	var hookMgr *hooks.Manager
	if cfg.HooksEnabled {
		hookMgr = hooks.NewManager(cfg.AgentCwd, cfg.SessionID, processMgr)
		if err := hookMgr.Init(); err != nil {
			log.Printf("warn: hooks init: %v", err)
		}
		hookMgr.SetEnvSnapshot(func() map[string]string {
			return credMgr.HooksSnapshot()
		})
		hookMgr.SetChunkEmitter(func(chunk message.MessageChunk) {
			conversations.EmitEphemeralChunk(chunk)
		})
		hookMgr.SetRepromptRunner(conversations, promptQueue)
		// The HTTP handler owns post-completion hook evaluation so hook-failure
		// re-prompts preserve structured metadata for optimized UI rendering.
	}

	// ── Service manager ──────────────────────────────────────────────────────
	svcMgr := services.NewManager(cfg.AgentCwd, processMgr)
	svcMgr.SetEnvSnapshot(func() map[string]string {
		return credMgr.ServicesSnapshot()
	})

	// ── Control socket ────────────────────────────────────────────────────────
	controlCtx, cancelControl := context.WithCancel(context.Background())
	var controlSocket *controlsocket.Client
	if os.Getenv("DISCOBOT_ENABLE_GIT_CONTROL_SOCKET") == "true" {
		controlSocket = controlsocket.New()
		gitTunnel := controlfeatures.NewGitTunnel(controlSocket)
		if gitRemoteURL, err := gitTunnel.StartEndpoint(controlCtx); err != nil {
			log.Printf("warn: git control endpoint: %v", err)
		} else {
			controlfeatures.ConfigureGitRemote(controlCtx, cfg.AgentCwd, gitRemoteURL, cfg.SessionID)
		}
	}

	// ── HTTP handler ─────────────────────────────────────────────────────────
	sudoAuthorizer := credentials.NewSudoAuthorizer(authorizer, credMgr)
	h := handler.New(cfg.AgentCwd, conversations, hookMgr, svcMgr, a, promptQueue, browserMgr, processMgr, sudoAuthorizer, controlSocket)

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Standard middleware (no global timeout — SSE and long AI calls need
	// unbounded connection lifetimes).
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	// Browser CDP: loopback-only websocket route authenticated by the session
	// token in the query string rather than the normal Bearer middleware.
	if browserMgr != nil {
		r.Get("/sessions/{sessionId}/browser/cdp", h.ProxyBrowserCDP)
	}

	// All remaining routes stay behind the normal auth and credentials stack.
	authed := chi.NewRouter()

	// Auth: validates Bearer token against DISCOBOT_SECRET hash.
	authed.Use(middleware.Auth(cfg.SecretHash))

	// Credentials: applies X-Discobot-Credentials env vars and git user config.
	authed.Use(middleware.Credentials(credMgr, promptQueue.EnableTimers))

	// Register all agent API routes (also populates the global routes registry).
	h.RegisterRoutes(authed)
	r.Mount("/", authed)

	// ── Meta routes ──────────────────────────────────────────────────────────

	// GET /api/ui — embedded API browser HTML (same UI as the main server).
	r.Get("/api/ui", func(w http.ResponseWriter, _ *http.Request) {
		content, err := staticFiles.Files.ReadFile("api-ui.html")
		if err != nil {
			http.Error(w, "API UI not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	})

	// GET /api/routes — returns route metadata for the API browser.
	r.Get("/api/routes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(routes.All())
	})

	// ── HTTP server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
		// No read/write/idle timeouts — SSE and long-running AI calls need
		// connections that can stay open for minutes or longer.
	}

	go func() {
		log.Printf("agent-api: listening on :%d (cwd: %s)", cfg.Port, cfg.AgentCwd)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("agent-api: server error: %v", err)
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("agent-api: shutting down...")
	cancelControl()

	// Close MCP server connections.
	a.Close()
	if err := browserMgr.Close(); err != nil {
		log.Printf("browser shutdown: %v", err)
	}

	// Give in-flight requests up to 5 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("agent-api: shutdown error: %v", err)
	}

	log.Println("agent-api: stopped")
}

func emitThreadUpdate(conversations *agent.ConversationManager, threadID string, chunk message.ThreadUpdateChunk) {
	if !conversations.EmitChunkIfActive(threadID, chunk) {
		conversations.EmitEphemeralChunk(chunk)
	}
}

func threadUpdateChunk(conversations *agent.ConversationManager, info agent.ThreadInfo, queue []promptqueue.Prompt) message.ThreadUpdateChunk {
	applyThreadStateOverlay(conversations, &info)
	chunk := message.ThreadUpdateChunk{
		Data: message.ThreadUpdateData{
			Thread: message.ThreadUpdateInfo{
				ID:            info.ID,
				Name:          info.Name,
				CWD:           info.CWD,
				LastMessage:   info.LastMessage,
				ErrorMessage:  info.ErrorMessage,
				Model:         info.Model,
				Reasoning:     info.Reasoning,
				ServiceTier:   info.ServiceTier,
				State:         string(info.State),
				ActiveCommand: info.ActiveCommand,
				Metadata:      info.Metadata,
			},
		},
	}
	chunk.Data.Thread.PromptQueue = promptqueue.ToThreadUpdateInfo(queue)
	return chunk
}

func applyThreadStateOverlay(conversations *agent.ConversationManager, info *agent.ThreadInfo) {
	if info == nil || conversations == nil || strings.TrimSpace(info.ID) == "" {
		return
	}
	if conversations.ActiveCompletionID(info.ID) != "" {
		return
	}
	if interrupted, err := conversations.HasInterruptedTurn(info.ID); err == nil && interrupted {
		info.State = agent.ThreadStateInterrupted
	}
}
