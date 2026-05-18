package command

import (
	"fmt"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func sidebarItemName(displayName string, name string, fallback string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	if strings.TrimSpace(name) != "" {
		return name
	}
	return fallback
}

func clearSidebarDialogs(view *viewmodel.ShellSnapshot) {
	view.Sidebar.OpenMenu = viewmodel.SidebarMenuSnapshot{}
	view.Sidebar.RenameDialog = viewmodel.SidebarRenameDialogSnapshot{}
	view.Sidebar.DeleteDialog = viewmodel.SidebarDeleteDialogSnapshot{}
}

func sidebarSessionByID(view viewmodel.ShellSnapshot, sessionID string) (viewmodel.SidebarSessionItem, bool) {
	for _, group := range view.Sidebar.SessionGroups {
		for _, session := range group.Sessions {
			if session.ID == sessionID {
				return session, true
			}
		}
	}
	return viewmodel.SidebarSessionItem{}, false
}

func sidebarWorkspaceByID(view viewmodel.ShellSnapshot, workspaceID string) (viewmodel.SidebarSessionGroup, bool) {
	for _, group := range view.Sidebar.SessionGroups {
		if group.WorkspaceID == workspaceID {
			return group, true
		}
	}
	return viewmodel.SidebarSessionGroup{}, false
}

func sidebarThreadByID(view viewmodel.ShellSnapshot, sessionID string, threadID string) (viewmodel.SidebarThreadItem, bool) {
	for _, group := range view.Sidebar.SessionGroups {
		for _, session := range group.Sessions {
			if session.ID != sessionID {
				continue
			}
			for _, thread := range session.Threads {
				if found, ok := sidebarThreadInTree(thread, threadID); ok {
					return found, true
				}
			}
		}
	}
	for _, thread := range view.Sidebar.RecentThreads {
		if thread.SessionID == sessionID {
			if found, ok := sidebarThreadInTree(thread, threadID); ok {
				return found, true
			}
		}
	}
	return viewmodel.SidebarThreadItem{}, false
}

func sidebarThreadInTree(thread viewmodel.SidebarThreadItem, threadID string) (viewmodel.SidebarThreadItem, bool) {
	if thread.ID == threadID {
		return thread, true
	}
	for _, child := range thread.Children {
		if found, ok := sidebarThreadInTree(child, threadID); ok {
			return found, true
		}
	}
	return viewmodel.SidebarThreadItem{}, false
}

func selectedSessionAfterDelete(view viewmodel.ShellSnapshot, deletedSessionID string) string {
	for _, group := range view.Sidebar.SessionGroups {
		for _, session := range group.Sessions {
			if session.ID != deletedSessionID {
				return session.ID
			}
		}
	}
	return ""
}

func selectedSessionAfterWorkspaceDelete(view viewmodel.ShellSnapshot, deletedWorkspaceID string) string {
	for _, group := range view.Sidebar.SessionGroups {
		if group.WorkspaceID == deletedWorkspaceID {
			continue
		}
		for _, session := range group.Sessions {
			return session.ID
		}
	}
	return ""
}

func renameDialogTarget(view viewmodel.ShellSnapshot) (viewmodel.SidebarRenameDialogSnapshot, error) {
	if !view.Sidebar.RenameDialog.Open {
		return viewmodel.SidebarRenameDialogSnapshot{}, fmt.Errorf("rename dialog is not open")
	}
	return view.Sidebar.RenameDialog, nil
}

func deleteDialogTarget(view viewmodel.ShellSnapshot) (viewmodel.SidebarDeleteDialogSnapshot, error) {
	if !view.Sidebar.DeleteDialog.Open {
		return viewmodel.SidebarDeleteDialogSnapshot{}, fmt.Errorf("delete dialog is not open")
	}
	return view.Sidebar.DeleteDialog, nil
}
