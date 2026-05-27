package app

import "github.com/obot-platform/discobot/discobot/internal/state"

func sessionsSidebarToggleLabel(view state.View) string {
	if view.SessionsSidebarVisible {
		return "Hide sessions sidebar"
	}
	return "Show sessions sidebar"
}

func terminalToggleLabel(view state.View) string {
	if view.TerminalPanelVisible {
		return "Hide terminal"
	}
	return "Show terminal"
}

func terminalToggleClass(view state.View) string {
	class := "window-chrome__icon-button"
	if view.TerminalPanelVisible {
		class += " window-chrome__icon-button--active"
	}
	return class
}
