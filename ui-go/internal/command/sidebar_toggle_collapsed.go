package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarToggleCollapsed toggles the left sessions sidebar between expanded and
// collapsed layout modes.
func (h *Handler) SidebarToggleCollapsed(w http.ResponseWriter, r *http.Request) {
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		view.Sidebar.Collapsed = !view.Sidebar.Collapsed
		if !view.Sidebar.Collapsed {
			view.Sidebar.FloatingOpen = false
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to toggle sidebar collapsed state", "error", err)
		http.Error(w, "failed to toggle sidebar", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
