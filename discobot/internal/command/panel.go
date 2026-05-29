package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// PanelToggle toggles a server-owned workspace panel's visibility.
func (h *Handler) PanelToggle(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "id")
	if _, ok := state.DefaultPanelLayout().Panels[panelID]; !ok {
		http.Error(w, "invalid panel", http.StatusBadRequest)
		return
	}

	h.view.SaveView(func(view *state.View) {
		panel := state.EnsurePanel(view, panelID)
		if panel.Visible && !state.CanHideWorkspacePanel(*view, panelID) {
			return
		}
		panel.Visible = !panel.Visible
		if !panel.Visible {
			panel.Maximized = false
		}
		view.PanelLayout.Panels[panelID] = panel
	})
	writeNoContent(w)
}

// PanelMaximize makes one visible workspace panel occupy the layout.
func (h *Handler) PanelMaximize(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "id")
	defaultPanel, ok := state.DefaultPanelLayout().Panels[panelID]
	if !ok {
		http.Error(w, "invalid panel", http.StatusBadRequest)
		return
	}
	if !defaultPanel.Maximizable {
		http.Error(w, "panel is not maximizable", http.StatusBadRequest)
		return
	}

	h.view.SaveView(func(view *state.View) {
		for id := range state.DefaultPanelLayout().Panels {
			panel := state.EnsurePanel(view, id)
			panel.Maximized = id == panelID
			if id == panelID {
				panel.Visible = true
			}
			view.PanelLayout.Panels[id] = panel
		}
	})
	writeNoContent(w)
}

// PanelRestore clears the maximized panel state.
func (h *Handler) PanelRestore(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "id")
	if _, ok := state.DefaultPanelLayout().Panels[panelID]; !ok {
		http.Error(w, "invalid panel", http.StatusBadRequest)
		return
	}

	h.view.SaveView(func(view *state.View) {
		for id := range state.DefaultPanelLayout().Panels {
			panel := state.EnsurePanel(view, id)
			panel.Maximized = false
			view.PanelLayout.Panels[id] = panel
		}
	})
	writeNoContent(w)
}
