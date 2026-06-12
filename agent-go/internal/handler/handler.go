package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/browser"
	controlsocket "github.com/obot-platform/discobot/agent-go/internal/controlsocket"
	"github.com/obot-platform/discobot/agent-go/internal/hooks"
	"github.com/obot-platform/discobot/agent-go/internal/processes"
	"github.com/obot-platform/discobot/agent-go/internal/routes"
	"github.com/obot-platform/discobot/agent-go/internal/services"
	"github.com/obot-platform/discobot/agent-go/internal/sudoauth"
	"github.com/obot-platform/discobot/agent-go/promptqueue"
)

// Handler contains all HTTP handlers for the agent API.
type Handler struct {
	agentCwd       string
	conversations  *agent.ConversationManager
	threadManager  threadManager
	hookManager    *hooks.Manager          // nil if hooks are disabled
	serviceManager *services.Manager       // always initialized
	processManager *processes.Manager      // owns agent-side exec sessions
	defaultAgent   *agentimpl.DefaultAgent // for MCP manager access; may be nil
	browserManager *browser.Manager
	promptQueue    *promptqueue.Manager
	sudoAuthorizer sudoauth.Authorizer
	controlSocket  *controlsocket.Client
	chatPingEvery  time.Duration
	activity       *activityNotifier
	workspaceFiles *workspaceFileNotifier

	answeredMu        sync.Mutex
	answeredQuestions map[string]bool // toolCallID → true (tracks answered questions for status polling)
}

type threadManager interface {
	ListThreadInfos() ([]agent.ThreadInfo, error)
	GetThreadInfo(threadID string) (agent.ThreadInfo, error)
	GetThreadTokenUsageDetails(threadID string) (agent.ThreadTokenUsageDetails, error)
	CreateThread(ctx context.Context, req agent.CreateThreadRequest) (agent.ThreadInfo, error)
	UpdateThread(ctx context.Context, threadID string, req agent.UpdateThreadRequest) (agent.ThreadInfo, error)
	DeleteThread(ctx context.Context, threadID string) error
}

// New creates a new Handler.
func New(agentCwd string, conversations *agent.ConversationManager, hookManager *hooks.Manager, serviceManager *services.Manager, defaultAgent *agentimpl.DefaultAgent, options ...any) *Handler {
	var bm *browser.Manager
	var pq *promptqueue.Store
	var promptQueue *promptqueue.Manager
	var processManager *processes.Manager
	var controlSocket *controlsocket.Client
	for _, option := range options {
		switch value := option.(type) {
		case *browser.Manager:
			bm = value
		case *promptqueue.Store:
			pq = value
		case *promptqueue.Manager:
			promptQueue = value
		case *processes.Manager:
			processManager = value
		case *controlsocket.Client:
			controlSocket = value
		}
	}
	if processManager == nil {
		processManager = processes.NewManager(agentCwd)
	}
	h := &Handler{
		agentCwd:          agentCwd,
		conversations:     conversations,
		threadManager:     conversations,
		hookManager:       hookManager,
		serviceManager:    serviceManager,
		processManager:    processManager,
		defaultAgent:      defaultAgent,
		browserManager:    bm,
		controlSocket:     controlSocket,
		chatPingEvery:     defaultChatStreamPingInterval,
		activity:          newActivityNotifier(),
		workspaceFiles:    newWorkspaceFileNotifier(agentCwd),
		answeredQuestions: make(map[string]bool),
	}
	for _, option := range options {
		if value, ok := option.(sudoauth.Authorizer); ok && value != nil {
			h.sudoAuthorizer = value
		}
	}
	if defaultAgent != nil {
		h.threadManager = defaultAgent
	}
	if promptQueue == nil && pq != nil && conversations != nil {
		promptQueue = promptqueue.NewManager(pq, conversations, nil)
	}
	if promptQueue != nil {
		h.promptQueue = promptQueue
		h.promptQueue.SetChangeFunc(h.onPromptQueueChange)
	}
	if hookManager != nil && conversations != nil {
		hookManager.SetRepromptRunner(conversations, h.promptQueue)
	}
	if hookManager != nil && defaultAgent != nil {
		hookManager.SetAIHookAgent(defaultAgent)
		hookManager.SetThreadPhaseLookup(func(threadID string) string {
			info, err := defaultAgent.GetThreadInfo(threadID)
			if err != nil {
				return ""
			}
			return info.Phase
		})
	}
	if conversations != nil {
		conversations.AddCompletionListener(h)
	}

	return h
}

// OnTurnStart handles handler-owned turn startup bookkeeping.
func (h *Handler) OnTurnStart(threadID string) {
	if h.promptQueue != nil {
		h.promptQueue.ClearTimer(threadID)
	}
	h.notifyActivityChanged()
}

