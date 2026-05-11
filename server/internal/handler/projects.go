package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/service"
)

// ListProjects returns all projects for the current user
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		h.Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	projects, err := h.projectService.ListProjects(r.Context(), userID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list projects")
		return
	}

	h.JSON(w, http.StatusOK, projects)
}

// CreateProject creates a new project
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		h.Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Name == "" {
		h.Error(w, http.StatusBadRequest, "Name is required")
		return
	}

	project, err := h.projectService.CreateProject(r.Context(), userID, req.Name)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to create project")
		return
	}

	h.JSON(w, http.StatusCreated, project)
}

// GetProject returns a single project
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	project, err := h.projectService.GetProject(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusNotFound, "Project not found")
		return
	}

	h.JSON(w, http.StatusOK, project)
}

// UpdateProject updates a project
func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	// Check if user is admin or owner
	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		h.Error(w, http.StatusForbidden, "Admin access required")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	project, err := h.projectService.UpdateProject(r.Context(), projectID, req.Name)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to update project")
		return
	}

	h.JSON(w, http.StatusOK, project)
}

// DeleteProject deletes a project
func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	// Check if user is owner
	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || role != "owner" {
		h.Error(w, http.StatusForbidden, "Owner access required")
		return
	}

	if err := h.projectService.DeleteProject(r.Context(), projectID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to delete project")
		return
	}

	h.JSON(w, http.StatusOK, map[string]bool{"success": true})
}

// GetProviderResources returns provider VM resources.
func (h *Handler) GetProviderResources(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	resources, err := h.sandboxService.GetProviderResourcesForProvider(r.Context(), projectID, h.sandboxService.DefaultProviderName())
	if err != nil {
		if errors.Is(err, sandbox.ErrProviderResourcesUnsupported) {
			h.Error(w, http.StatusNotImplemented, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get provider resources")
		return
	}

	h.JSON(w, http.StatusOK, resources)
}

// UpdateProviderResources updates provider VM resources.
func (h *Handler) UpdateProviderResources(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		h.Error(w, http.StatusForbidden, "Admin access required")
		return
	}

	var req service.UpdateProviderResourcesRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := h.sandboxService.UpdateProviderResourcesForProvider(r.Context(), projectID, h.sandboxService.DefaultProviderName(), req)
	if err != nil {
		var validationErr *service.RequestValidationError
		switch {
		case errors.As(err, &validationErr):
			h.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, sandbox.ErrProviderResourcesUnsupported):
			h.Error(w, http.StatusNotImplemented, err.Error())
		default:
			h.Error(w, http.StatusInternalServerError, "Failed to update provider resources")
		}
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// GetProjectInspection returns project inspection-container details.
func (h *Handler) GetProjectInspection(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	info, err := h.sandboxService.GetProjectInspectionForProvider(r.Context(), projectID, h.sandboxService.DefaultProviderName())
	if err != nil {
		if errors.Is(err, sandbox.ErrProjectInspectionUnsupported) {
			h.Error(w, http.StatusNotImplemented, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get project inspection info")
		return
	}

	h.JSON(w, http.StatusOK, info)
}

// ProjectInspectionTerminalWebSocket attaches to the project inspection shell.
func (h *Handler) ProjectInspectionTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	if projectID == "" {
		h.Error(w, http.StatusBadRequest, "project ID is required")
		return
	}

	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		h.Error(w, http.StatusForbidden, "Admin access required")
		return
	}

	rows, _ := strconv.Atoi(r.URL.Query().Get("rows"))
	cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
	if rows < minTermRows {
		rows = minTermRows
	}
	if cols < minTermCols {
		cols = minTermCols
	}

	termUserID := userID
	if termUserID == "" {
		termUserID = "anonymous"
	}

	ctx := r.Context()
	termKey := "project-inspection:" + projectID + ":" + termUserID
	termSession, err := h.terminalManager.GetOrCreate(ctx, termKey, func(ctx context.Context) (sandbox.PTY, error) {
		return h.sandboxService.AttachProjectInspectionForProvider(ctx, projectID, h.sandboxService.DefaultProviderName(), sandbox.AttachOptions{
			Cmd:  []string{"nsenter", "-m", "-t", "1", "--", "/bin/bash", "-lc", "cd /root && exec /bin/bash -l"},
			Rows: rows,
			Cols: cols,
			User: "root",
		})
	})
	if err != nil {
		if errors.Is(err, sandbox.ErrProjectInspectionUnsupported) {
			h.Error(w, http.StatusNotImplemented, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, "failed to attach to inspection terminal")
		return
	}

	if err := termSession.Resize(ctx, rows, cols); err != nil {
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	sub := termSession.Subscribe()
	defer termSession.Unsubscribe(sub)

	handlePersistentTerminalSession(ctx, termSession, sub, conn)
}

// ListProjectMembers returns project members
func (h *Handler) ListProjectMembers(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	members, err := h.projectService.ListMembers(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list members")
		return
	}

	h.JSON(w, http.StatusOK, members)
}

// RemoveProjectMember removes a member from a project
func (h *Handler) RemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")
	targetUserID := chi.URLParam(r, "userId")

	// Check if current user is admin or owner
	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		h.Error(w, http.StatusForbidden, "Admin access required")
		return
	}

	// Cannot remove owner
	targetRole, _ := h.projectService.GetMemberRole(r.Context(), projectID, targetUserID)
	if targetRole == "owner" {
		h.Error(w, http.StatusForbidden, "Cannot remove project owner")
		return
	}

	if err := h.projectService.RemoveMember(r.Context(), projectID, targetUserID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to remove member")
		return
	}

	h.JSON(w, http.StatusOK, map[string]bool{"success": true})
}

// CreateInvitation creates a project invitation
func (h *Handler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	// Check if user is admin or owner
	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		h.Error(w, http.StatusForbidden, "Admin access required")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Email == "" {
		h.Error(w, http.StatusBadRequest, "Email is required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}

	invitation, err := h.projectService.CreateInvitation(r.Context(), projectID, userID, req.Email, req.Role)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to create invitation")
		return
	}

	h.JSON(w, http.StatusCreated, invitation)
}

// AcceptInvitation accepts a project invitation
func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	userID := middleware.GetUserID(r.Context())

	if err := h.projectService.AcceptInvitation(r.Context(), token, userID); err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ListProjectCacheVolumes lists cache volumes for a project
func (h *Handler) ListProjectCacheVolumes(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	volumes, err := h.sandboxService.ListProjectCacheVolumes(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, sandbox.ErrProjectCacheUnsupported) {
			h.Error(w, http.StatusNotImplemented, "Cache volumes not supported by provider")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to list cache volumes")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{
		"volumes": volumes,
	})
}

// DeleteProjectCacheVolume deletes the cache volume for a project
func (h *Handler) DeleteProjectCacheVolume(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectId")

	// Check if user is admin or owner
	userID := middleware.GetUserID(r.Context())
	role, err := h.projectService.GetMemberRole(r.Context(), projectID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		h.Error(w, http.StatusForbidden, "Admin access required")
		return
	}

	if err := h.sandboxService.ClearProjectCache(r.Context(), projectID); err != nil {
		if errors.Is(err, sandbox.ErrProjectCacheUnsupported) {
			h.Error(w, http.StatusNotImplemented, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to delete cache volume")
		return
	}

	h.JSON(w, http.StatusOK, map[string]bool{"success": true})
}
