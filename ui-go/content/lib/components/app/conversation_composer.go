package app

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func composerSubmitDisabled(snapshot viewmodel.ConversationComposerSnapshot) bool {
	return strings.TrimSpace(snapshot.DisabledMessage) != ""
}

func composerInputEmpty(snapshot viewmodel.ConversationComposerSnapshot) bool {
	return strings.TrimSpace(snapshot.Draft) == "" && len(snapshot.Attachments) == 0
}

func credentialKeyTone(count int) string {
	if count > 0 {
		return "bg-yellow-500"
	}
	return "bg-muted-foreground"
}

func credentialCountClass(count int) string {
	base := "inline-flex min-w-3 items-center justify-center text-[10px] tabular-nums"
	if count <= 1 {
		return base + " invisible"
	}
	return base + " text-foreground"
}

func promptHistoryRecentHeaderClass(snapshot viewmodel.ConversationPromptHistorySnapshot) string {
	if len(snapshot.PinnedPrompts) > 0 {
		return "border-t border-border"
	}
	return ""
}

func composerMobileWorkspaceSelector(snapshot viewmodel.ConversationWorkspaceSelectorSnapshot) viewmodel.ConversationWorkspaceSelectorSnapshot {
	snapshot.FullWidth = true
	return snapshot
}
