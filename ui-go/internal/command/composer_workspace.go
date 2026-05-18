package command

import (
	"net/http"
	"strings"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go/internal/live"
	"github.com/obot-platform/discobot/ui-go/internal/state"
)

// ComposerWorkspace handles the pending-session workspace selector. The
// browser owns transient typing state; the server stores the last value it was
// asked to validate plus the resulting validation/suggestion state used for
// session creation.
func (h *Handler) ComposerWorkspace(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Warn("failed to parse composer workspace form", "error", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	if action == "input" || action == "suggestion" {
		value := r.FormValue("source_input")
		if action == "suggestion" {
			value = r.FormValue("value")
		}
		sourceType, shouldValidate := h.setPendingWorkspaceInput(session, value, r.FormValue("source_type"))
		if shouldValidate {
			h.validatePendingWorkspaceInput(r, session, value, sourceType)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	session.Save(func(view *viewmodel.ShellSnapshot) {
		selector := &view.Workspace.Composer.WorkspaceSelector
		switch action {
		case "option":
			setPendingWorkspaceOption(selector, r.FormValue("option"))
		case "reset":
			resetPendingWorkspaceSelector(selector)
		}
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setPendingWorkspaceInput(session *state.Session, value string, sourceTypeHint string) (string, bool) {
	sourceType := ""
	shouldValidate := false
	session.Save(func(view *viewmodel.ShellSnapshot) {
		selector := &view.Workspace.Composer.WorkspaceSelector
		if selector.SourceType == "" && sourceTypeHint != "" {
			setPendingWorkspaceSourceType(selector, sourceTypeHint)
		}
		selector.SourceInput = value
		selector.SetupMessage = ""
		selector.ValidationPath = ""
		selector.ValidationSourceType = ""
		selector.ValidationValid = false
		selector.ValidationError = ""
		selector.Suggestions = nil
		selector.HasSuggestionSelection = false
		selector.SelectedSuggestionIndex = -1

		trimmed := strings.TrimSpace(value)
		sourceType = selector.SourceType
		shouldValidate = trimmed != "" && sourceType != ""
		selector.Validating = shouldValidate
	})
	return sourceType, shouldValidate
}

func (h *Handler) validatePendingWorkspaceInput(r *http.Request, session *state.Session, value string, sourceType string) {
	if h.client == nil {
		session.Save(func(view *viewmodel.ShellSnapshot) {
			selector := &view.Workspace.Composer.WorkspaceSelector
			if selector.SourceInput != value || selector.SourceType != sourceType {
				return
			}
			selector.Validating = false
			selector.ValidationError = "Failed to validate workspace."
		})
		return
	}

	result, err := h.client.Workspaces.Validate(r.Context(), live.DefaultProjectID, api.ValidateWorkspaceRequest{
		Path:       strings.TrimSpace(value),
		SourceType: sourceType,
	})
	session.Save(func(view *viewmodel.ShellSnapshot) {
		selector := &view.Workspace.Composer.WorkspaceSelector
		if selector.SourceInput != value || selector.SourceType != sourceType {
			return
		}
		selector.Validating = false
		if err != nil {
			selector.ValidationError = err.Error()
			return
		}
		selector.ValidationPath = result.Path
		selector.ValidationSourceType = result.SourceType
		selector.ValidationValid = result.Valid
		selector.ValidationError = result.Error
		selector.Suggestions = make([]viewmodel.WorkspaceSuggestion, 0, len(result.Suggestions))
		for _, suggestion := range result.Suggestions {
			selector.Suggestions = append(selector.Suggestions, viewmodel.WorkspaceSuggestion{
				Value: suggestion.Value,
				Valid: suggestion.Valid,
			})
		}
	})
}

func setPendingWorkspaceOption(selector *viewmodel.ConversationWorkspaceSelectorSnapshot, option string) {
	selector.SelectedOption = option
	selector.SourceInput = ""
	selector.SetupMessage = ""
	selector.ValidationPath = ""
	selector.ValidationSourceType = ""
	selector.ValidationValid = false
	selector.ValidationError = ""
	selector.Validating = false
	selector.Suggestions = nil
	selector.HasSuggestionSelection = false
	selector.SelectedSuggestionIndex = -1
	selector.Branch = ""

	switch option {
	case "local-directory":
		selector.RequiresInput = true
		selector.SourceType = "local"
	case "git-repo":
		selector.RequiresInput = true
		selector.SourceType = "git"
	default:
		selector.RequiresInput = false
		selector.SourceType = ""
	}
}

func setPendingWorkspaceSourceType(selector *viewmodel.ConversationWorkspaceSelectorSnapshot, sourceType string) {
	switch sourceType {
	case "local":
		selector.SelectedOption = "local-directory"
		selector.RequiresInput = true
		selector.SourceType = "local"
	case "git":
		selector.SelectedOption = "git-repo"
		selector.RequiresInput = true
		selector.SourceType = "git"
	}
}

func resetPendingWorkspaceSelector(selector *viewmodel.ConversationWorkspaceSelectorSnapshot) {
	selector.RequiresInput = false
	selector.SourceType = ""
	selector.SourceInput = ""
	selector.SelectedOption = "new-workspace"
	selector.SetupMessage = ""
	selector.Validating = false
	selector.ValidationPath = ""
	selector.ValidationSourceType = ""
	selector.ValidationValid = false
	selector.ValidationError = ""
	selector.Suggestions = nil
	selector.HasSuggestionSelection = false
	selector.SelectedSuggestionIndex = -1
	selector.Branch = ""
}
