package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/browser"
	"github.com/obot-platform/discobot/agent-go/internal/hooks"
	"github.com/obot-platform/discobot/agent-go/internal/routes"
	"github.com/obot-platform/discobot/agent-go/internal/services"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

const maxHookRetries = 3

// Handler contains all HTTP handlers for the agent API.
type Handler struct {
	agentCwd       string
	completions    *agent.CompletionManager
	hookManager    *hooks.Manager          // nil if hooks are disabled
	serviceManager *services.Manager       // always initialized
	defaultAgent   *agentimpl.DefaultAgent // for MCP manager access; may be nil
	browserManager *browser.Manager
	chatPingEvery  time.Duration

	hookMu             sync.Mutex
	hookRetryCount     map[string]int
	hookNotificationTo map[string]string

	answeredMu        sync.Mutex
	answeredQuestions map[string]bool // toolCallID → true (tracks answered questions for status polling)
	queueMu           sync.Mutex
	queueTimers       map[string]*time.Timer
	queueTimersReady  bool
}

// New creates a new Handler.
func New(agentCwd string, completions *agent.CompletionManager, hookManager *hooks.Manager, serviceManager *services.Manager, defaultAgent *agentimpl.DefaultAgent, browserManager ...*browser.Manager) *Handler {
	var bm *browser.Manager
	if len(browserManager) > 0 {
		bm = browserManager[0]
	}
	h := &Handler{
		agentCwd:           agentCwd,
		completions:        completions,
		hookManager:        hookManager,
		serviceManager:     serviceManager,
		defaultAgent:       defaultAgent,
		browserManager:     bm,
		chatPingEvery:      defaultChatStreamPingInterval,
		hookRetryCount:     make(map[string]int),
		hookNotificationTo: make(map[string]string),
		answeredQuestions:  make(map[string]bool),
		queueTimers:        make(map[string]*time.Timer),
	}

	if hookManager != nil {
		hookManager.SetChunkEmitter(func(chunk message.MessageChunk) {
			completions.EmitEphemeralChunk("hooks-status", chunk)
		})
	}
	completions.AddCompletionListener(h)

	return h
}

// OnTurnStart clears thread-local error state when a turn begins.
func (h *Handler) OnTurnStart(threadID string) {
	h.clearQueuedPromptTimer(threadID)
	if cfg, cleared := h.clearThreadError(threadID); cleared {
		go h.completions.EmitChunkIfActive(threadID, thread.UpdateChunkFromConfig(threadID, cfg))
	}
}

// OnTurnComplete is called when a completion finishes. It schedules hook evaluation.
func (h *Handler) OnTurnComplete(threadID string, err error) {
	h.persistThreadError(threadID, err)
	if h.hookManager != nil && h.hookManager.HasFileHooks() {
		// Fire-and-forget goroutine matching the TS scheduleHookEvaluation pattern.
		go h.scheduleHookEvaluation(threadID)
	}
	go h.startNextQueuedPrompt(threadID)
}

func (h *Handler) persistThreadError(threadID string, err error) {
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return
	}
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}

	store := h.defaultAgent.Store()
	cfg, loadErr := store.LoadConfig(threadID)
	if loadErr != nil {
		log.Printf("thread error: failed to load config for %s: %v", threadID, loadErr)
		return
	}
	cfg.ErrorMessage = strings.TrimSpace(err.Error())
	if cfg.ErrorMessage == "" {
		return
	}
	if saveErr := store.SaveConfig(threadID, cfg); saveErr != nil {
		log.Printf("thread error: failed to save config for %s: %v", threadID, saveErr)
	}
}

func (h *Handler) clearThreadError(threadID string) (thread.Config, bool) {
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return thread.Config{}, false
	}

	store := h.defaultAgent.Store()
	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		log.Printf("thread error: failed to load config for %s: %v", threadID, err)
		return thread.Config{}, false
	}
	if strings.TrimSpace(cfg.ErrorMessage) == "" {
		return thread.Config{}, false
	}
	cfg.ErrorMessage = ""
	if err := store.SaveConfig(threadID, cfg); err != nil {
		log.Printf("thread error: failed to clear config for %s: %v", threadID, err)
		return thread.Config{}, false
	}
	return cfg, true
}

