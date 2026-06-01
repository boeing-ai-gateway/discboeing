package command

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
	serverapi "github.com/obot-platform/discobot/server/api"
)

const composerWorkspaceRequestTimeout = 15 * time.Second

type composerWorkspaceInputPayload struct {
	Value string `json:"value"`
}

// ComposerWorkspaceSelect selects the workspace used by the new-session composer.
func (h *Handler) ComposerWorkspaceSelect(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspaceID")
	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		if !composerWorkspaceExists(*data, workspaceID) {
			return
		}
		composer := state.EnsureComposerPanelState(view)
		composer.SelectedWorkspaceID = workspaceID
		composer.WorkspaceSourceType = ""
		composer.WorkspaceSourceInput = ""
		composer.WorkspaceValidation = serverapi.ValidateWorkspaceResponse{}
		composer.WorkspaceValidationSet = false
		composer.WorkspaceSetupMessage = ""
	})
	writeNoContent(w)
}

// ComposerWorkspaceSourceStart switches the composer workspace selector into a
// source input mode for local directories or Git repositories.
func (h *Handler) ComposerWorkspaceSourceStart(w http.ResponseWriter, r *http.Request) {
	sourceType := normalizeComposerWorkspaceSourceType(chi.URLParam(r, "sourceType"))
	if sourceType == "" {
		http.Error(w, "invalid workspace source type", http.StatusBadRequest)
		return
	}

	h.view.SaveView(r.Context(), func(view *state.View) {
		composer := state.EnsureComposerPanelState(view)
		composer.SelectedWorkspaceID = ""
		composer.WorkspaceSourceType = sourceType
		composer.WorkspaceSourceInput = ""
		composer.WorkspaceValidation = serverapi.ValidateWorkspaceResponse{}
		composer.WorkspaceValidationSet = false
		composer.WorkspaceSetupMessage = ""
	})
	writeNoContent(w)
}

// ComposerWorkspaceSourceCancel returns the workspace selector to existing
// workspace/menu mode.
func (h *Handler) ComposerWorkspaceSourceCancel(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		composer := state.EnsureComposerPanelState(view)
		composer.WorkspaceSourceType = ""
		composer.WorkspaceSourceInput = ""
		composer.WorkspaceValidation = serverapi.ValidateWorkspaceResponse{}
		composer.WorkspaceValidationSet = false
		composer.WorkspaceSetupMessage = ""
	})
	writeNoContent(w)
}

// ComposerWorkspaceSourceInput updates and validates the pending source input.
func (h *Handler) ComposerWorkspaceSourceInput(w http.ResponseWriter, r *http.Request) {
	payload, err := decodeComposerWorkspaceInput(r)
	if err != nil {
		http.Error(w, "invalid workspace input payload", http.StatusBadRequest)
		return
	}

	var projectID string
	var sourceType string
	input := strings.TrimSpace(payload.Value)
	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		composer := state.EnsureComposerPanelState(view)
		composer.WorkspaceSourceInput = payload.Value
		composer.WorkspaceSetupMessage = ""
		composer.WorkspaceValidation = serverapi.ValidateWorkspaceResponse{}
		composer.WorkspaceValidationSet = false
		sourceType = composer.WorkspaceSourceType
		projectID = composerWorkspaceProjectID(*data, composer.SelectedWorkspaceID)
	})

	if input == "" || sourceType == "" {
		writeNoContent(w)
		return
	}

	validation, err := h.validateComposerWorkspace(r.Context(), projectID, input, sourceType)
	if err != nil {
		validation = invalidComposerWorkspaceValidation(input, sourceType, err)
	}
	h.view.SaveView(r.Context(), func(view *state.View) {
		composer := state.EnsureComposerPanelState(view)
		if composer.WorkspaceSourceType != sourceType || strings.TrimSpace(composer.WorkspaceSourceInput) != input {
			return
		}
		composer.WorkspaceValidation = validation
		composer.WorkspaceValidationSet = true
	})
	writeNoContent(w)
}

