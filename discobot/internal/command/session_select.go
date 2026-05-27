package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionSelect selects a session row and ensures its details are expanded.
func (h *Handler) SessionSelect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	generation := h.view.SaveView(func(view *state.View) {
		view.SelectedSessionID = id
		if view.ExpandedSessionIDs == nil {
			view.ExpandedSessionIDs = map[string]bool{}
		}
		view.ExpandedSessionIDs[id] = true
	})
	writeGeneration(w, generation)
}
