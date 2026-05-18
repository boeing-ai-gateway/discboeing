package parts

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

const terminalPanelWorkspacePath = "/home/discobot/workspace"

func boolStringParts(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func terminalPanelSwitchClass(enabled bool, disabled bool) string {
	base := "relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-transparent transition-colors"
	if enabled {
		base += " bg-primary"
	} else {
		base += " bg-input"
	}
	if disabled {
		base += " opacity-50"
	}
	return base
}

func terminalPanelSwitchThumbClass(enabled bool) string {
	base := "pointer-events-none block size-4 rounded-full bg-background shadow-sm transition-transform"
	if enabled {
		return base + " translate-x-4"
	}
	return base + " translate-x-0.5"
}

func terminalPanelSessionID(snapshot viewmodel.DockPanelSnapshot) string {
	if snapshot.Terminal.SessionID != "" {
		return snapshot.Terminal.SessionID
	}
	return snapshot.SessionID
}

func terminalPanelSessionLabel(snapshot viewmodel.DockPanelSnapshot) string {
	if sessionID := terminalPanelSessionID(snapshot); sessionID != "" {
		return "Session: " + sessionID
	}
	return "No session"
}

func terminalPanelConnectionStatus(snapshot viewmodel.DockPanelSnapshot) string {
	switch snapshot.Terminal.ConnectionStatus {
	case "connecting", "connected", "error", "disconnected":
		return snapshot.Terminal.ConnectionStatus
	default:
		return "disconnected"
	}
}

func terminalPanelStatusDotClass(snapshot viewmodel.DockPanelSnapshot) string {
	base := "size-2 shrink-0 rounded-full"
	switch terminalPanelConnectionStatus(snapshot) {
	case "connected":
		return base + " bg-green-500"
	case "connecting":
		return base + " bg-yellow-500"
	case "error":
		return base + " bg-red-500"
	default:
		return base + " bg-muted-foreground/50"
	}
}

func terminalPanelOverlayMessage(snapshot viewmodel.DockPanelSnapshot) string {
	if terminalPanelSessionID(snapshot) == "" {
		return "No session selected."
	}
	switch terminalPanelConnectionStatus(snapshot) {
	case "connecting":
		return "Connecting…"
	case "error":
		return "Connection error — reopen the terminal panel to retry."
	case "disconnected":
		return "Disconnected"
	default:
		return ""
	}
}

func terminalPanelMaximizeTitle(snapshot viewmodel.DockPanelSnapshot) string {
	if snapshot.DockMaximized {
		return "Restore split view"
	}
	return "Maximize terminal panel"
}

func terminalPanelSSHHost(snapshot viewmodel.DockPanelSnapshot) string {
	if snapshot.Terminal.SSHHost != "" {
		return snapshot.Terminal.SSHHost
	}
	return "localhost"
}

func terminalPanelSSHPort(snapshot viewmodel.DockPanelSnapshot) int {
	if snapshot.Terminal.SSHPort > 0 {
		return snapshot.Terminal.SSHPort
	}
	return 2222
}

func terminalPanelSSHCommand(snapshot viewmodel.DockPanelSnapshot) string {
	sessionID := terminalPanelSessionID(snapshot)
	if sessionID == "" {
		return ""
	}
	return "ssh -p " + strconv.Itoa(terminalPanelSSHPort(snapshot)) + " " + sessionID + "@" + terminalPanelSSHHost(snapshot)
}

func terminalPanelPullCommand(snapshot viewmodel.DockPanelSnapshot) string {
	sessionID := terminalPanelSessionID(snapshot)
	if sessionID == "" {
		return ""
	}
	return "git pull \"ssh://" + sessionID + "@" + terminalPanelSSHHost(snapshot) + ":" + strconv.Itoa(terminalPanelSSHPort(snapshot)) + terminalPanelWorkspacePath + "\" HEAD"
}

func terminalPanelCopied(snapshot viewmodel.DockPanelSnapshot, command string) bool {
	return snapshot.Terminal.CopiedCommand == command
}