func cloneUIParts(parts []message.UIPart) []message.UIPart {
	if len(parts) == 0 {
		return nil
	}
	return append([]message.UIPart{}, parts...)
}

func queuedPromptFromRequest(userMessage message.UIMessage, model, reasoning, mode string, runAfter time.Time) thread.QueuedPrompt {
	queued := thread.QueuedPrompt{
		Message: message.UIMessage{
			ID:       userMessage.ID,
			Role:     userMessage.Role,
			Parts:    cloneUIParts(userMessage.Parts),
			Metadata: userMessage.Metadata,
		},
		Model:     model,
		Reasoning: reasoning,
		Mode:      mode,
	}
	if !runAfter.IsZero() {
		queued.RunAfter = runAfter.UTC()
	}
	return queued
}

func (h *Handler) enqueuePrompt(threadID string, queued thread.QueuedPrompt) (thread.Config, thread.QueuedPrompt, error) {
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return thread.Config{}, thread.QueuedPrompt{}, errors.New("prompt queue unavailable")
	}

	h.queueMu.Lock()
	defer h.queueMu.Unlock()

	cfg, saved, err := h.defaultAgent.Store().AppendQueuedPrompt(threadID, queued)
	if err != nil {
		return thread.Config{}, thread.QueuedPrompt{}, err
	}
	h.rescheduleQueuedPromptTimerLocked(threadID)
	return cfg, saved, nil
}

func (h *Handler) startNextQueuedPrompt(threadID string) {
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return
	}

	store := h.defaultAgent.Store()
	h.queueMu.Lock()
	h.stopQueuedPromptTimerLocked(threadID)
	if h.completions.ActiveCompletionID(threadID) != "" {
		h.rescheduleQueuedPromptTimerLocked(threadID)
		h.queueMu.Unlock()
		return
	}

	cfg, queuedPrompt, err := store.PopQueuedPrompt(threadID)
	if err != nil {
		log.Printf("queue: failed to pop queued prompt for %s: %v", threadID, err)
		h.rescheduleQueuedPromptTimerLocked(threadID)
		h.queueMu.Unlock()
		return
	}
	if queuedPrompt == nil {
		h.rescheduleQueuedPromptTimerLocked(threadID)
		h.queueMu.Unlock()
		return
	}
	h.queueMu.Unlock()

	leafID := strings.TrimSpace(cfg.ActiveLeafID)
	if leafID == "" {
		leafID, err = store.FindLeaf(threadID)
		if err != nil {
			log.Printf("queue: failed to resolve leaf for %s: %v", threadID, err)
			h.queueMu.Lock()
			if _, restoreErr := store.PrependQueuedPrompt(threadID, *queuedPrompt); restoreErr != nil {
				log.Printf("queue: failed to restore queued prompt for %s: %v", threadID, restoreErr)
			}
			h.rescheduleQueuedPromptTimerLocked(threadID)
			h.queueMu.Unlock()
			return
		}
	}

	req := agent.PromptRequest{
		LeafID:    leafID,
		UserParts: append([]message.UIPart{}, queuedPrompt.Message.Parts...),
		Metadata:  queuedPrompt.Message.Metadata,
		Model:     queuedPrompt.Model,
		Reasoning: queuedPrompt.Reasoning,
		Mode:      queuedPrompt.Mode,
	}
	if _, err := h.startPromptRequest(threadID, req); err != nil {
		h.queueMu.Lock()
		if _, restoreErr := store.PrependQueuedPrompt(threadID, *queuedPrompt); restoreErr != nil {
			log.Printf("queue: failed to restore queued prompt for %s: %v", threadID, restoreErr)
		}
		log.Printf("queue: failed to start queued prompt for %s: %v", threadID, err)
		h.rescheduleQueuedPromptTimerLocked(threadID)
		h.queueMu.Unlock()
		return
	}

	h.queueMu.Lock()
	h.rescheduleQueuedPromptTimerLocked(threadID)
	h.queueMu.Unlock()
	h.completions.EmitChunkIfActive(threadID, thread.UpdateChunkFromConfig(threadID, cfg))
}

