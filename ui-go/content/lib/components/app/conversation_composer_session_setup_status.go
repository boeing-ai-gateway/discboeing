package app

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func showComposerSessionSetupStatus(isPending bool, snapshot viewmodel.ComposerSessionSetupStatusSnapshot) bool {
	return isPending || (strings.TrimSpace(snapshot.SessionStatus) != "" && snapshot.SessionStatus != "ready")
}

func showComposerSessionStatus(snapshot viewmodel.ComposerSessionSetupStatusSnapshot) bool {
	return strings.TrimSpace(snapshot.SessionStatus) != "" && snapshot.SessionStatus != "ready"
}

func setupInlineMessage(snapshot viewmodel.ComposerSessionSetupStatusSnapshot) string {
	if snapshot.WorkspacesLoading {
		return "Loading workspaces..."
	}
	if message := strings.TrimSpace(snapshot.SetupMessage); message != "" {
		return message
	}
	return strings.TrimSpace(snapshot.ValidationMessage)
}

func setupInlineMessageClass(snapshot viewmodel.ComposerSessionSetupStatusSnapshot) string {
	base := "truncate text-xs leading-4 "
	if snapshot.WorkspacesLoading || (strings.TrimSpace(snapshot.SetupMessage) == "" && snapshot.ValidationIsValid) {
		return base + "text-muted-foreground"
	}
	return base + "text-destructive"
}
