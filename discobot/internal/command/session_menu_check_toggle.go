package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionMenuCheckToggle toggles a checkable item in the sessions menu.
func (h *Handler) SessionMenuCheckToggle(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	generation := h.view.SaveView(func(view *state.View) {
		if view.SessionMenuChecks == nil {
			view.SessionMenuChecks = map[string]bool{}
		}
		view.SessionMenuChecks[key] = !view.SessionMenuChecks[key]
	})
	writeGeneration(w, generation)
}
