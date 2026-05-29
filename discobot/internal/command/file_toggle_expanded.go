package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// FileToggleExpanded toggles a directory node in a server-rendered file tree.
func (h *Handler) FileToggleExpanded(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveView(func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		if sessionPanel.ExpandedFileIDs == nil {
			sessionPanel.ExpandedFileIDs = map[string]bool{}
		}
		sessionPanel.ExpandedFileIDs[id] = !sessionPanel.ExpandedFileIDs[id]
	})
	writeNoContent(w)
}
