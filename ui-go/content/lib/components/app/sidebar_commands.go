package app

import "net/url"

func sidebarNewSessionCommand() string {
	return "@post('/ui/commands/sidebar/new-session')"
}

func sidebarToggleGroupingCommand() string {
	return "@post('/ui/commands/sidebar/toggle-grouping')"
}

func sidebarToggleCollapsedCommand() string {
	return "@post('/ui/commands/sidebar/toggle-collapsed')"
}

func sidebarToggleFloatingCommand() string {
	return "@post('/ui/commands/sidebar/toggle-floating')"
}

func sidebarSelectSessionCommand(sessionID string) string {
	return "@post('/ui/commands/sidebar/select-session?session_id=" + url.QueryEscape(sessionID) + "')"
}

func sidebarSelectThreadCommand(sessionID string, threadID string) string {
	return "@post('/ui/commands/sidebar/select-thread?session_id=" + url.QueryEscape(sessionID) + "&thread_id=" + url.QueryEscape(threadID) + "')"
}

func sidebarSessionMenuCommand(sessionID string) string {
	return "@post('/ui/commands/sidebar/session-menu?session_id=" + url.QueryEscape(sessionID) + "')"
}

func sidebarThreadMenuCommand(sessionID string, threadID string) string {
	return "@post('/ui/commands/sidebar/thread-menu?session_id=" + url.QueryEscape(sessionID) + "&thread_id=" + url.QueryEscape(threadID) + "')"
}

func sidebarWorkspaceMenuCommand(workspaceID string) string {
	return "@post('/ui/commands/sidebar/workspace-menu?workspace_id=" + url.QueryEscape(workspaceID) + "')"
}

func sidebarToggleSectionCommand(section string) string {
	return "@post('/ui/commands/sidebar/toggle-section?section=" + url.QueryEscape(section) + "')"
}

func sidebarCloseMenuCommand() string {
	return "@post('/ui/commands/sidebar/close-menu')"
}

func sidebarSessionActionCommand(sessionID string, action string) string {
	return "@post('/ui/commands/sidebar/session-action?session_id=" + url.QueryEscape(sessionID) + "&action=" + url.QueryEscape(action) + "')"
}

func sidebarThreadActionCommand(sessionID string, threadID string, action string) string {
	return "@post('/ui/commands/sidebar/thread-action?session_id=" + url.QueryEscape(sessionID) + "&thread_id=" + url.QueryEscape(threadID) + "&action=" + url.QueryEscape(action) + "')"
}

func sidebarWorkspaceActionCommand(workspaceID string, action string) string {
	return "@post('/ui/commands/sidebar/workspace-action?workspace_id=" + url.QueryEscape(workspaceID) + "&action=" + url.QueryEscape(action) + "')"
}

func sidebarRenameCommand() string {
	return "@post('/ui/commands/sidebar/rename')"
}

func sidebarDeleteCommand() string {
	return "@post('/ui/commands/sidebar/delete')"
}
