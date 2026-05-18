package app

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

type toolbarCommandGroup struct {
	Label    string
	Commands []viewmodel.ToolbarCommand
}

func toolbarButtonClass(active bool) string {
	base := "inline-flex h-7 items-center justify-center rounded px-2 text-xs font-medium transition-colors"
	if active {
		return base + " bg-secondary text-secondary-foreground"
	}
	return base + " text-muted-foreground hover:bg-accent hover:text-accent-foreground"
}

func toolbarCommandLabel(command viewmodel.ToolbarCommand) string {
	if command.Label != "" {
		return command.Label
	}
	if command.Name != "" {
		return command.Name
	}
	return "Run"
}

func toolbarOperationLabel(snapshot viewmodel.SessionToolbarSnapshot) string {
	if snapshot.Pending {
		return "Pending..."
	}
	if snapshot.Busy {
		if snapshot.ActiveCommand != "" {
			return snapshot.ActiveCommand + "..."
		}
		if snapshot.PrimaryCommand.ActiveLabel != "" {
			return snapshot.PrimaryCommand.ActiveLabel
		}
		return toolbarCommandLabel(snapshot.PrimaryCommand) + "..."
	}
	return toolbarCommandLabel(snapshot.PrimaryCommand)
}

func toolbarHasPrimaryCommand(snapshot viewmodel.SessionToolbarSnapshot) bool {
	return snapshot.PrimaryCommand.Name != "" || snapshot.PrimaryCommand.Label != ""
}

func toolbarPreferredIDE(snapshot viewmodel.SessionToolbarSnapshot) string {
	if snapshot.PreferredIDE != "" {
		return snapshot.PreferredIDE
	}
	if len(snapshot.IDEOptions) > 0 && snapshot.IDEOptions[0].Label != "" {
		return snapshot.IDEOptions[0].Label
	}
	return "Cursor"
}

func toolbarDiffLabel(snapshot viewmodel.SessionToolbarSnapshot) string {
	return strconv.Itoa(snapshot.DiffStats.FilesChanged) + " files changed, " +
		strconv.Itoa(snapshot.DiffStats.Additions) + " additions, " +
		strconv.Itoa(snapshot.DiffStats.Deletions) + " deletions"
}

func toolbarSecondaryCommandGroups(snapshot viewmodel.SessionToolbarSnapshot) []toolbarCommandGroup {
	groups := []toolbarCommandGroup{}
	for _, command := range snapshot.SecondaryCommands {
		label := command.Group
		found := false
		for index := range groups {
			if groups[index].Label == label {
				groups[index].Commands = append(groups[index].Commands, command)
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, toolbarCommandGroup{
				Label:    label,
				Commands: []viewmodel.ToolbarCommand{command},
			})
		}
	}
	return groups
}
