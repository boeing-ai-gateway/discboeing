package parts

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func desktopConnectionStatus(snapshot viewmodel.DockPanelSnapshot) string {
	if !snapshot.DesktopAvailable {
		return "unavailable"
	}
	switch snapshot.Desktop.ConnectionStatus {
	case "connected", "disconnected", "unavailable":
		return snapshot.Desktop.ConnectionStatus
	default:
		return "connecting"
	}
}

func desktopStatusLabel(status string) string {
	switch status {
	case "connected":
		return "Connected"
	case "disconnected":
		return "Disconnected"
	case "unavailable":
		return "Unavailable"
	default:
		return "Connecting"
	}
}

func desktopStatusDotClass(status string) string {
	className := "size-2 shrink-0 rounded-full "
	switch status {
	case "connected":
		return className + "bg-green-500"
	case "disconnected":
		return className + "bg-red-500"
	case "unavailable":
		return className + "bg-sidebar-foreground/30"
	default:
		return className + "bg-yellow-500"
	}
}

func desktopOverlayMessage(snapshot viewmodel.DockPanelSnapshot, status string) string {
	if !snapshot.DesktopAvailable {
		return "Desktop is not available for this session."
	}
	if status == "disconnected" {
		return "Desktop disconnected"
	}
	if status == "connected" {
		return ""
	}

	return "Connecting to desktop..."
}

func desktopTitleLabel(snapshot viewmodel.DockPanelSnapshot, status string) string {
	if snapshot.Desktop.DesktopName != "" {
		return snapshot.Desktop.DesktopName
	}

	return desktopStatusLabel(status)
}

func desktopMaximizeTitle(snapshot viewmodel.DockPanelSnapshot) string {
	if snapshot.DockMaximized {
		return "Restore split view"
	}

	return "Maximize desktop panel"
}
