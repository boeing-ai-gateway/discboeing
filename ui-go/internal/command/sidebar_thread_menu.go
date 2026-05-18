package command

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarThreadMenu opens a thread action menu in the session view.
func (h *Handler) SidebarThreadMenu(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	threadID := strings.TrimSpace(r.URL.Query().Get("thread_id"))
	if sessionID == "" || threadID == "" {
		http.Error(w, "missing session_id or thread_id", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		item, ok := sidebarThreadByID(*view, sessionID, threadID)
		if !ok {
			return fmt.Errorf("thread %q not found", threadID)
		}
		view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{
			Kind:      "thread",
			SessionID: sessionID,
			ThreadID:  threadID,
			Title:     sidebarItemName(item.DisplayName, item.Name, "New Thread"),
			CanDelete: !item.Primary,
		}
		view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{}
		view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{}
		return nil
	}); err != nil {
		h.logger.Warn("failed to open sidebar thread menu", "sessionID", sessionID, "threadID", threadID, "error", err)
		http.Error(w, "failed to open thread menu", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SidebarThreadAction handles actions selected from a thread menu.
func (h *Handler) SidebarThreadAction(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	threadID := strings.TrimSpace(r.URL.Query().Get("thread_id"))
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if sessionID == "" || threadID == "" || action == "" {
		http.Error(w, "missing session_id, thread_id, or action", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		return h.applyThreadAction(view, sessionID, threadID, action)
	}); err != nil {
		h.logger.Warn("failed to run sidebar thread action", "sessionID", sessionID, "threadID", threadID, "action", action, "error", err)
		http.Error(w, "failed to run thread action", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) applyThreadAction(view *viewmodel.ShellSnapshot, sessionID string, threadID string, action string) error {
	item, ok := sidebarThreadByID(*view, sessionID, threadID)
	if !ok {
		return fmt.Errorf("thread %q not found", threadID)
	}
	title := sidebarItemName(item.DisplayName, item.Name, "New Thread")
	switch action {
	case "rename":
		view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{}
		view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{}
		view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{Open: true, Kind: "thread", SessionID: sessionID, ThreadID: threadID, Title: title, Value: title}
	case "delete":
		if item.Primary {
			return fmt.Errorf("primary thread %q cannot be deleted", threadID)
		}
		view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{}
		view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{}
		view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{Open: true, Kind: "thread", SessionID: sessionID, ThreadID: threadID, Title: title}
	default:
		return fmt.Errorf("unknown thread action %q", action)
	}
	return nil
}
