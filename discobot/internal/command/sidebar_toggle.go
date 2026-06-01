package command

import (
	"net/http"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SidebarToggle toggles the left sessions panel visibility.
func (h *Handler) SidebarToggle(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(func(view *state.View) {
		panel := state.EnsurePanel(view, "session")
		panel.Visible = !panel.Visible
		if !panel.Visible {
			panel.Maximized = false
		}
		state.SavePanel(view, "session", panel)
	})
	writeNoContent(w)
}

// SidebarHide hides the left sessions panel without changing session state.
func (h *Handler) SidebarHide(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(func(view *state.View) {
		panel := state.EnsurePanel(view, "session")
		panel.Visible = false
		panel.Maximized = false
		state.SavePanel(view, "session", panel)
	})
	writeNoContent(w)
}
