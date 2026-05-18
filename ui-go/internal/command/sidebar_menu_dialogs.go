package command

import (
	"fmt"
	"net/http"
	"strings"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SidebarCloseMenu clears any server-owned sidebar menu/dialog state.
func (h *Handler) SidebarCloseMenu(w http.ResponseWriter, r *http.Request) {
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		clearSidebarDialogs(view)
		return nil
	}); err != nil {
		h.logger.Warn("failed to close sidebar menu", "error", err)
		http.Error(w, "failed to close sidebar menu", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SidebarRename applies the active sidebar rename dialog.
func (h *Handler) SidebarRename(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		target, err := renameDialogTarget(*view)
		if err != nil {
			return err
		}
		if name == "" && target.Kind != "workspace" {
			return fmt.Errorf("missing name")
		}
		projectID, _, err := h.defaultProjectWorkspace(r)
		if err != nil {
			return err
		}
		switch target.Kind {
		case "session":
			if _, err := h.client.Sessions.Update(r.Context(), projectID, target.SessionID, api.UpdateSessionRequest{Name: &name}); err != nil {
				return err
			}
			clearSidebarDialogs(view)
			return h.rebuildSidebarView(r.Context(), view, target.SessionID, "")
		case "thread":
			if _, err := h.client.Sessions.UpdateThread(r.Context(), projectID, target.SessionID, target.ThreadID, api.UpdateThreadRequest{Name: name}); err != nil {
				return err
			}
			clearSidebarDialogs(view)
			return h.rebuildSidebarView(r.Context(), view, target.SessionID, target.ThreadID)
		case "workspace":
			var displayName *string
			if name != "" {
				displayName = &name
			}
			if _, err := h.client.Workspaces.Update(r.Context(), projectID, target.WorkspaceID, api.UpdateWorkspaceRequest{DisplayName: displayName}); err != nil {
				return err
			}
			clearSidebarDialogs(view)
			return h.rebuildSidebarView(r.Context(), view, "", "")
		default:
			return fmt.Errorf("unknown rename kind %q", target.Kind)
		}
	}); err != nil {
		h.logger.Warn("failed to rename sidebar item", "error", err)
		http.Error(w, "failed to rename", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SidebarDelete confirms the active sidebar delete dialog.
func (h *Handler) SidebarDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.saveSidebarView(r, func(view *viewmodel.ShellSnapshot) error {
		target, err := deleteDialogTarget(*view)
		if err != nil {
			return err
		}
		projectID, _, err := h.defaultProjectWorkspace(r)
		if err != nil {
			return err
		}
		switch target.Kind {
		case "session":
			if err := h.client.Sessions.Delete(r.Context(), projectID, target.SessionID); err != nil {
				return err
			}
			nextSessionID := selectedSessionAfterDelete(*view, target.SessionID)
			clearSidebarDialogs(view)
			return h.rebuildSidebarView(r.Context(), view, nextSessionID, "")
		case "thread":
			if err := h.client.Sessions.DeleteThread(r.Context(), projectID, target.SessionID, target.ThreadID); err != nil {
				return err
			}
			clearSidebarDialogs(view)
			return h.rebuildSidebarView(r.Context(), view, target.SessionID, "")
		case "workspace":
			if err := h.client.Workspaces.Delete(r.Context(), projectID, target.WorkspaceID); err != nil {
				return err
			}
			nextSessionID := selectedSessionAfterWorkspaceDelete(*view, target.WorkspaceID)
			clearSidebarDialogs(view)
			return h.rebuildSidebarView(r.Context(), view, nextSessionID, "")
		default:
			return fmt.Errorf("unknown delete kind %q", target.Kind)
		}
	}); err != nil {
		h.logger.Warn("failed to delete sidebar item", "error", err)
		http.Error(w, "failed to delete", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
