package app

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func workspaceSelectorClass(fullWidth bool) string {
	if fullWidth {
		return "flex items-center gap-1.5 flex-1"
	}
	return "flex items-center gap-1.5"
}

func workspaceInputWidthClass(fullWidth bool) string {
	if fullWidth {
		return "w-full"
	}
	return "w-[320px]"
}

func workspaceSourcePlaceholder(sourceType string) string {
	if sourceType == "local" {
		return "~/projects/my-app"
	}
	return "https://github.com/org/repo or org/repo"
}

func workspaceOptionValue(workspace viewmodel.WorkspaceOption) string {
	return "existing:" + workspace.ID
}

func workspaceIconSource(snapshot viewmodel.ConversationWorkspaceSelectorSnapshot) string {
	if snapshot.RequiresInput {
		if snapshot.SourceType == "local" {
			return "local"
		}
		return "git"
	}
	switch snapshot.SelectedOption {
	case "local-directory":
		return "local"
	case "git-repo":
		return "git"
	default:
		for _, workspace := range snapshot.Workspaces {
			if workspaceOptionValue(workspace) == snapshot.SelectedOption {
				return workspace.SourceType
			}
		}
		return "managed"
	}
}

func workspaceSuggestionClass(selected bool, valid bool) string {
	base := "flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-xs hover:bg-accent"
	if selected {
		base += " bg-accent"
	}
	if !valid {
		base += " opacity-70"
	}
	return base
}

func workspaceSuggestionSelected(snapshot viewmodel.ConversationWorkspaceSelectorSnapshot, index int) bool {
	return snapshot.HasSuggestionSelection && snapshot.SelectedSuggestionIndex == index
}
