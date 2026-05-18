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

func setupValidationClass(snapshot viewmodel.ComposerSessionSetupStatusSnapshot) string {
	if snapshot.ValidationIsValid {
		return "mb-2 truncate px-1 text-xs text-muted-foreground"
	}
	return "mb-2 truncate px-1 text-xs text-destructive"
}
