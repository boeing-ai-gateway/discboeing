package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/promptqueue"
)

const defaultActivityStreamSnapshotInterval = time.Minute

func (h *Handler) requireConversations(w http.ResponseWriter) bool {
	if h.threadManager == nil {
		h.Error(w, http.StatusNotImplemented, "thread manager unavailable")
		return false
	}
	return true
}

func (h *Handler) threadResponse(info agent.ThreadInfo) api.Thread {
	h.applyThreadStateOverlay(&info)
	queue := h.loadPromptQueue(info.ID)

	thread := api.Thread{
		ID:           info.ID,
		Name:         strings.TrimSpace(info.Name),
		CWD:          strings.TrimSpace(info.CWD),
		Phase:        strings.TrimSpace(info.Phase),
		LastMessage:  strings.TrimSpace(info.LastMessage),
		ErrorMessage: strings.TrimSpace(info.ErrorMessage),
		Model:        info.Model,
		Reasoning:    info.Reasoning,
		ServiceTier:  info.ServiceTier,
		State:        string(info.State),
		TokenUsage: api.TokenUsageInfo{
			Total:           info.TokenUsage.Total,
			LastStep:        info.TokenUsage.LastStep,
			LastTurn:        info.TokenUsage.LastTurn,
			ModelMaxTokens:  info.TokenUsage.ModelMaxTokens,
			MaxOutputTokens: info.TokenUsage.MaxOutputTokens,
			Prices:          info.TokenUsage.Prices,
		},
		PendingQuestion: info.PendingQuestion,
		ActiveCommand:   strings.TrimSpace(info.ActiveCommand),
		PromptQueue:     queuedPromptResponse(queue),
		Metadata:        info.Metadata,
	}
	if state := h.threadActivityState(thread); state != nil {
		thread.ActivityStatus = &api.ThreadActivity{
			Status:       state.Status,
			Reason:       state.Reason,
			CompletionID: state.CompletionID,
			QueueCount:   state.QueueCount,
			NextRunAfter: state.NextRunAfter,
			Message:      state.Message,
		}
	}
	return thread
}

func tokenUsageDetailsResponse(details agent.ThreadTokenUsageDetails) api.ThreadTokenUsageDetails {
	turns := make([]api.TokenUsageTurn, 0, len(details.Turns))
	for _, turn := range details.Turns {
		steps := make([]api.TokenUsageStep, 0, len(turn.Steps))
		for _, step := range turn.Steps {
			toolCalls := make([]api.TokenUsageToolCall, 0, len(step.ToolCalls))
			for _, toolCall := range step.ToolCalls {
				toolCalls = append(toolCalls, api.TokenUsageToolCall{
					ID:   toolCall.ID,
					Name: toolCall.Name,
				})
			}
			steps = append(steps, api.TokenUsageStep{
				Index:              step.Index,
				AssistantMessageID: step.AssistantMessageID,
				ToolCalls:          toolCalls,
				Usage:              step.Usage,
			})
		}
		turns = append(turns, api.TokenUsageTurn{
			ID:              turn.ID,
			Model:           turn.Model,
			Reasoning:       turn.Reasoning,
			ServiceTier:     turn.ServiceTier,
			ModelMaxTokens:  turn.ModelMaxTokens,
			MaxOutputTokens: turn.MaxOutputTokens,
			Prices:          turn.Prices,
			Usage:           turn.Usage,
			StartedAt:       turn.StartedAt,
			FinishedAt:      turn.FinishedAt,
			Steps:           steps,
		})
	}

	return api.ThreadTokenUsageDetails{
		ThreadID: details.ThreadID,
		Summary: api.TokenUsageInfo{
			Total:           details.Summary.Total,
			LastStep:        details.Summary.LastStep,
			LastTurn:        details.Summary.LastTurn,
			ModelMaxTokens:  details.Summary.ModelMaxTokens,
			MaxOutputTokens: details.Summary.MaxOutputTokens,
			Prices:          details.Summary.Prices,
		},
		Turns: turns,
	}
}

func (h *Handler) threadActivityState(thread api.Thread) *api.SessionThreadActivityState {
	state := api.SessionThreadActivityState{ThreadID: thread.ID}
	if thread.PendingQuestion {
		state.Status = "needs_attention"
		state.Reason = "pending_question"
		return &state
	}
	if thread.ErrorMessage != "" {
		state.Status = "needs_attention"
		state.Reason = "thread_error"
		state.Message = thread.ErrorMessage
		return &state
	}
	if h.conversations != nil {
		state.CompletionID = h.conversations.ActiveCompletionID(thread.ID)
	}
	if thread.ActiveCommand != "" || state.CompletionID != "" {
		state.Status = "running"
		state.Reason = "completion"
		state.Message = thread.ActiveCommand
		return &state
	}
	switch strings.TrimSpace(thread.State) {
	case "interrupted":
		state.Status = "needs_attention"
		state.Reason = "interrupted"
		return &state
	case "cancelled":
		state.Status = "needs_attention"
		state.Reason = "cancelled"
		return &state
	}
	if len(thread.PromptQueue) > 0 {
		state.Status = "queued"
		state.Reason = "queued_prompt"
		state.QueueCount = len(thread.PromptQueue)
		for _, queued := range thread.PromptQueue {
			if queued.RunAfter == "" {
				continue
			}
			if state.NextRunAfter == "" || queued.RunAfter < state.NextRunAfter {
				state.NextRunAfter = queued.RunAfter
			}
		}
		return &state
	}
	return nil
}

