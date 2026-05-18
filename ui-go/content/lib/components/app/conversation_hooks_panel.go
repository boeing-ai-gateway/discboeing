package app

import (
	"fmt"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func hookPassedCount(hooks []viewmodel.HookStatus) int {
	count := 0
	for _, hook := range hooks {
		if hook.DisplayState == "success" {
			count++
		}
	}
	return count
}

func hookStatusTone(state string) string {
	switch state {
	case "running":
		return "text-blue-500"
	case "success":
		return "text-green-500"
	case "failure":
		return "text-red-500"
	default:
		return "text-muted-foreground"
	}
}

func hookStatusLabel(state string) string {
	switch state {
	case "running":
		return "Running"
	case "success":
		return "Passed"
	case "failure":
		return "Failed"
	default:
		return "Pending"
	}
}

func hookRowClass(state string) string {
	base := "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm"
	if state == "running" {
		return base + " bg-blue-500/10"
	}
	return base + " hover:bg-muted/50"
}

func canRerunHook(state string) bool {
	return state != "running"
}

func hookOutputText(output string) string {
	if output == "" {
		return "No output available"
	}
	return output
}

func hookOutputSizeText(displayed int64, total int64) string {
	return fmt.Sprintf("Showing the last %s of %s.", formatHookBytes(displayed), formatHookBytes(total))
}

func hookRowAriaLabel(hook viewmodel.HookStatus) string {
	name := strings.TrimSpace(hook.Name)
	if name == "" {
		name = "Hook"
	}
	return fmt.Sprintf("%s, %s, %d runs", name, hookStatusLabel(hook.DisplayState), hook.RunCount)
}

func formatHookBytes(value int64) string {
	if value <= 0 {
		return "0 B"
	}
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	units := []string{"KB", "MB", "GB"}
	size := float64(value)
	unitIndex := -1
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	if size >= 10 {
		return fmt.Sprintf("%.0f %s", size, units[unitIndex])
	}
	return fmt.Sprintf("%.1f %s", size, units[unitIndex])
}
