package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionToggleExpanded toggles a session row between compact and expanded mode.
func (h *Handler) SessionToggleExpanded(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	generation := h.view.SaveView(func(view *state.View) {
		if view.ExpandedSessionIDs == nil {
			view.ExpandedSessionIDs = map[string]bool{}
		}
		view.ExpandedSessionIDs[id] = !view.ExpandedSessionIDs[id]
	})
	writeGeneration(w, generation)
}