func activityPriority(status string) int {
	switch status {
	case "needs_attention":
		return 4
	case "running":
		return 3
	case "queued":
		return 2
	case "unknown":
		return 1
	default:
		return 0
	}
}

func (h *Handler) sessionActivityResponse(threads []api.Thread) api.SessionActivityResponse {
	resp := api.SessionActivityResponse{Status: "idle"}
	for _, thread := range threads {
		state := h.threadActivityState(thread)
		if state == nil {
			continue
		}
		resp.Threads = append(resp.Threads, *state)
		switch state.Status {
		case "needs_attention":
			resp.NeedsAttentionCount++
		case "running":
			resp.RunningCount++
		case "queued":
			resp.QueuedCount++
		case "unknown":
			resp.UnknownCount++
		}
		if activityPriority(state.Status) > activityPriority(resp.Status) {
			resp.Status = state.Status
			resp.Reason = state.Reason
			resp.RepresentativeThreadID = state.ThreadID
		}
	}
	return resp
}

func (h *Handler) sessionActivitySnapshot() (api.SessionActivityResponse, error) {
	infos, err := h.threadManager.ListThreadInfos()
	if err != nil {
		return api.SessionActivityResponse{}, err
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })

	threads := make([]api.Thread, 0, len(infos))
	for _, info := range infos {
		threads = append(threads, h.threadResponse(info))
	}

	return h.sessionActivityResponse(threads), nil
}

func (h *Handler) applyThreadStateOverlay(info *agent.ThreadInfo) {
	if info == nil || h.conversations == nil || strings.TrimSpace(info.ID) == "" {
		return
	}
	if h.conversations.ActiveCompletionID(info.ID) != "" {
		return
	}
	if interrupted, err := h.conversations.HasInterruptedTurn(info.ID); err == nil && interrupted {
		info.State = agent.ThreadStateInterrupted
	}
}

func (h *Handler) loadPromptQueue(threadID string) []promptqueue.Prompt {
	if h.promptQueue == nil {
		return nil
	}
	queue, err := h.promptQueue.List(threadID)
	if err != nil {
		return nil
	}
	return queue
}

func queuedPromptResponse(queue []promptqueue.Prompt) []api.QueuedPrompt {
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
			ID:          prompt.ID,
			CreatedAt:   prompt.CreatedAt.UTC().Format(time.RFC3339Nano),
			RunAfter:    runAfter,
			Message:     prompt.Message,
			Model:       prompt.Model,
			Reasoning:   prompt.Reasoning,
			ServiceTier: prompt.ServiceTier,
		})
	}
	return items
}

// ListThreads handles GET /threads — lists all threads.
func (h *Handler) ListThreads(w http.ResponseWriter, _ *http.Request) {
	if !h.requireConversations(w) {
		return
	}

	infos, err := h.threadManager.ListThreadInfos()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })

	threads := make([]api.Thread, 0, len(infos))
	for _, info := range infos {
		threads = append(threads, h.threadResponse(info))
	}

	h.JSON(w, http.StatusOK, api.ListThreadsResponse{Threads: threads})
}

// GetSessionActivity handles GET /threads/activity — returns one aggregate
// snapshot of non-idle activity for the whole sandbox session.
func (h *Handler) GetSessionActivity(w http.ResponseWriter, _ *http.Request) {
	if !h.requireConversations(w) {
		return
	}

	snapshot, err := h.sessionActivitySnapshot()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, snapshot)
}

