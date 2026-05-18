package command

import (
	"net/http"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarSelectSession selects the requested session in the left sidebar.
func (h *Handler) SidebarSelectSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(w, "missing session_id", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		if err := h.rebuildSidebarView(r.Context(), view, sessionID, ""); err != nil {
			return err
		}
		view.Sidebar.FloatingOpen = false
		return nil
	}); err != nil {
		h.logger.Warn("failed to select sidebar session", "sessionID", sessionID, "error", err)
		http.Error(w, "failed to select session", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