func decodeComposerWorkspaceInput(r *http.Request) (composerWorkspaceInputPayload, error) {
	var payload composerWorkspaceInputPayload
	if r.Body == nil {
		return payload, nil
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func (h *Handler) validateComposerWorkspace(ctx context.Context, projectID string, path string, sourceType string) (serverapi.ValidateWorkspaceResponse, error) {
	if h.client == nil {
		return serverapi.ValidateWorkspaceResponse{}, errors.New("workspace validation is unavailable")
	}
	if projectID == "" {
		return serverapi.ValidateWorkspaceResponse{}, errors.New("no project is available")
	}
	ctx, cancel := context.WithTimeout(ctx, composerWorkspaceRequestTimeout)
	defer cancel()
	validation, err := h.client.Project(projectID).Workspaces.ValidateInput(ctx, serverapi.ValidateWorkspaceRequest{
		Path:       path,
		SourceType: sourceType,
	})
	if err != nil {
		return serverapi.ValidateWorkspaceResponse{}, err
	}
	return *validation, nil
}

func invalidComposerWorkspaceValidation(path string, sourceType string, err error) serverapi.ValidateWorkspaceResponse {
	errorMessage := err.Error()
	return serverapi.ValidateWorkspaceResponse{
		Path:           path,
		SourceType:     sourceType,
		Valid:          false,
		Classification: "invalid",
		Error:          &errorMessage,
		Suggestions:    []serverapi.Suggestion{},
	}
}

func normalizeComposerWorkspaceSourceType(sourceType string) string {
	switch sourceType {
	case "local", "git":
		return sourceType
	default:
		return ""
	}
}

func composerWorkspaceEmptyInputMessage(sourceType string) string {
	if sourceType == "git" {
		return "Enter a Git repository URL."
	}
	return "Enter a local directory path."
}

func composerWorkspaceValidationMessage(validation serverapi.ValidateWorkspaceResponse, sourceType string) string {
	if validation.Error != nil && *validation.Error != "" {
		return *validation.Error
	}
	if sourceType == "git" {
		return "Enter a valid Git repository URL."
	}
	return "Enter a valid local directory path."
}

func composerWorkspaceExists(data state.Data, workspaceID string) bool {
	for _, workspace := range state.Workspaces(data) {
		if workspace.ID == workspaceID {
			return true
		}
	}
	return false
}

func composerWorkspaceProjectID(data state.Data, selectedWorkspaceID string) string {
	for _, project := range data.Project {
		for _, workspace := range project.Workspaces {
			if workspace.ID == selectedWorkspaceID {
				return workspace.ProjectID
			}
		}
	}
	for _, project := range data.Projects {
		if project.ID != "" {
			return project.ID
		}
	}
	for projectID := range data.Project {
		return projectID
	}
	return ""
}

func composerWorkspaceIDBySourceAndPath(data state.Data, sourceType string, path string) string {
	path = strings.TrimSpace(path)
	if sourceType == "" || path == "" {
		return ""
	}
	for _, workspace := range state.Workspaces(data) {
		if workspace.SourceType == sourceType && strings.TrimSpace(workspace.Path) == path {
			return workspace.ID
		}
	}
	return ""
}

func upsertComposerWorkspace(data *state.Data, workspace serverapi.Workspace) {
	if data.Project == nil {
		data.Project = map[string]state.ProjectData{}
	}
	project := data.Project[workspace.ProjectID]
	project.Project.ID = workspace.ProjectID
	if project.Workspace == nil {
		project.Workspace = map[string]serverapi.Workspace{}
	}
	project.Workspace[workspace.ID] = workspace
	updated := false
	for i := range project.Workspaces {
		if project.Workspaces[i].ID == workspace.ID {
			project.Workspaces[i] = workspace
			updated = true
			break
		}
	}
	if !updated {
		project.Workspaces = append(project.Workspaces, workspace)
	}
	data.Project[workspace.ProjectID] = project
}
