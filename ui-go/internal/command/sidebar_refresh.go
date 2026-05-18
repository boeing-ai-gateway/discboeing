package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarRefresh rebuilds the sidebar view from the client read side.
func (h *Handler) SidebarRefresh(w http.ResponseWriter, r *http.Request) {
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		return h.rebuildSidebarView(r.Context(), view, "", "")
	}); err != nil {
		h.logger.Warn("failed to refresh sidebar", "error", err)
		http.Error(w, "failed to refresh sidebar", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
