package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionSelect selects a session row and ensures its details are expanded.
func (h *Handler) SessionSelect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveView(r.Context(), func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		sessionPanel.SelectedSessionID = id
		if sessionPanel.ExpandedSessionIDs == nil {
			sessionPanel.ExpandedSessionIDs = map[string]bool{}
		}
		sessionPanel.ExpandedSessionIDs[id] = true
	})
	writeNoContent(w)
}
