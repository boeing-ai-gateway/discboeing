package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarToggleGrouping toggles workspace grouping in the left sidebar.
func (h *Handler) SidebarToggleGrouping(w http.ResponseWriter, r *http.Request) {
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		view.Sidebar.GroupedByWorkspace = !view.Sidebar.GroupedByWorkspace
		return h.rebuildSidebarView(r.Context(), view, "", "")
	}); err != nil {
		h.logger.Warn("failed to toggle sidebar grouping", "error", err)
		http.Error(w, "failed to toggle sidebar grouping", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
