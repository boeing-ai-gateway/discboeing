package command

import (
	"net/http"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// TerminalToggle toggles the session workspace terminal panel.
func (h *Handler) TerminalToggle(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(func(view *state.View) {
		panel := state.EnsurePanel(view, "terminal")
		if panel.Visible && !state.CanHideWorkspacePanel(*view, "terminal") {
			return
		}
		panel.Visible = !panel.Visible
		if !panel.Visible {
			panel.Maximized = false
		}
		state.SavePanel(view, "terminal", panel)
	})
	writeNoContent(w)
}