// OnTurnComplete handles handler-owned turn completion bookkeeping.
func (h *Handler) OnTurnComplete(threadID string, _ error) {
	if h.hookManager != nil {
		h.hookManager.OnTurnComplete(threadID)
	}
	if h.promptQueue != nil {
		go h.promptQueue.StartNext(threadID)
	}
	h.notifyActivityChanged()
}

func (h *Handler) onPromptQueueChange(string, []promptqueue.Prompt) {
	h.notifyActivityChanged()
}

func (h *Handler) notifyActivityChanged() {
	if h != nil && h.activity != nil {
		h.activity.Notify()
	}
}

// RegisterRoutes registers all API routes on the given router and records
// metadata in the global routes registry (used by GET /api/routes).
func (h *Handler) RegisterRoutes(r chi.Router) {
	reg := routes.GetRegistry()

	// Global routes (no auth required)
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/", Handler: h.Root,
		Meta: routes.Meta{Group: "Health", Description: "API root / health check"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/health", Handler: h.Health,
		Meta: routes.Meta{Group: "Health", Description: "Health check"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/user", Handler: h.User,
		Meta: routes.Meta{Group: "Health", Description: "Current user info"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/commands", Handler: h.ListCommands,
		Meta: routes.Meta{Group: "Commands", Description: "List available slash commands"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/ports", Handler: h.ListPorts,
		Meta: routes.Meta{Group: "Ports", Description: "List visible TCP listening ports"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/session/stream", Handler: h.StreamSession,
		Meta: routes.Meta{Group: "Session", Description: "Stream session-level resource events (SSE)"}})
	if h.controlSocket != nil {
		reg.Register(r, routes.Route{Method: "GET", Pattern: "/control/ws", Handler: h.ControlSocket,
			Meta: routes.Meta{Group: "Control", Description: "Server-initiated sandbox control WebSocket"}})
	}
	if h.browserManager != nil {
		reg.Register(r, routes.Route{Method: "GET", Pattern: "/sessions/{sessionId}/browser", Handler: h.GetBrowserSession,
			Meta: routes.Meta{Group: "Browser", Description: "Get session-scoped browser runtime info"}})
		reg.Register(r, routes.Route{Method: "GET", Pattern: "/sessions/{sessionId}/browser/cdp", Handler: h.ProxyBrowserCDP,
			Meta: routes.Meta{Group: "Browser", Description: "Proxy session-scoped browser CDP WebSocket"}})
	}

	// Thread routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/threads", Handler: h.ListThreads,
		Meta: routes.Meta{Group: "Threads", Description: "List all threads"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/threads/activity", Handler: h.GetSessionActivity,
		Meta: routes.Meta{Group: "Threads", Description: "Get session-level thread activity"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/threads", Handler: h.CreateThread,
		Meta: routes.Meta{
			Group:       "Threads",
			Description: "Create a thread",
			Body:        map[string]any{"id": "thread-1", "name": "Debug build failure"},
		}})
	r.Route("/threads/{id}", func(r chi.Router) {
		threadReg := reg.WithPrefix("/threads/{id}")

		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/", Handler: h.GetThread,
			Meta: routes.Meta{Group: "Threads", Description: "Get thread metadata"}})
		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/token-usage", Handler: h.GetThreadTokenUsage,
			Meta: routes.Meta{Group: "Threads", Description: "Get detailed thread token usage"}})
		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/messages", Handler: h.ListMessages,
			Meta: routes.Meta{
				Group:       "Threads",
				Description: "List thread messages",
				Params:      []routes.Param{{Name: "leafId", In: "query"}},
			}})
		threadReg.Register(r, routes.Route{Method: "PUT", Pattern: "/", Handler: h.UpdateThread,
			Meta: routes.Meta{
				Group:       "Threads",
				Description: "Replace thread metadata",
				Body:        map[string]any{"name": "Investigate failing CI"},
			}})
		threadReg.Register(r, routes.Route{Method: "PATCH", Pattern: "/", Handler: h.UpdateThread,
			Meta: routes.Meta{
				Group:       "Threads",
				Description: "Update thread metadata",
				Body:        map[string]any{"name": "Investigate failing CI"},
			}})
		threadReg.Register(r, routes.Route{Method: "DELETE", Pattern: "/", Handler: h.DeleteThread,
			Meta: routes.Meta{Group: "Threads", Description: "Delete a thread"}})
		threadReg.Register(r, routes.Route{Method: "DELETE", Pattern: "/queue/{queueId}", Handler: h.DeleteQueuedPrompt,
			Meta: routes.Meta{Group: "Threads", Description: "Delete a queued prompt"}})
		threadReg.Register(r, routes.Route{Method: "PATCH", Pattern: "/queue/{queueId}", Handler: h.UpdateQueuedPrompt,
			Meta: routes.Meta{
				Group:       "Threads",
				Description: "Update a queued prompt",
				Body:        map[string]any{"runAfter": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)},
			}})

		threadReg.Register(r, routes.Route{Method: "POST", Pattern: "/chat", Handler: h.PostChat,
			Meta: routes.Meta{
				Group:       "Chat",
				Description: "Start a completion turn",
				Body: map[string]any{
					"messages": []map[string]any{{
						"id":   "msg-1",
						"role": "user",
						"parts": []map[string]any{{
							"type":  "text",
							"text":  "Help me understand this repository.",
							"state": "done",
						}},
					}},
				},
			}})
		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/chat/status", Handler: h.ChatStatus,
			Meta: routes.Meta{Group: "Chat", Description: "Check whether a completion is active"}})
		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/chat/stream", Handler: h.ChatStream,
			Meta: routes.Meta{Group: "Chat", Description: "Stream completion events (SSE)"}})
		threadReg.Register(r, routes.Route{Method: "POST", Pattern: "/chat/cancel", Handler: h.CancelChat,
			Meta: routes.Meta{Group: "Chat", Description: "Cancel the active completion"}})
		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/chat/question", Handler: h.GetPendingQuestion,
			Meta: routes.Meta{Group: "Chat", Description: "Get current pending AskUserQuestion"}})
		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/artifacts/read", Handler: h.ReadThreadArtifact,
			Meta: routes.Meta{
				Group:       "Threads",
				Description: "Read a thread-local artifact by artifacts:// URI",
				Params:      []routes.Param{{Name: "uri", In: "query", Required: true}},
			}})
		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/chat/question/{questionId}", Handler: h.GetQuestion,
			Meta: routes.Meta{Group: "Chat", Description: "Get pending AskUserQuestion"}})
		threadReg.Register(r, routes.Route{Method: "POST", Pattern: "/chat/answer/{questionId}", Handler: h.PostAnswer,
			Meta: routes.Meta{
				Group:       "Chat",
				Description: "Submit answer to pending question",
				Body: map[string]any{
					"answers": map[string]string{
						"Which model should we use?": "Claude Sonnet",
					},
				},
			}})
	})

	// File system routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/files", Handler: h.ListFiles,
		Meta: routes.Meta{Group: "Files", Description: "List directory contents",
			Params: []routes.Param{{Name: "path", In: "query"}, {Name: "hidden", In: "query"}}}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/files/search", Handler: h.SearchFiles,
		Meta: routes.Meta{Group: "Files", Description: "Fuzzy-search files",
			Params: []routes.Param{{Name: "q", In: "query", Required: true}, {Name: "limit", In: "query"}}}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/files/read", Handler: h.ReadFile,
		Meta: routes.Meta{Group: "Files", Description: "Read file contents",
			Params: []routes.Param{{Name: "path", In: "query", Required: true}}}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/files/write", Handler: h.WriteFile,
		Meta: routes.Meta{
			Group:       "Files",
			Description: "Write file contents",
			Body:        map[string]any{"path": "notes/todo.txt", "content": "Ship it\n", "encoding": "utf8"},
		}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/files/delete", Handler: h.DeleteFile,
		Meta: routes.Meta{
			Group:       "Files",
			Description: "Delete a file or directory",
			Body:        map[string]any{"path": "notes/todo.txt"},
		}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/files/rename", Handler: h.RenameFile,
		Meta: routes.Meta{
			Group:       "Files",
			Description: "Rename or move a file",
			Body:        map[string]any{"oldPath": "notes/todo.txt", "newPath": "notes/archive/todo.txt"},
		}})

	// Git routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/diff", Handler: h.GetDiff,
		Meta: routes.Meta{Group: "Git", Description: "Get workspace diff",
			Params: []routes.Param{{Name: "path", In: "query"}, {Name: "format", In: "query"}}}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/commits", Handler: h.GetCommits,
		Meta: routes.Meta{Group: "Git", Description: "Get recent commit patches",
			Params: []routes.Param{{Name: "parent", In: "query"}}}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/workspace-change-commits", Handler: h.ListWorkspaceChangeCommits,
		Meta: routes.Meta{Group: "Git", Description: "List Discobot workspace change commits"}})

	// Hook routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/hooks/status", Handler: h.HooksStatus,
		Meta: routes.Meta{Group: "Hooks", Description: "Get hook evaluation status"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/hooks/state", Handler: h.HooksState,
		Meta: routes.Meta{Group: "Hooks", Description: "Get hook status and output logs"}})
	reg.Register(r, routes.Route{Method: "PATCH", Pattern: "/hooks/execution", Handler: h.UpdateHooksExecution,
		Meta: routes.Meta{Group: "Hooks", Description: "Toggle hook execution"}})
	reg.Register(r, routes.Route{Method: "PATCH", Pattern: "/hooks/{hookId}/execution", Handler: h.UpdateHookExecution,
		Meta: routes.Meta{Group: "Hooks", Description: "Toggle execution for one hook"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/hooks/{hookId}/output", Handler: h.HookOutput,
		Meta: routes.Meta{Group: "Hooks", Description: "Get hook output log"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/hooks/{hookId}/output/download", Handler: h.HookOutputDownload,
		Meta: routes.Meta{Group: "Hooks", Description: "Download hook output log"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/hooks/{hookId}/rerun", Handler: h.RerunHook,
		Meta: routes.Meta{Group: "Hooks", Description: "Manually rerun a hook"}})

	// Service routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/exec/capabilities", Handler: h.ExecCapabilities,
		Meta: routes.Meta{Group: "Exec", Description: "Get exec supervisor capabilities"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/sudo/authorize", Handler: h.AuthorizeSudo,
		Meta: routes.Meta{Group: "Exec", Description: "Authorize a sudo invocation from the sandbox wrapper"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/exec", Handler: h.CreateExec,
		Meta: routes.Meta{Group: "Exec", Description: "Create an exec session"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/exec", Handler: h.ListExec,
		Meta: routes.Meta{Group: "Exec", Description: "List exec sessions"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/exec/{id}", Handler: h.GetExec,
		Meta: routes.Meta{Group: "Exec", Description: "Get exec session metadata"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/exec/{id}/output", Handler: h.ExecOutput,
		Meta: routes.Meta{Group: "Exec", Description: "Get persisted exec output events"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/exec/{id}/events", Handler: h.ExecEvents,
		Meta: routes.Meta{Group: "Exec", Description: "Get or follow exec events"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/exec/{id}/resize", Handler: h.ResizeExec,
		Meta: routes.Meta{Group: "Exec", Description: "Resize an exec TTY"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/exec/{id}/close-write", Handler: h.CloseWriteExec,
		Meta: routes.Meta{Group: "Exec", Description: "Close exec stdin"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/exec/{id}/kill", Handler: h.KillExec,
		Meta: routes.Meta{Group: "Exec", Description: "Kill an exec session"}})
	reg.Register(r, routes.Route{Method: "DELETE", Pattern: "/exec/{id}", Handler: h.DeleteExec,
		Meta: routes.Meta{Group: "Exec", Description: "Kill an exec session"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/exec/{id}/attach", Handler: h.AttachExec,
		Meta: routes.Meta{Group: "Exec", Description: "Attach to an exec session over WebSocket"}})

	reg.Register(r, routes.Route{Method: "GET", Pattern: "/services", Handler: h.ListServices,
		Meta: routes.Meta{Group: "Services", Description: "List all services with status"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/services/{serviceId}/start", Handler: h.StartService,
		Meta: routes.Meta{Group: "Services", Description: "Start a service"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/services/{serviceId}/stop", Handler: h.StopService,
		Meta: routes.Meta{Group: "Services", Description: "Stop a running service"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/services/{serviceId}/output", Handler: h.ServiceOutput,
		Meta: routes.Meta{Group: "Services", Description: "Stream service output (SSE)"}})
	// ServiceProxy is registered directly (HandleFunc, not method-specific)
	r.HandleFunc("/services/{serviceId}/http", h.ServiceProxy)
	r.HandleFunc("/services/{serviceId}/http/*", h.ServiceProxy)

	// MCP server routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/mcp/servers", Handler: h.ListMCPServers,
		Meta: routes.Meta{Group: "MCP", Description: "List MCP server connection status"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/mcp/servers/{name}/oauth", Handler: h.GetMCPServerOAuth,
		Meta: routes.Meta{Group: "MCP", Description: "Get OAuth authorization URL for MCP server"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/mcp/servers/{name}/oauth/code", Handler: h.PostMCPServerOAuthCode,
		Meta: routes.Meta{
			Group:       "MCP",
			Description: "Submit OAuth authorization code for MCP server",
			Body:        map[string]any{"code": "abc123", "state": "xyz789"},
		}})
}

// JSON writes a JSON response with the given status code.
func (h *Handler) JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// Error writes a JSON error response.
func (h *Handler) Error(w http.ResponseWriter, status int, message string) {
	h.JSON(w, status, map[string]string{"error": message})
}

// DecodeJSON decodes the request body as JSON.
func (h *Handler) DecodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
