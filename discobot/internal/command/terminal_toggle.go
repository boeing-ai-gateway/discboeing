package command

import (
	"net/http"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// TerminalToggle toggles the session workspace terminal panel.
func (h *Handler) TerminalToggle(w http.ResponseWriter, r *http.Request) {
	generation := h.view.SaveView(func(view *state.View) {
		view.TerminalPanelVisible = !view.TerminalPanelVisible
	})
	writeGeneration(w, generation)
}
