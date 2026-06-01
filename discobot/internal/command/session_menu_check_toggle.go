package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionMenuCheckToggle toggles a checkable item in the sessions menu.
func (h *Handler) SessionMenuCheckToggle(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	h.view.SaveView(r.Context(), func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		if sessionPanel.SessionMenuChecks == nil {
			sessionPanel.SessionMenuChecks = map[string]bool{}
		}
		sessionPanel.SessionMenuChecks[key] = !sessionPanel.SessionMenuChecks[key]
	})
	writeNoContent(w)
}
