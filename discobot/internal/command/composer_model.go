package command

import (
	"net/http"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// ComposerModelSettingsSelect selects the model, reasoning effort, and service
// tier used by the new-session composer.
func (h *Handler) ComposerModelSettingsSelect(w http.ResponseWriter, r *http.Request) {
	modelID := r.URL.Query().Get("model")
	reasoning := r.URL.Query().Get("reasoning")
	serviceTier := r.URL.Query().Get("serviceTier")

	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		composer := state.EnsureComposerPanelState(view)
		if modelID == "" {
			composer.SelectedModelID = ""
			composer.SelectedReasoning = ""
			composer.SelectedServiceTier = ""
			return
		}

		model, ok := composerModelByID(*data, composer.SelectedWorkspaceID, modelID)
		if !ok {
			return
		}
		if reasoning != "" && !stringInSlice(reasoning, stringSlice(model.ReasoningLevels)) {
			return
		}
		if serviceTier != "" && !stringInSlice(serviceTier, stringSlice(model.ServiceTiers)) {
			return
		}

		composer.SelectedModelID = modelID
		composer.SelectedReasoning = reasoning
		composer.SelectedServiceTier = serviceTier
	})
	writeNoContent(w)
}

func composerModelByID(data state.Data, selectedWorkspaceID string, modelID string) (serverModelInfo, bool) {
	for _, model := range composerModelsForWorkspace(data, selectedWorkspaceID) {
		if model.ID == modelID {
			return model, true
		}
	}
	return serverModelInfo{}, false
}

type serverModelInfo struct {
	ID              string
	ReasoningLevels *[]string
	ServiceTiers    *[]string
}

func composerModelsForWorkspace(data state.Data, selectedWorkspaceID string) []serverModelInfo {
	projectID := composerProjectID(data, selectedWorkspaceID)
	var models []serverModelInfo
	for id, project := range data.Project {
		if projectID != "" && id != projectID {
			continue
		}
		for _, model := range project.Models {
			models = append(models, serverModelInfo{
				ID:              model.ID,
				ReasoningLevels: model.ReasoningLevels,
				ServiceTiers:    model.ServiceTiers,
			})
		}
	}
	return models
}

func composerProjectID(data state.Data, selectedWorkspaceID string) string {
	for _, project := range data.Project {
		for _, workspace := range project.Workspaces {
			if workspace.ID == selectedWorkspaceID {
				return workspace.ProjectID
			}
		}
	}
	return ""
}

func stringSlice(values *[]string) []string {
	if values == nil {
		return nil
	}
	return *values
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
