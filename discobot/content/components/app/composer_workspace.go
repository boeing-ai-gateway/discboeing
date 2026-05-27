package app

func composerWorkspaceClass(sidePane bool) string {
	class := "ide-panel composer-workspace"
	if sidePane {
		class += " composer-workspace--side-pane"
	}
	return class
}
