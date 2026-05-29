package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionSetViewMode switches an expanded session row between files and diff.
func (h *Handler) SessionSetViewMode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mode := state.SessionViewMode(chi.URLParam(r, "mode"))
	if mode != state.SessionViewModeFiles && mode != state.SessionViewModeDiff {
		http.Error(w, "invalid session view mode", http.StatusBadRequest)
		return
	}

	h.view.SaveView(func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		if sessionPanel.SessionViewModes == nil {
			sessionPanel.SessionViewModes = map[string]state.SessionViewMode{}
		}
		sessionPanel.SessionViewModes[id] = mode
	})
	writeNoContent(w)
}
