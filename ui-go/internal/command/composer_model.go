package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// ComposerModel updates the session-scoped composer model selection.
func (h *Handler) ComposerModel(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	modelID := r.URL.Query().Get("model")
	session.Save(func(view *viewmodel.ShellSnapshot) {
		view.Workspace.Composer.ModelID = modelID
		view.Workspace.Composer.ModelSelectionSet = true
		view.Workspace.Composer.ReasoningValue = ""
		view.Workspace.Composer.ReasoningSet = false
		view.Workspace.Composer.ServiceTierValue = ""
		view.Workspace.Composer.ServiceTierSet = false
	})
	w.WriteHeader(http.StatusNoContent)
}
