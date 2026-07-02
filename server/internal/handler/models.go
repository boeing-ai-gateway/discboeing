package handler

import (
	"net/http"

	api "github.com/boeing-ai-gateway/discboeing/server/api"
	"github.com/boeing-ai-gateway/discboeing/server/internal/middleware"
	"github.com/boeing-ai-gateway/discboeing/server/internal/providers"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
)

// toModelInfos converts service models to API response models.
func toModelInfos(models []service.Model) []api.ModelInfo {
	modelInfos := make([]api.ModelInfo, len(models))
	for i, m := range models {
		modelInfos[i] = api.ModelInfo{
			ID:       m.ID,
			Name:     m.Name,
			Provider: m.Provider,
		}
		if m.Description != "" {
			modelInfos[i].Description = &m.Description
		}
		if m.Reasoning {
			modelInfos[i].Reasoning = &m.Reasoning
		}
		if len(m.ReasoningLevels) > 0 {
			reasoningLevels := append([]string(nil), m.ReasoningLevels...)
			modelInfos[i].ReasoningLevels = &reasoningLevels
		}
		if m.DefaultReasoning != "" {
			modelInfos[i].DefaultReasoning = &m.DefaultReasoning
		}
		if len(m.ServiceTiers) > 0 {
			serviceTiers := append([]string(nil), m.ServiceTiers...)
			modelInfos[i].ServiceTiers = &serviceTiers
		}
	}
	return modelInfos
}

// GetProjectModels returns available models for a project based on configured credentials.
func (h *Handler) GetProjectModels(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if projectID == "" {
		h.Error(w, http.StatusBadRequest, "Project ID is required")
		return
	}

	models, err := h.modelsService.GetModelsForProject(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to get models for project")
		return
	}

	h.JSON(w, http.StatusOK, api.ModelsResponse{Models: toModelInfos(models)})
}

// GetAuthProviders returns available auth providers from models.dev data
func (h *Handler) GetAuthProviders(w http.ResponseWriter, _ *http.Request) {
	h.JSON(w, http.StatusOK, map[string]any{"authProviders": providers.GetAll()})
}
