package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/services"
)

func (h *Handlers) AddOrganizationMember(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("addOrganizationMember", w, r)
}

func (h *Handlers) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("createOrganization", w, r)
}

func (h *Handlers) CreateOrganizationOAuthApplication(w http.ResponseWriter, r *http.Request) {
	var req services.OAuthApplicationInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	app, err := h.OAuthApplications.CreateOAuthApplication(r.Context(), chi.URLParam(r, "organizationDomain"), createdByPrincipal(r), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *Handlers) DeleteOrganizationOAuthApplication(w http.ResponseWriter, r *http.Request) {
	err := h.OAuthApplications.DeleteOAuthApplication(r.Context(), chi.URLParam(r, "organizationDomain"), chi.URLParam(r, "oauthApplicationId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) GetOrganization(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("getOrganization", w, r)
}

func (h *Handlers) GetOrganizationOAuthApplication(w http.ResponseWriter, r *http.Request) {
	app, err := h.OAuthApplications.GetOAuthApplication(r.Context(), chi.URLParam(r, "organizationDomain"), chi.URLParam(r, "oauthApplicationId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handlers) ListOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("listOrganizationMembers", w, r)
}

func (h *Handlers) ListOrganizationOAuthApplications(w http.ResponseWriter, r *http.Request) {
	items, err := h.OAuthApplications.ListOAuthApplications(r.Context(), chi.URLParam(r, "organizationDomain"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("listOrganizations", w, r)
}

func (h *Handlers) RemoveOrganizationMember(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("removeOrganizationMember", w, r)
}

func (h *Handlers) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("updateOrganization", w, r)
}

func (h *Handlers) UpdateOrganizationMember(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("updateOrganizationMember", w, r)
}

func (h *Handlers) UpdateOrganizationOAuthApplication(w http.ResponseWriter, r *http.Request) {
	var req services.OAuthApplicationInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	app, err := h.OAuthApplications.UpdateOAuthApplication(r.Context(), chi.URLParam(r, "organizationDomain"), chi.URLParam(r, "oauthApplicationId"), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func createdByPrincipal(r *http.Request) string {
	user, ok := auth.UserInfoFromRequest(r)
	if !ok || user == nil {
		return auth.AnonymousName
	}
	if user.Name != "" {
		return user.Name
	}
	return user.UID
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case services.IsInvalidRequest(err):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case services.IsNotFound(err):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case services.IsUnavailable(err):
		writeError(w, http.StatusInternalServerError, "service_unavailable", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