// StreamSessionActivity handles GET /threads/activity/stream. The stream emits
// one initial aggregate snapshot, then emits another snapshot whenever agent
// state changes in a way that may affect session activity.
func (h *Handler) StreamSessionActivity(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	var changes <-chan struct{}
	unsubscribe := func() {}
	if h.activity != nil {
		changes, unsubscribe = h.activity.Subscribe()
	}
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writeSnapshot := func() bool {
		snapshot, err := h.sessionActivitySnapshot()
		if err != nil {
			log.Printf("activity stream: failed to build session activity snapshot: %v", err)
			data, marshalErr := json.Marshal(api.ErrorResponse{Error: err.Error()})
			if marshalErr == nil {
				writeSSEEvent(w, "", "error", data)
				flusher.Flush()
			}
			return false
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			log.Printf("activity stream: failed to marshal session activity snapshot: %v", err)
			return false
		}
		writeSSEEvent(w, "", "activity", data)
		flusher.Flush()
		return true
	}

	if !writeSnapshot() {
		return
	}

	pingEvery := h.chatPingEvery
	if pingEvery <= 0 {
		pingEvery = defaultChatStreamPingInterval
	}
	pingTicker := time.NewTicker(pingEvery)
	defer pingTicker.Stop()

	activityEvery := h.activityEvery
	if activityEvery <= 0 {
		activityEvery = defaultActivityStreamSnapshotInterval
	}
	activityTicker := time.NewTicker(activityEvery)
	defer activityTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-changes:
			for {
				select {
				case <-changes:
					continue
				default:
				}
				break
			}
			if !writeSnapshot() {
				return
			}
		case <-activityTicker.C:
			if !writeSnapshot() {
				return
			}
		case <-pingTicker.C:
			writeSSEEvent(w, "", "ping", json.RawMessage(`{}`))
			flusher.Flush()
		}
	}
}

// CreateThread handles POST /threads — creates a new thread.
func (h *Handler) CreateThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
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

	info, err := h.threadManager.CreateThread(r.Context(), agent.CreateThreadRequest{
		ID:    req.ID,
		Name:  req.Name,
		CWD:   req.CWD,
		Phase: req.Phase,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.Error(w, http.StatusConflict, "thread already exists")
			return
		}
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.notifyActivityChanged()
	h.JSON(w, http.StatusCreated, h.threadResponse(info))
}

// GetThread handles GET /threads/{id} — returns thread metadata.
func (h *Handler) GetThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	info, err := h.threadManager.GetThreadInfo(threadID)
	if err != nil {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	h.JSON(w, http.StatusOK, h.threadResponse(info))
}

// GetThreadTokenUsage handles GET /threads/{id}/token-usage.
func (h *Handler) GetThreadTokenUsage(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	details, err := h.threadManager.GetThreadTokenUsageDetails(threadID)
	if err != nil {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	h.JSON(w, http.StatusOK, tokenUsageDetailsResponse(details))
}

// UpdateThread handles PUT/PATCH /threads/{id} — updates thread metadata.
func (h *Handler) UpdateThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
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
	if strings.TrimSpace(req.Name) == "" && req.Phase == nil {
		h.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	var update agent.UpdateThreadRequest
	if strings.TrimSpace(req.Name) != "" {
		name := strings.TrimSpace(req.Name)
		update.Name = &name
	}
	if req.Phase != nil {
		phase := strings.TrimSpace(*req.Phase)
		update.Phase = &phase
	}
	info, err := h.threadManager.UpdateThread(r.Context(), threadID, update)
	if err != nil {
		if strings.Contains(err.Error(), "invalid thread phase") {
			h.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	if req.Phase != nil && h.hookManager != nil {
		h.hookManager.TriggerEvaluation(threadID)
	}
	h.notifyActivityChanged()
	h.JSON(w, http.StatusOK, h.threadResponse(info))
}

// DeleteThread handles DELETE /threads/{id} — removes a thread.
func (h *Handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
		return
	}

	threadID := chi.URLParam(r, "id")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	if h.promptQueue != nil {
		h.promptQueue.ClearTimer(threadID)
	}

	if err := h.threadManager.DeleteThread(r.Context(), threadID); err != nil {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	h.notifyActivityChanged()
	h.JSON(w, http.StatusOK, api.DeleteThreadResponse{Success: true})
}

// DeleteQueuedPrompt handles DELETE /threads/{id}/queue/{queueId} — removes a queued prompt.
func (h *Handler) DeleteQueuedPrompt(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
		return
	}
	if h.promptQueue == nil {
		h.Error(w, http.StatusNotImplemented, "prompt queue unavailable")
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

	if _, err := h.threadManager.GetThreadInfo(threadID); err != nil {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	_, removed, err := h.promptQueue.Delete(threadID, queueID)
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
	if !h.requireConversations(w) {
		return
	}
	if h.promptQueue == nil {
		h.Error(w, http.StatusNotImplemented, "prompt queue unavailable")
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

	if _, err := h.threadManager.GetThreadInfo(threadID); err != nil {
		h.Error(w, http.StatusNotFound, "thread not found")
		return
	}

	queue, updated, err := h.promptQueue.UpdatePrompt(threadID, queueID, promptqueue.Update{
		RunAfter:      runAfter,
		ClearRunAfter: req.ClearRunAfter,
		Message:       req.Message,
		Position:      req.Position,
	})
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !updated {
		h.Error(w, http.StatusNotFound, "queued prompt not found")
		return
	}

	var queued *api.QueuedPrompt
	for _, item := range queuedPromptResponse(queue) {
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
