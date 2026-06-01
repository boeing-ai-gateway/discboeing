package helpers

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/discobot/internal/state"
	serverapi "github.com/obot-platform/discobot/server/api"
)

type ComposerWorkspaceOption struct {
	Workspace   serverapi.Workspace
	Label       string
	Description string
	Selected    bool
}

func ComposerWorkspaceOptions(workspaces []serverapi.Workspace, selectedWorkspaceID string) []ComposerWorkspaceOption {
	selected := ComposerSelectedWorkspace(workspaces, selectedWorkspaceID)
	options := make([]ComposerWorkspaceOption, 0, len(workspaces))
	for _, workspace := range workspaces {
		options = append(options, ComposerWorkspaceOption{
			Workspace:   workspace,
			Label:       ComposerWorkspaceLabel(workspace),
			Description: ComposerWorkspaceDescription(workspace),
			Selected:    workspace.ID == selected.ID,
		})
	}
	return options
}

func ComposerSelectedWorkspace(workspaces []serverapi.Workspace, selectedWorkspaceID string) serverapi.Workspace {
	for _, workspace := range workspaces {
		if workspace.ID == selectedWorkspaceID {
			return workspace
		}
	}
	for _, workspace := range workspaces {
		if workspace.Status == "ready" {
			return workspace
		}
	}
	if len(workspaces) > 0 {
		return workspaces[0]
	}
	return serverapi.Workspace{}
}

func ComposerWorkspaceLabel(workspace serverapi.Workspace) string {
	if workspace.DisplayName != nil && strings.TrimSpace(*workspace.DisplayName) != "" {
		return strings.TrimSpace(*workspace.DisplayName)
	}
	if workspace.SourceType == "managed" {
		return "Unnamed Workspace"
	}
	if workspace.Path != "" {
		return filepath.Base(strings.TrimRight(workspace.Path, string(filepath.Separator)))
	}
	if workspace.ID != "" {
		return workspace.ID
	}
	return "Select workspace"
}

func ComposerWorkspaceDescription(workspace serverapi.Workspace) string {
	if workspace.Path != "" {
		return ComposerShortWorkspacePath(workspace.Path)
	}
	if workspace.SourceType != "" {
		return workspace.SourceType
	}
	return "Workspace"
}

func ComposerShortWorkspacePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	home := "~"
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return path
	}
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	if len(parts) <= 3 {
		return path
	}
	return "…" + string(filepath.Separator) + filepath.Join(parts[len(parts)-2:]...)
}

func ComposerWorkspaceSelectCommand(workspaceID string) string {
	return "@discobotCommand('/ui/commands/composer/workspaces/" + url.PathEscape(workspaceID) + "/select', {method: 'POST'})"
}

func ComposerWorkspaceSourceStartCommand(sourceType string) string {
	return "$composerWorkspaceSourceInput = ''; @discobotCommand('/ui/commands/composer/workspaces/source/" + url.PathEscape(sourceType) + "/start', {method: 'POST'})"
}

func ComposerWorkspaceSourceCancelCommand() string {
	return "$composerWorkspaceSourceInput = ''; @discobotCommand('/ui/commands/composer/workspaces/source/cancel', {method: 'POST'})"
}

func ComposerWorkspaceSourceInputCommand() string {
	return "@discobotCommand('/ui/commands/composer/workspaces/source/input', {method: 'POST', payload: {value: evt.target.value}})"
}

func ComposerWorkspaceSuggestionCommand(value string) string {
	return "$composerWorkspaceSourceInput = " + strconv.Quote(value) + "; @discobotCommand('/ui/commands/composer/workspaces/source/input', {method: 'POST', payload: {value: " + strconv.Quote(value) + "}})"
}

func ComposerWorkspaceSourceIcon(sourceType string) string {
	if sourceType == "git" {
		return "git-branch"
	}
	return "folder"
}

func ComposerWorkspaceSourceLabel(sourceType string) string {
	if sourceType == "git" {
		return "GitHub Repo"
	}
	return "Local Directory"
}

func ComposerWorkspaceSourcePlaceholder(sourceType string) string {
	if sourceType == "git" {
		return "https://github.com/org/repo or org/repo"
	}
	return "~/projects/my-app"
}

func ComposerWorkspaceSourceMessage(composerState state.ComposerPanelState) string {
	if composerState.WorkspaceSetupMessage != "" {
		return composerState.WorkspaceSetupMessage
	}
	if !composerState.WorkspaceValidationSet {
		return ""
	}
	validation := composerState.WorkspaceValidation
	if validation.Error != nil && *validation.Error != "" {
		return *validation.Error
	}
	switch validation.Classification {
	case "new":
		if composerState.WorkspaceSourceType == "git" {
			return "Repository looks valid and will be cloned."
		}
		return "Directory will be created."
	case "empty":
		return "Empty directory is ready to use."
	case "existing_git":
		return "Existing Git workspace is ready to use."
	}
	if validation.Valid {
		return "Workspace is ready to use."
	}
	if composerState.WorkspaceSourceType == "git" {
		return "Enter a valid Git repository URL."
	}
	return "Enter a valid local directory path."
}

func ComposerWorkspaceSourceValid(composerState state.ComposerPanelState) bool {
	return strings.TrimSpace(composerState.WorkspaceSourceInput) != "" &&
		composerState.WorkspaceValidationSet &&
		composerState.WorkspaceValidation.Valid
}
