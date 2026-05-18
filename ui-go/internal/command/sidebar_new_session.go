package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go/internal/live"
	"github.com/obot-platform/discobot/ui-go/internal/readmodel"
)

// SidebarNewSession returns the normal conversation workspace to its pending
// new-session state without creating a server session until the composer is
// submitted.
func (h *Handler) SidebarNewSession(w http.ResponseWriter, r *http.Request) {
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		if h.live != nil {
			scope := readmodel.LiveScopeFromView(*view)
			if err := h.live.EnsureLoaded(r.Context(), scope); err != nil {
				return err
			}
			next := readmodel.BuildPendingShellFromBackend(h.live.Snapshot(scope), view.Sidebar.GroupedByWorkspace)
			next.Sidebar.StreamEvents = view.Sidebar.StreamEvents
			next.Sidebar.Commands = view.Sidebar.Commands
			next.Sidebar.Collapsed = view.Sidebar.Collapsed
			next.Sidebar.RecentOpen = view.Sidebar.RecentOpen
			next.Sidebar.AllOpen = view.Sidebar.AllOpen
			next.Header.Settings = view.Header.Settings
			*view = next
			view.Sidebar.FloatingOpen = false
			return nil
		}
		next, err := readmodel.BuildPendingShellFromClient(r.Context(), h.client, view.Sidebar.GroupedByWorkspace)
		if err != nil {
			return err
		}
		next.Sidebar.StreamEvents = view.Sidebar.StreamEvents
		next.Sidebar.Commands = view.Sidebar.Commands
		next.Sidebar.Collapsed = view.Sidebar.Collapsed
		next.Sidebar.FloatingOpen = view.Sidebar.FloatingOpen
		next.Sidebar.RecentOpen = view.Sidebar.RecentOpen
		next.Sidebar.AllOpen = view.Sidebar.AllOpen
		next.Header.Settings = view.Header.Settings
		*view = next
		view.Sidebar.FloatingOpen = false
		return nil
	}); err != nil {
		h.logger.Warn("failed to open pending sidebar session", "error", err)
		http.Error(w, "failed to open new session", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) defaultProjectWorkspace(r *http.Request) (string, string, error) {
	projectID := live.DefaultProjectID
	workspaces, err := h.client.Workspaces.List(r.Context(), projectID)
	if err != nil {
		return "", "", err
	}
	if len(workspaces) == 0 {
		return projectID, "", nil
	}
	return projectID, workspaces[0].ID, nil
}