func hookFailurePromptRequest(result hooks.FileHookEvalResult) agent.PromptRequest {
	return agent.PromptRequest{
		Metadata: func() json.RawMessage {
			if result.HookFailure == nil {
				return nil
			}
			data, err := json.Marshal(map[string]any{
				"discobot": result.HookFailure,
			})
			if err != nil {
				return nil
			}
			return data
		}(),
		UserParts: []message.UIPart{
			message.UITextPart{Text: result.LLMMessage},
		},
	}
}

func (h *Handler) startPromptRequest(threadID string, req agent.PromptRequest) (string, error) {
	interrupted, err := h.completions.HasInterruptedTurn(threadID)
	if err != nil {
		return "", err
	}
	if interrupted {
		return h.completions.Resume(threadID, req)
	}
	return h.completions.Chat(threadID, req)
}

// startHookFailureReprompt sends a hook-failure follow-up message to the LLM.
func (h *Handler) startHookFailureReprompt(threadID string, result hooks.FileHookEvalResult) error {
	req := hookFailurePromptRequest(result)
	_, err := h.startPromptRequest(threadID, req)
	return err
}

func (h *Handler) enqueueHookFailureReprompt(threadID string, result hooks.FileHookEvalResult) error {
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return nil
	}

	h.queueMu.Lock()
	defer h.queueMu.Unlock()

	store := h.defaultAgent.Store()
	cfg, err := store.PrependQueuedPrompt(threadID, thread.QueuedPrompt{
		Message: message.UIMessage{
			Role:     "user",
			Parts:    []message.UIPart{message.UITextPart{Text: result.LLMMessage}},
			Metadata: hookFailurePromptRequest(result).Metadata,
		},
	})
	if err != nil {
		return err
	}
	h.rescheduleQueuedPromptTimerLocked(threadID)
	h.completions.EmitChunkIfActive(threadID, thread.UpdateChunkFromConfig(threadID, cfg))
	return nil
}

func (h *Handler) resumeQueuedPromptTimers() {
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return
	}
	threadIDs, err := h.defaultAgent.Store().ListThreads()
	if err != nil {
		log.Printf("queue: failed to list threads for timer resume: %v", err)
		return
	}
	for _, threadID := range threadIDs {
		h.rescheduleQueuedPromptTimer(threadID)
	}
}

func (h *Handler) clearQueuedPromptTimer(threadID string) {
	h.queueMu.Lock()
	defer h.queueMu.Unlock()
	h.stopQueuedPromptTimerLocked(threadID)
}

func (h *Handler) EnableQueuedPromptTimers() {
	h.queueMu.Lock()
	if h.queueTimersReady {
		h.queueMu.Unlock()
		return
	}
	h.queueTimersReady = true
	h.queueMu.Unlock()
	go h.resumeQueuedPromptTimers()
}

func (h *Handler) rescheduleQueuedPromptTimer(threadID string) {
	h.queueMu.Lock()
	defer h.queueMu.Unlock()
	h.rescheduleQueuedPromptTimerLocked(threadID)
}

func (h *Handler) stopQueuedPromptTimerLocked(threadID string) {
	timer := h.queueTimers[threadID]
	if timer == nil {
		return
	}
	timer.Stop()
	delete(h.queueTimers, threadID)
}

func (h *Handler) rescheduleQueuedPromptTimerLocked(threadID string) {
	if !h.queueTimersReady {
		return
	}
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		return
	}

	h.stopQueuedPromptTimerLocked(threadID)
	if h.completions.ActiveCompletionID(threadID) != "" {
		return
	}

	cfg, err := h.defaultAgent.Store().LoadConfig(threadID)
	if err != nil {
		log.Printf("queue: failed to load config for timer reschedule on %s: %v", threadID, err)
		return
	}

	now := time.Now().UTC()
	var nextRunAfter *time.Time
	for _, queued := range cfg.PromptQueue {
		if queued.RunAfter.IsZero() || !queued.RunAfter.After(now) {
			go h.startNextQueuedPrompt(threadID)
			return
		}
		if nextRunAfter == nil || queued.RunAfter.Before(*nextRunAfter) {
			runAfter := queued.RunAfter
			nextRunAfter = &runAfter
		}
	}
	if nextRunAfter == nil {
		return
	}

	delay := max(time.Until(*nextRunAfter), 0)
	h.queueTimers[threadID] = time.AfterFunc(delay, func() {
		h.startNextQueuedPrompt(threadID)
	})
}

