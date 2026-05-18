package command

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarWorkspaceMenu opens a workspace action menu in the session view.
func (h *Handler) SidebarWorkspaceMenu(w http.ResponseWriter, r *http.Request) {
	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))
	if workspaceID == "" {
		http.Error(w, "missing workspace_id", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		group, ok := sidebarWorkspaceByID(*view, workspaceID)
		if !ok {
			return fmt.Errorf("workspace %q not found", workspaceID)
		}
		view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{
			Kind:        "workspace",
			WorkspaceID: workspaceID,
			Title:       group.Label,
			CanDelete:   true,
		}
		view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{}
		view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{}
		return nil
	}); err != nil {
		h.logger.Warn("failed to open sidebar workspace menu", "workspaceID", workspaceID, "error", err)
		http.Error(w, "failed to open workspace menu", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SidebarWorkspaceAction handles actions selected from a workspace menu.
func (h *Handler) SidebarWorkspaceAction(w http.ResponseWriter, r *http.Request) {
	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspace_id"))
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if workspaceID == "" || action == "" {
		http.Error(w, "missing workspace_id or action", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		group, ok := sidebarWorkspaceByID(*view, workspaceID)
		if !ok {
			return fmt.Errorf("workspace %q not found", workspaceID)
		}
		switch action {
		case "rename":
			view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{}
			view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{}
			view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{Open: true, Kind: "workspace", WorkspaceID: workspaceID, Title: group.Label, Value: group.Label}
		case "delete":
			view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{}
			view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{}
			view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{Open: true, Kind: "workspace", WorkspaceID: workspaceID, Title: group.Label}
		default:
			return fmt.Errorf("unknown workspace action %q", action)
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to run sidebar workspace action", "workspaceID", workspaceID, "action", action, "error", err)
		http.Error(w, "failed to run workspace action", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
