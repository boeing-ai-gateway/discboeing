package app

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func sessionWorkspaceShellClass(visible bool) string {
	if visible {
		return "contents"
	}
	return "hidden"
}

func sessionWorkspaceMainClass(snapshot viewmodel.SessionWorkspaceSnapshot) string {
	if snapshot.MainClass != "" {
		return snapshot.MainClass
	}
	return "flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden pt-1"
}

func sessionWorkspaceShowDock(snapshot viewmodel.SessionWorkspaceSnapshot) bool {
	return !snapshot.IsPending && snapshot.Dock.ActiveKind != ""
}

func sessionWorkspaceMode(snapshot viewmodel.SessionWorkspaceSnapshot) string {
	if sessionWorkspaceShowDock(snapshot) && snapshot.Dock.DockMaximized {
		return "dock-maximized"
	}
	if sessionWorkspaceShowDock(snapshot) {
		return "split-dock"
	}
	return "chat"
}

func sessionWorkspaceSessionID(snapshot viewmodel.SessionWorkspaceSnapshot) string {
	if snapshot.Dock.SessionID != "" {
		return snapshot.Dock.SessionID
	}
	return ""
}
