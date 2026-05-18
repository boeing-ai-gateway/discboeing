package parts

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func projectInspectionTitle(snapshot viewmodel.ProjectInspectionTerminalSnapshot) string {
	if snapshot.Title != "" {
		return snapshot.Title
	}
	return "Inspection shell"
}

func projectInspectionDescription(snapshot viewmodel.ProjectInspectionTerminalSnapshot) string {
	if snapshot.Description != "" {
		return snapshot.Description
	}
	return "Open a troubleshooting shell in the inspection container."
}

func projectInspectionStatus(snapshot viewmodel.ProjectInspectionTerminalSnapshot) string {
	switch snapshot.ConnectionStatus {
	case "connecting", "connected", "error", "disconnected":
		return snapshot.ConnectionStatus
	default:
		return "disconnected"
	}
}

func projectInspectionStatusClass(status string) string {
	switch status {
	case "connected":
		return "bg-green-500"
	case "connecting":
		return "bg-yellow-500"
	case "error":
		return "bg-red-500"
	default:
		return "bg-muted-foreground/50"
	}
}

func projectInspectionOverlayMessage(status string) string {
	switch status {
	case "connecting":
		return "Connecting…"
	case "error":
		return "Connection error — retry to reconnect to the inspection shell."
	case "disconnected":
		return "Disconnected"
	default:
		return ""
	}
}
