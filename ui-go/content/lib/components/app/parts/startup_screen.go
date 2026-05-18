package parts

import (
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func startupScreenClass(snapshot viewmodel.StartupScreenSnapshot) string {
	base := "w-full space-y-4"
	if strings.TrimSpace(snapshot.ClassName) != "" {
		base += " " + snapshot.ClassName
	}
	return base
}

func startupScreenCardClass(snapshot viewmodel.StartupScreenSnapshot) string {
	base := "mx-auto flex w-full flex-col items-center rounded-2xl border border-border bg-background text-center shadow-sm"
	if snapshot.ShowShellPreview {
		return base + " max-w-md gap-4 px-8 py-8"
	}
	return base + " max-w-sm gap-3 px-6 py-6"
}

func startupScreenBrandHeight(snapshot viewmodel.StartupScreenSnapshot) string {
	if snapshot.ShowShellPreview {
		return "h-[2.5rem]"
	}
	return "h-5"
}

func startupScreenModeLabel(snapshot viewmodel.StartupScreenSnapshot) string {
	if snapshot.ModeLabel != "" {
		return snapshot.ModeLabel
	}
	return "Starting up"
}

func startupScreenStatusLabel(snapshot viewmodel.StartupScreenSnapshot) string {
	if snapshot.StatusLabel != "" {
		return snapshot.StatusLabel
	}
	return "Starting"
}

func startupScreenProgress(snapshot viewmodel.StartupScreenSnapshot) int {
	if snapshot.Progress < 0 {
		return 0
	}
	if snapshot.Progress > 100 {
		return 100
	}
	return snapshot.Progress
}

func startupScreenAPIState(snapshot viewmodel.StartupScreenSnapshot) string {
	switch snapshot.APIState {
	case "online", "retrying", "offline":
		return snapshot.APIState
	default:
		return "offline"
	}
}

func startupScreenAPILabel(state string) string {
	switch state {
	case "online":
		return "reachable"
	case "retrying":
		return "retrying"
	default:
		return "offline"
	}
}

func startupScreenAPIClass(state string) string {
	switch state {
	case "online":
		return "text-emerald-600 dark:text-emerald-400"
	case "retrying":
		return "text-blue-600 dark:text-blue-400"
	default:
		return "text-destructive"
	}
}

func startupScreenStepTone(state string) string {
	switch state {
	case "done":
		return "text-emerald-600 dark:text-emerald-400"
	case "running":
		return "text-blue-600 dark:text-blue-400"
	case "failed":
		return "text-destructive"
	default:
		return "text-muted-foreground"
	}
}

func startupScreenStepDotClass(state string) string {
	switch state {
	case "done":
		return "bg-emerald-500"
	case "running":
		return "bg-blue-500 animate-pulse"
	case "failed":
		return "bg-destructive"
	default:
		return "bg-muted-foreground/30"
	}
}

func startupScreenStepState(state string) string {
	if state == "" {
		return "pending"
	}
	return state
}

func startupScreenDismissLabel(snapshot viewmodel.StartupScreenSnapshot) string {
	if snapshot.DismissLabel != "" {
		return snapshot.DismissLabel
	}
	return "Continue in background"
}

func startupScreenShowDetailsToggle(snapshot viewmodel.StartupScreenSnapshot) bool {
	return snapshot.ErrorMessage != "" || len(snapshot.Steps) > 0 || snapshot.ShowShellPreview
}

func startupScreenRetryLabel(snapshot viewmodel.StartupScreenSnapshot) string {
	if snapshot.RetryCount <= 0 {
		return ""
	}
	return "retry " + strconv.Itoa(snapshot.RetryCount)
}
