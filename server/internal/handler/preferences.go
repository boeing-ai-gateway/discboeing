package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/service"
	"github.com/obot-platform/discobot/server/internal/store"
)

// ListPreferences returns all preferences for the authenticated user
func (h *Handler) ListPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		h.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	prefs, err := h.preferenceService.ListPreferences(r.Context(), userID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list preferences")
		return
	}

	h.JSON(w, http.StatusOK, api.PreferencesResponse{Preferences: preferenceResponses(prefs)})
}

// GetPreference returns a single preference by key
func (h *Handler) GetPreference(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		h.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		h.Error(w, http.StatusBadRequest, "Key is required")
		return
	}

	pref, err := h.preferenceService.GetPreference(r.Context(), userID, key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Preference not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get preference")
		return
	}

	h.JSON(w, http.StatusOK, preferenceResponse(pref))
}

// SetPreference creates or updates a preference
func (h *Handler) SetPreference(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		h.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		h.Error(w, http.StatusBadRequest, "Key is required")
		return
	}

	var req api.SetPreferenceRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	pref, err := h.preferenceService.SetPreference(r.Context(), userID, key, req.Value)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to set preference")
		return
	}

	h.JSON(w, http.StatusOK, preferenceResponse(pref))
}

// SetPreferences sets multiple preferences at once
func (h *Handler) SetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		h.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req api.SetPreferencesRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var prefs []*service.UserPreference
	for key, value := range req.Preferences {
		pref, err := h.preferenceService.SetPreference(r.Context(), userID, key, value)
		if err != nil {
			h.Error(w, http.StatusInternalServerError, "Failed to set preferences")
			return
		}
		prefs = append(prefs, pref)
	}

	h.JSON(w, http.StatusOK, api.PreferencesResponse{Preferences: preferenceResponses(prefs)})
}

// DeletePreference deletes a preference by key
func (h *Handler) DeletePreference(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		h.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	key := chi.URLParam(r, "key")
	if key == "" {
		h.Error(w, http.StatusBadRequest, "Key is required")
		return
	}

	if err := h.preferenceService.DeletePreference(r.Context(), userID, key); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Preference not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to delete preference")
		return
	}

	h.JSON(w, http.StatusOK, api.MessageResponse{Success: new(true)})
}

func preferenceResponse(pref *service.UserPreference) api.PreferenceResponse {
	resp := api.PreferenceResponse{
		Key:   pref.Key,
		Value: pref.Value,
	}
	if pref.UpdatedAt != "" {
		resp.UpdatedAt = &pref.UpdatedAt
	}
	return resp
}

func preferenceResponses(prefs []*service.UserPreference) []api.PreferenceResponse {
	responses := make([]api.PreferenceResponse, 0, len(prefs))
	for _, pref := range prefs {
		responses = append(responses, preferenceResponse(pref))
	}
	return responses
}
