package handler

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func (h *Handler) requireThreadStore(w http.ResponseWriter) bool {
	if h.defaultAgent == nil || h.defaultAgent.Store() == nil {
		h.Error(w, http.StatusNotImplemented, "thread store unavailable")
		return false
	}
	return true
}

func (h *Handler) threadResponse(threadID string, cfg thread.Config) api.Thread {
	name := strings.TrimSpace(cfg.Name)

	mode := "build"
	if strings.EqualFold(strings.TrimSpace(cfg.Mode.Value), "plan") {
		mode = "plan"
	}

	pendingQuestion := false
	if state, err := h.defaultAgent.Store().LoadTurnState(threadID); err == nil && state != nil {
		pendingQuestion = state.Phase == thread.PhaseWaitingForAnswer
	}

	state := string(cfg.LastTurnState)
	if h.completions.ActiveCompletionID(threadID) == "" {
		if interrupted, err := h.completions.HasInterruptedTurn(threadID); err == nil && interrupted {
			state = string(thread.StateInterrupted)
		}
	}

	return api.Thread{
		ID:              threadID,
		Name:            name,
		CWD:             strings.TrimSpace(cfg.CWD),
		LastMessage:     strings.TrimSpace(cfg.LastMessage),
		ErrorMessage:    strings.TrimSpace(cfg.ErrorMessage),
		Model:           cfg.Model,
		Reasoning:       string(cfg.Reasoning),
		Mode:            mode,
		State:           state,
		PendingQuestion: pendingQuestion,
		ActiveCommand:   strings.TrimSpace(cfg.ActiveCommand),
		PromptQueue:     queuedPromptResponse(cfg.PromptQueue),
		Metadata:        cfg.Metadata,
	}
}

func queuedPromptResponse(queue []thread.QueuedPrompt) []api.QueuedPrompt {
	if len(queue) == 0 {
		return nil
	}
	items := make([]api.QueuedPrompt, 0, len(queue))
	for _, prompt := range queue {
		runAfter := ""
		if !prompt.RunAfter.IsZero() {
			runAfter = prompt.RunAfter.UTC().Format(time.RFC3339Nano)
		}
		items = append(items, api.QueuedPrompt{
			ID:        prompt.ID,
			CreatedAt: prompt.CreatedAt.UTC().Format(time.RFC3339Nano),
			RunAfter:  runAfter,
			Message:   prompt.Message,
			Model:     prompt.Model,
			Reasoning: prompt.Reasoning,
			Mode:      prompt.Mode,
		})
	}
	return items
}

// ListThreads handles GET /threads — lists all threads.
func (h *Handler) ListThreads(w http.ResponseWriter, _ *http.Request) {
	if !h.requireThreadStore(w) {
		return
	}

	threadIDs, err := h.completions.ListThreads()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if threadIDs == nil {
		threadIDs = []string{}
	}
	sort.Strings(threadIDs)

	threads := make([]api.Thread, 0, len(threadIDs))
	store := h.defaultAgent.Store()
	for _, threadID := range threadIDs {
		cfg, _ := store.LoadConfig(threadID)
		threads = append(threads, h.threadResponse(threadID, cfg))
	}

	h.JSON(w, http.StatusOK, api.ListThreadsResponse{Threads: threads})
}

