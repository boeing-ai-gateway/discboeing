package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionDetailSectionShow adds a VS Code-style detail panel above the dock.
func (h *Handler) SessionDetailSectionShow(w http.ResponseWriter, r *http.Request) {
	h.setSessionDetailSectionVisible(w, r, true)
}

// SessionDetailSectionHide removes a VS Code-style detail panel from the stack.
func (h *Handler) SessionDetailSectionHide(w http.ResponseWriter, r *http.Request) {
	h.setSessionDetailSectionVisible(w, r, false)
}

func (h *Handler) setSessionDetailSectionVisible(w http.ResponseWriter, r *http.Request, visible bool) {
	sessionID := chi.URLParam(r, "id")
	section := state.SessionDetailSection(chi.URLParam(r, "section"))
	if !state.IsSessionDetailSection(section) {
		http.Error(w, "invalid session detail section", http.StatusBadRequest)
		return
	}

	h.view.SaveView(r.Context(), func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		key := state.SessionDetailSectionKey(sessionID, section)
		if visible {
			if sessionPanel.VisibleSessionDetailSections == nil {
				sessionPanel.VisibleSessionDetailSections = map[string]bool{}
			}
			sessionPanel.VisibleSessionDetailSections[key] = true
			if sessionPanel.ExpandedSessionIDs == nil {
				sessionPanel.ExpandedSessionIDs = map[string]bool{}
			}
			sessionPanel.ExpandedSessionIDs[sessionID] = true
			return
		}

		if sessionPanel.VisibleSessionDetailSections != nil {
			delete(sessionPanel.VisibleSessionDetailSections, key)
		}
	})
	writeNoContent(w)
}