// scheduleHookEvaluation runs hook evaluation after a grace period, and
// triggers a re-prompt if a hook fails with notifyLlm=true.
func (h *Handler) scheduleHookEvaluation(threadID string) {
	// 200ms grace period to let SSE flush
	time.Sleep(200 * time.Millisecond)

	result := h.hookManager.EvaluateFileHooks()
	h.reconcileHookNotificationState()
	if !result.ShouldReprompt {
		return
	}

	hookID := ""
	if result.FailedResult != nil {
		hookID = strings.TrimSpace(result.FailedResult.Hook.ID)
	}
	if hookID == "" {
		hookID = threadID
	}

	count, shouldNotify := h.claimHookNotificationThread(hookID, threadID)
	if !shouldNotify {
		return
	}
	if count >= maxHookRetries {
		log.Printf("hooks: max retries (%d) reached for hook %q, not re-prompting", maxHookRetries, hookID)
		return
	}

	if err := h.startHookFailureReprompt(threadID, result); err != nil {
		if strings.Contains(err.Error(), "completion_in_progress") {
			if queueErr := h.enqueueHookFailureReprompt(threadID, result); queueErr != nil {
				log.Printf("hooks: failed to queue re-prompt after conflict: %v", queueErr)
			}
		}
		log.Printf("hooks: failed to start re-prompt: %v", err)
	}
}

func (h *Handler) reconcileHookNotificationState() {
	if h.hookManager == nil {
		return
	}

	status := h.hookManager.GetStatus()
	pending := make(map[string]struct{}, len(status.PendingHooks))
	for _, hookID := range status.PendingHooks {
		pending[hookID] = struct{}{}
	}

	h.hookMu.Lock()
	defer h.hookMu.Unlock()
	for hookID := range h.hookNotificationTo {
		if _, ok := pending[hookID]; !ok {
			delete(h.hookNotificationTo, hookID)
			delete(h.hookRetryCount, hookID)
		}
	}
	for hookID := range h.hookRetryCount {
		if _, ok := pending[hookID]; !ok {
			delete(h.hookRetryCount, hookID)
		}
	}
}

func (h *Handler) claimHookNotificationThread(hookID, threadID string) (int, bool) {
	h.hookMu.Lock()
	defer h.hookMu.Unlock()

	owner := h.hookNotificationTo[hookID]
	if owner == "" {
		h.hookNotificationTo[hookID] = threadID
		owner = threadID
	}
	if owner != threadID {
		return 0, false
	}

	h.hookRetryCount[hookID]++
	return h.hookRetryCount[hookID], true
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
	if h.browserManager != nil {
		reg.Register(r, routes.Route{Method: "GET", Pattern: "/sessions/{sessionId}/browser", Handler: h.GetBrowserSession,
			Meta: routes.Meta{Group: "Browser", Description: "Get session-scoped browser runtime info"}})
	}

	// Thread routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/threads", Handler: h.ListThreads,
		Meta: routes.Meta{Group: "Threads", Description: "List all threads"}})
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

		threadReg.Register(r, routes.Route{Method: "GET", Pattern: "/models", Handler: h.ListModels,
			Meta: routes.Meta{Group: "Threads", Description: "List available models"}})

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

	// Hook routes
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/hooks/status", Handler: h.HooksStatus,
		Meta: routes.Meta{Group: "Hooks", Description: "Get hook evaluation status"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/hooks/{hookId}/output", Handler: h.HookOutput,
		Meta: routes.Meta{Group: "Hooks", Description: "Get hook output log"}})
	reg.Register(r, routes.Route{Method: "GET", Pattern: "/hooks/{hookId}/output/download", Handler: h.HookOutputDownload,
		Meta: routes.Meta{Group: "Hooks", Description: "Download hook output log"}})
	reg.Register(r, routes.Route{Method: "POST", Pattern: "/hooks/{hookId}/rerun", Handler: h.RerunHook,
		Meta: routes.Meta{Group: "Hooks", Description: "Manually rerun a hook"}})

	// Service routes
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
