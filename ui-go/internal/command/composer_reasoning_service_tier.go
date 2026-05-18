package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// ComposerReasoning updates the session-scoped composer reasoning selection.
func (h *Handler) ComposerReasoning(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	reasoning := r.URL.Query().Get("reasoning")
	session.Save(func(view *viewmodel.ShellSnapshot) {
		view.Workspace.Composer.ReasoningValue = reasoning
		view.Workspace.Composer.ReasoningSet = true
	})
	w.WriteHeader(http.StatusNoContent)
}

// ComposerServiceTier updates the session-scoped composer service tier selection.
func (h *Handler) ComposerServiceTier(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	tier := r.URL.Query().Get("tier")
	session.Save(func(view *viewmodel.ShellSnapshot) {
		view.Workspace.Composer.ServiceTierValue = tier
		view.Workspace.Composer.ServiceTierSet = true
	})
	w.WriteHeader(http.StatusNoContent)
}