// CreateThread handles POST /threads — creates a new thread.
func (h *Handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireThreadStore(w) {
		return
	}

	var req api.CreateThreadRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	store := h.defaultAgent.Store()
	exists, err := store.ThreadExists(req.ID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists {
		h.Error(w, http.StatusConflict, "thread already exists")
		return
	}

	if err := store.CreateThread(req.ID); err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	cfg, err := store.LoadConfig(req.ID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if trimmedCWD := strings.TrimSpace(req.CWD); trimmedCWD != "" {
		cfg.CWD = trimmedCWD
	} else if strings.TrimSpace(cfg.CWD) == "" {
		cfg.CWD = strings.TrimSpace(h.agentCwd)
	}
	if trimmedName := strings.TrimSpace(req.Name); trimmedName != "" {
		cfg.Name = trimmedName
		cfg.NameSource = thread.ThreadNameSourceUser
	}
	if strings.TrimSpace(cfg.CWD) != "" || strings.TrimSpace(req.Name) != "" {
		if err := store.SaveConfig(req.ID, cfg); err != nil {
			h.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	h.JSON(w, http.StatusCreated, h.threadResponse(req.ID, cfg))
}

// GetThread handles GET /threads/{id} — returns thread metadata.
func (h *Handler) GetThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireThreadStore(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	store := h.defaultAgent.Store()
	exists, err := store.ThreadExists(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, h.threadResponse(threadID, cfg))
}

// UpdateThread handles PUT/PATCH /threads/{id} — updates thread metadata.
func (h *Handler) UpdateThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireThreadStore(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	var req api.UpdateThreadRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" && strings.TrimSpace(req.CWD) == "" {
		h.Error(w, http.StatusBadRequest, "name or cwd is required")
		return
	}

	store := h.defaultAgent.Store()
	exists, err := store.ThreadExists(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if trimmedName := strings.TrimSpace(req.Name); trimmedName != "" {
		cfg.Name = trimmedName
		cfg.NameSource = thread.ThreadNameSourceUser
	}
	if trimmedCWD := strings.TrimSpace(req.CWD); trimmedCWD != "" {
		cfg.CWD = trimmedCWD
	}
	if err := store.SaveConfig(threadID, cfg); err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, h.threadResponse(threadID, cfg))
}

// DeleteThread handles DELETE /threads/{id} — removes a thread.
func (h *Handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireThreadStore(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	store := h.defaultAgent.Store()
	exists, err := store.ThreadExists(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	// Best-effort cancel if a completion is currently active for this thread.
	h.completions.Cancel(threadID)
	h.clearQueuedPromptTimer(threadID)

	if err := store.DeleteThread(threadID); err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, api.DeleteThreadResponse{Success: true})
}

// DeleteQueuedPrompt handles DELETE /threads/{id}/queue/{queueId} — removes a queued prompt.
func (h *Handler) DeleteQueuedPrompt(w http.ResponseWriter, r *http.Request) {
	if !h.requireThreadStore(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	queueID := chi.URLParam(r, "queueId")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}
	if strings.TrimSpace(queueID) == "" {
		h.Error(w, http.StatusBadRequest, "queueId is required")
		return
	}

	store := h.defaultAgent.Store()
	exists, err := store.ThreadExists(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	h.queueMu.Lock()
	cfg, removed, err := store.DeleteQueuedPrompt(threadID, queueID)
	if err == nil && removed {
		h.rescheduleQueuedPromptTimerLocked(threadID)
		h.completions.EmitChunkIfActive(threadID, thread.UpdateChunkFromConfig(threadID, cfg))
	}
	h.queueMu.Unlock()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !removed {
		h.Error(w, http.StatusNotFound, "queued prompt not found")
		return
	}

	h.JSON(w, http.StatusOK, api.DeleteQueuedPromptResponse{Success: true})
}

// UpdateQueuedPrompt handles PATCH /threads/{id}/queue/{queueId} — updates a queued prompt.
func (h *Handler) UpdateQueuedPrompt(w http.ResponseWriter, r *http.Request) {
	if !h.requireThreadStore(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	queueID := chi.URLParam(r, "queueId")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}
	if strings.TrimSpace(queueID) == "" {
		h.Error(w, http.StatusBadRequest, "queueId is required")
		return
	}

	var req api.UpdateQueuedPromptRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RunAfter == nil && !req.ClearRunAfter && req.Message == nil && req.Position == nil {
		h.Error(w, http.StatusBadRequest, "update field is required")
		return
	}

	var runAfter *time.Time
	if req.RunAfter != nil && !req.ClearRunAfter {
		value := strings.TrimSpace(*req.RunAfter)
		if value == "" {
			h.Error(w, http.StatusBadRequest, "runAfter is required")
			return
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			h.Error(w, http.StatusBadRequest, "runAfter must be RFC3339")
			return
		}
		runAfter = &parsed
	}

	store := h.defaultAgent.Store()
	exists, err := store.ThreadExists(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	h.queueMu.Lock()
	cfg, updated, err := store.UpdateQueuedPrompt(threadID, queueID, thread.QueuedPromptUpdate{
		RunAfter:      runAfter,
		ClearRunAfter: req.ClearRunAfter,
		Message:       req.Message,
		Position:      req.Position,
	})
	if err == nil && updated {
		h.rescheduleQueuedPromptTimerLocked(threadID)
		h.completions.EmitChunkIfActive(threadID, thread.UpdateChunkFromConfig(threadID, cfg))
	}
	h.queueMu.Unlock()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !updated {
		h.Error(w, http.StatusNotFound, "queued prompt not found")
		return
	}

	var queued *api.QueuedPrompt
	for _, item := range queuedPromptResponse(cfg.PromptQueue) {
		if item.ID == queueID {
			copyItem := item
			queued = &copyItem
			break
		}
	}

	h.JSON(w, http.StatusOK, api.UpdateQueuedPromptResponse{
		Success: true,
		Queue:   queued,
	})
}
