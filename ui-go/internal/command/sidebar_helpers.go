package command

import (
	"context"
	"fmt"
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go/internal/readmodel"
)

func (h *Handler) saveSidebarView(r *http.Request, update func(*viewmodel.ShellSnapshot) error) error {
	session, ok := h.session(r)
	if !ok {
		return fmt.Errorf("missing session")
	}
	var updateErr error
	session.Save(func(view *viewmodel.ShellSnapshot) {
		updateErr = update(view)
		if updateErr == nil {
			nextLabel(&view.Sidebar.Commands)
		}
	})
	return updateErr
}

func (h *Handler) rebuildSidebarView(ctx context.Context, view *viewmodel.ShellSnapshot, selectedSessionID string, selectedThreadID string) error {
	if h.live != nil {
		scope := readmodel.LiveScopeFromView(*view)
		if selectedSessionID != "" {
			scope.SessionID = selectedSessionID
		}
		if selectedThreadID != "" {
			scope.ThreadID = selectedThreadID
		}
		if err := h.live.Refresh(ctx, scope); err != nil {
			return err
		}
		next := readmodel.BuildShellFromBackend(*view, h.live.Snapshot(scope))
		if selectedSessionID != "" || selectedThreadID != "" {
			markSidebarSelection(&next, selectedSessionID, selectedThreadID)
		}
		*view = next
		return nil
	}
	grouped := view.Sidebar.GroupedByWorkspace
	pending := view.Workspace.IsPending && selectedSessionID == "" && selectedThreadID == ""
	if selectedSessionID == "" || selectedThreadID == "" {
		currentSessionID, currentThreadID := currentSidebarSelection(*view)
		if selectedSessionID == "" {
			selectedSessionID = currentSessionID
		}
		if selectedThreadID == "" {
			selectedThreadID = currentThreadID
		}
	}
	var (
		next viewmodel.ShellSnapshot
		err  error
	)
	if pending {
		next, err = readmodel.BuildPendingShellFromClient(ctx, h.client, grouped)
	} else {
		next, err = readmodel.BuildShellFromClient(ctx, h.client, selectedSessionID, selectedThreadID, grouped)
	}
	if err != nil {
		return err
	}
	next.Sidebar.StreamEvents = view.Sidebar.StreamEvents
	next.Sidebar.Commands = view.Sidebar.Commands
	next.Sidebar.Collapsed = view.Sidebar.Collapsed
	next.Sidebar.FloatingOpen = view.Sidebar.FloatingOpen
	next.Sidebar.RecentOpen = view.Sidebar.RecentOpen
	next.Sidebar.AllOpen = view.Sidebar.AllOpen
	next.Sidebar.OpenMenu = view.Sidebar.OpenMenu
	next.Sidebar.RenameDialog = view.Sidebar.RenameDialog
	next.Sidebar.DeleteDialog = view.Sidebar.DeleteDialog
	next.Header.Settings = view.Header.Settings
	*view = next
	return nil
}

func markSidebarSelection(view *viewmodel.ShellSnapshot, sessionID string, threadID string) {
	for groupIndex := range view.Sidebar.SessionGroups {
		for sessionIndex := range view.Sidebar.SessionGroups[groupIndex].Sessions {
			session := &view.Sidebar.SessionGroups[groupIndex].Sessions[sessionIndex]
			session.Selected = session.ID == sessionID
			markThreadSelection(session.Threads, sessionID, threadID)
		}
	}
	markThreadSelection(view.Sidebar.RecentThreads, sessionID, threadID)
}

func markThreadSelection(threads []viewmodel.SidebarThreadItem, sessionID string, threadID string) {
	for i := range threads {
		threads[i].Selected = threads[i].SessionID == sessionID && (threadID == "" || threads[i].ID == threadID)
		markThreadSelection(threads[i].Children, sessionID, threadID)
	}
}

func currentSidebarSelection(view viewmodel.ShellSnapshot) (string, string) {
	for _, group := range view.Sidebar.SessionGroups {
		for _, session := range group.Sessions {
			if session.Selected {
				for _, thread := range session.Threads {
					if selectedThreadID, ok := selectedThread(thread); ok {
						return session.ID, selectedThreadID
					}
				}
				return session.ID, ""
			}
		}
	}
	for _, thread := range view.Sidebar.RecentThreads {
		if thread.Selected {
			return thread.SessionID, thread.ID
		}
	}
	return "", ""
}

func selectedThread(thread viewmodel.SidebarThreadItem) (string, bool) {
	if thread.Selected {
		return thread.ID, true
	}
	for _, child := range thread.Children {
		if id, ok := selectedThread(child); ok {
			return id, true
		}
	}
	return "", false
}
