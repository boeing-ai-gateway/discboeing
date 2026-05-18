package command

import (
	"net/http"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarSelectThread selects the requested thread in the left sidebar.
func (h *Handler) SidebarSelectThread(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	threadID := strings.TrimSpace(r.URL.Query().Get("thread_id"))
	if sessionID == "" || threadID == "" {
		http.Error(w, "missing session_id or thread_id", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		if err := h.rebuildSidebarView(r.Context(), view, sessionID, threadID); err != nil {
			return err
		}
		view.Sidebar.FloatingOpen = false
		return nil
	}); err != nil {
		h.logger.Warn("failed to select sidebar thread", "sessionID", sessionID, "threadID", threadID, "error", err)
		http.Error(w, "failed to select thread", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
