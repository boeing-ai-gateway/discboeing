package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionSideChatSelect selects a compact side-chat thread in an expanded session row.
func (h *Handler) SessionSideChatSelect(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	threadID := chi.URLParam(r, "threadID")
	h.view.SaveShell(func(data *state.Data, view *state.View) {
		if !sessionSideChatExists(data.Sessions, sessionID, threadID) {
			return
		}

		sessionPanel := state.EnsureSessionPanelState(view)
		sessionPanel.SelectedSessionID = sessionID
		sessionPanel.SelectedSideChatID = threadID
		if sessionPanel.ExpandedSessionIDs == nil {
			sessionPanel.ExpandedSessionIDs = map[string]bool{}
		}
		sessionPanel.ExpandedSessionIDs[sessionID] = true
	})
	writeNoContent(w)
}

func sessionSideChatExists(sessions []state.Session, sessionID string, threadID string) bool {
	if sessionID == "" || threadID == "" {
		return false
	}
	for _, session := range sessions {
		if session.ID != sessionID {
			continue
		}
		for _, thread := range session.SideChats {
			if thread.ID == threadID {
				return true
			}
		}
		return false
	}
	return false
}
