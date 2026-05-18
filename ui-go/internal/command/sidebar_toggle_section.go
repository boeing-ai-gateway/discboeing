package command

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarToggleSection toggles the Recent or All sessions sidebar section.
func (h *Handler) SidebarToggleSection(w http.ResponseWriter, r *http.Request) {
	section := strings.TrimSpace(r.URL.Query().Get("section"))
	if section == "" {
		http.Error(w, "missing section", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		switch section {
		case "recent":
			view.Sidebar.RecentOpen = !view.Sidebar.RecentOpen
		case "all":
			view.Sidebar.AllOpen = !view.Sidebar.AllOpen
		default:
			return fmt.Errorf("unknown sidebar section %q", section)
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to toggle sidebar section", "section", section, "error", err)
		http.Error(w, "failed to toggle sidebar section", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
