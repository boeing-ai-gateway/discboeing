package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionToggleExpanded toggles a session row between compact and expanded mode.
func (h *Handler) SessionToggleExpanded(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveView(func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		if sessionPanel.ExpandedSessionIDs == nil {
			sessionPanel.ExpandedSessionIDs = map[string]bool{}
		}
		sessionPanel.ExpandedSessionIDs[id] = !sessionPanel.ExpandedSessionIDs[id]
	})
	writeNoContent(w)
}
