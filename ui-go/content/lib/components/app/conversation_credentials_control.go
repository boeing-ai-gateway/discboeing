package app

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func visibleCredentialCount(snapshot viewmodel.ConversationCredentialsSnapshot) int {
	count := 0
	for _, assignment := range snapshot.Assignments {
		if assignment.Inactive {
			continue
		}
		if assignment.Visibility.Tools || assignment.Visibility.Console || assignment.Visibility.Services || assignment.Visibility.Hooks {
			count++
		}
	}
	return count
}

func credentialRuntimeClass(enabled bool, inactive bool) string {
	base := "inline-flex size-7 items-center justify-center rounded-md border text-[11px] font-semibold transition-colors"
	if inactive {
		return base + " border-border/60 bg-muted/40 text-muted-foreground opacity-50"
	}
	if enabled {
		return base + " border-yellow-500/40 bg-yellow-500/12 text-yellow-600 shadow-sm dark:text-yellow-400"
	}
	return base + " border-transparent bg-muted/55 text-muted-foreground"
}

func credentialUseClass(use viewmodel.CredentialUse) string {
	base := "flex w-full items-start gap-2 rounded-md border px-2 py-1.5"
	if use.Expired {
		return base + " border-border/50 bg-muted/25 text-muted-foreground"
	}
	return base + " border-border/70 bg-muted/35"
}

func credentialUsesLabel(count int) string {
	if count == 1 {
		return "1 use"
	}
	return strconv.Itoa(count) + " uses"
}

func credentialAllVisibilityAria(assignment viewmodel.SessionCredentialAssignment) string {
	enabled := 0
	if assignment.Visibility.Tools {
		enabled++
	}
	if assignment.Visibility.Console {
		enabled++
	}
	if assignment.Visibility.Services {
		enabled++
	}
	if assignment.Visibility.Hooks {
		enabled++
	}
	switch enabled {
	case 0:
		return "false"
	case 4:
		return "true"
	default:
		return "mixed"
	}
}

func credentialAllVisibilityClass(assignment viewmodel.SessionCredentialAssignment) string {
	base := "inline-flex size-4 items-center justify-center rounded-sm border"
	if assignment.Inactive {
		return base + " border-border/60 bg-muted/40 text-muted-foreground opacity-50"
	}
	if credentialAllVisibilityAria(assignment) == "false" {
		return base + " border-border bg-background"
	}
	return base + " border-yellow-500/40 bg-yellow-500/12 text-yellow-600 dark:text-yellow-400"
}
