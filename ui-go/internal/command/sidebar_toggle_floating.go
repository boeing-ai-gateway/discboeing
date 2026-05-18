package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarToggleFloating toggles the collapsed sidebar's floating body.
func (h *Handler) SidebarToggleFloating(w http.ResponseWriter, r *http.Request) {
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		if view.Sidebar.Collapsed {
			view.Sidebar.FloatingOpen = !view.Sidebar.FloatingOpen
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to toggle sidebar floating state", "error", err)
		http.Error(w, "failed to toggle sidebar floating state", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
