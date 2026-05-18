package app

import (
	"slices"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func dockPanelMounted(snapshot viewmodel.DockPanelSnapshot, kind string) bool {
	return snapshot.ActiveKind == kind || slices.Contains(snapshot.MountedKinds, kind)
}

func dockPanelVisibilityClass(snapshot viewmodel.DockPanelSnapshot, kind string) string {
	if snapshot.ActiveKind == kind {
		return "contents"
	}
	return "hidden"
}

func dockPanelTitle(kind string) string {
	switch kind {
	case "terminal":
		return "Terminal"
	case "desktop":
		return "Desktop"
	case "vscode":
		return "Editor"
	case "file":
		return "Files"
	case "diff-review":
		return "Diff review"
	case "services":
		return "Services"
	default:
		return "Dock panel"
	}
}

func dockPanelDescription(snapshot viewmodel.DockPanelSnapshot, kind string) string {
	switch kind {
	case "terminal":
		return "Terminal session for " + snapshot.SessionID
	case "desktop":
		if snapshot.DesktopAvailable {
			return "Remote desktop service is available."
		}
		return "Remote desktop service is not available yet."
	case "vscode":
		if snapshot.EditorEnabled && snapshot.VSCodeAvailable {
			return "VS Code service is available."
		}
		return "Editor service is disabled or unavailable."
	case "file":
		return "Browse and preview workspace files."
	case "diff-review":
		return "Review changed files and comment on selected diff excerpts."
	case "services":
		return "Manage workspace services exposed by the session."
	default:
		return "Panel content will appear here when selected."
	}
}
