package command

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarSessionMenu opens a session action menu in the session view.
func (h *Handler) SidebarSessionMenu(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(w, "missing session_id", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		item, ok := sidebarSessionByID(*view, sessionID)
		if !ok {
			return fmt.Errorf("session %q not found", sessionID)
		}
		view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{
			Kind:      "session",
			SessionID: sessionID,
			Title:     sidebarItemName(item.DisplayName, item.Name, "New Session"),
			CanStop:   item.Status != "stopped",
			CanDelete: true,
		}
		view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{}
		view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{}
		return nil
	}); err != nil {
		h.logger.Warn("failed to open sidebar session menu", "sessionID", sessionID, "error", err)
		http.Error(w, "failed to open session menu", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SidebarSessionAction handles actions selected from a session menu.
func (h *Handler) SidebarSessionAction(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if sessionID == "" || action == "" {
		http.Error(w, "missing session_id or action", http.StatusBadRequest)
		return
	}
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		return h.applySessionAction(r, view, sessionID, action)
	}); err != nil {
		h.logger.Warn("failed to run sidebar session action", "sessionID", sessionID, "action", action, "error", err)
		http.Error(w, "failed to run session action", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) applySessionAction(r *http.Request, view *viewmodel.ShellSnapshot, sessionID string, action string) error {
	item, ok := sidebarSessionByID(*view, sessionID)
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}
	title := sidebarItemName(item.DisplayName, item.Name, "New Session")
	projectID, _, err := h.defaultProjectWorkspace(r)
	if err != nil {
		return err
	}
	switch action {
	case "new-thread":
		threadID, err := newThreadID()
		if err != nil {
			return err
		}
		thread, err := h.client.Sessions.CreateThread(r.Context(), projectID, sessionID, api.CreateThreadRequest{ID: threadID})
		if err != nil {
			return err
		}
		clearSidebarDialogs(view)
		return h.rebuildSidebarView(r.Context(), view, sessionID, thread.ID)
	case "rename":
		view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{}
		view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{}
		view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{Open: true, Kind: "session", SessionID: sessionID, Title: title, Value: title}
	case "stop":
		if err := h.client.Sessions.Stop(r.Context(), projectID, sessionID); err != nil {
			return err
		}
		clearSidebarDialogs(view)
		return h.rebuildSidebarView(r.Context(), view, sessionID, "")
	case "delete":
		view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{}
		view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{}
		view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{Open: true, Kind: "session", SessionID: sessionID, Title: title}
	default:
		return fmt.Errorf("unknown session action %q", action)
	}
	return nil
}

func newThreadID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "thread-" + hex.EncodeToString(b[:]), nil
}
