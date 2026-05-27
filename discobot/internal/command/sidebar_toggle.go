package command

import (
	"net/http"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SidebarToggle toggles the left sessions sidebar visibility.
func (h *Handler) SidebarToggle(w http.ResponseWriter, r *http.Request) {
	generation := h.view.SaveView(func(view *state.View) {
		view.SessionsSidebarVisible = !view.SessionsSidebarVisible
	})
	writeGeneration(w, generation)
}
