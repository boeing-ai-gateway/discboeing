package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/boeing-ai-gateway/discboeing/server/internal/middleware"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/sandboxapi"
)

// GetHooksStatus returns hook evaluation status for a session's sandbox.
// GET /api/projects/{projectId}/sessions/{sessionId}/hooks/status
func (h *Handler) GetHooksStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	result, err := h.chatService.GetHooksStatus(ctx, projectID, sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// GetHooksState returns hook status and inline outputs for a session's sandbox.
// GET /api/projects/{projectId}/sessions/{sessionId}/hooks/state
func (h *Handler) GetHooksState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	result, err := h.chatService.GetHooksState(ctx, projectID, sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// ListWorkspaceChangeCommits returns workspace change commits for a session's sandbox.
// GET /api/projects/{projectId}/sessions/{sessionId}/workspace-change-commits
func (h *Handler) ListWorkspaceChangeCommits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	result, err := h.chatService.ListWorkspaceChangeCommits(ctx, projectID, sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// UpdateHooksExecution toggles whether hook failures report back to the LLM.
// PATCH /api/projects/{projectId}/sessions/{sessionId}/hooks/execution
func (h *Handler) UpdateHooksExecution(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	var req sandboxapi.UpdateHooksExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.chatService.UpdateHooksExecution(ctx, projectID, sessionID, req.Paused)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// UpdateHookExecution toggles whether one hook reports failures back to the LLM.
// PATCH /api/projects/{projectId}/sessions/{sessionId}/hooks/{hookId}/execution
func (h *Handler) UpdateHookExecution(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	hookID := chi.URLParam(r, "hookId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if hookID == "" {
		h.Error(w, http.StatusBadRequest, "hookId is required")
		return
	}

	var req sandboxapi.UpdateHooksExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.chatService.UpdateHookExecution(ctx, projectID, sessionID, hookID, req.Paused)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// GetHookOutput returns the output log for a specific hook in a session's sandbox.
// GET /api/projects/{projectId}/sessions/{sessionId}/hooks/{hookId}/output
func (h *Handler) GetHookOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	hookID := chi.URLParam(r, "hookId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if hookID == "" {
		h.Error(w, http.StatusBadRequest, "hookId is required")
		return
	}

	result, err := h.chatService.GetHookOutput(ctx, projectID, sessionID, hookID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// DownloadHookOutput returns the full output log for a specific hook as an attachment.
// GET /api/projects/{projectId}/sessions/{sessionId}/hooks/{hookId}/output/download
func (h *Handler) DownloadHookOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	hookID := chi.URLParam(r, "hookId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if hookID == "" {
		h.Error(w, http.StatusBadRequest, "hookId is required")
		return
	}

	result, err := h.chatService.DownloadHookOutput(ctx, projectID, sessionID, hookID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", hookID+".log"))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result)
}

// RerunHook manually reruns a specific hook in a session's sandbox.
// POST /api/projects/{projectId}/sessions/{sessionId}/hooks/{hookId}/rerun
func (h *Handler) RerunHook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	hookID := chi.URLParam(r, "hookId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if hookID == "" {
		h.Error(w, http.StatusBadRequest, "hookId is required")
		return
	}

	result, err := h.chatService.RerunHook(ctx, projectID, sessionID, hookID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}
