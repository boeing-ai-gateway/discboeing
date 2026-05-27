package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// FileToggleExpanded toggles a directory node in a server-rendered file tree.
func (h *Handler) FileToggleExpanded(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	generation := h.view.SaveView(func(view *state.View) {
		if view.ExpandedFileIDs == nil {
			view.ExpandedFileIDs = map[string]bool{}
		}
		view.ExpandedFileIDs[id] = !view.ExpandedFileIDs[id]
	})
	writeGeneration(w, generation)
}
