package app

import (
	"fmt"
	"strings"
)

func sessionButtonClass(selected bool) string {
	base := "flex h-8 min-w-0 flex-1 items-center gap-2 overflow-hidden rounded-md px-2 text-left text-sm font-medium transition-colors"
	if selected {
		return base + " bg-sidebar-accent text-sidebar-accent-foreground shadow-inner"
	}
	return base + " text-sidebar-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
}

func threadButtonClass(selected bool) string {
	base := "flex min-h-8 w-full min-w-0 items-center gap-2 overflow-hidden rounded-md px-2 py-1 text-left text-sm transition-colors"
	if selected {
		return base + " bg-sidebar-accent text-sidebar-accent-foreground shadow-inner"
	}
	return base + " text-sidebar-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
}

func rowActionButtonClass(selected bool) string {
	base := "h-8 w-7 rounded-md transition-colors"
	if selected {
		return base + " bg-sidebar-accent text-sidebar-accent-foreground shadow-inner hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
	}
	return base + " text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
}

func sidebarItemName(displayName string, name string, fallback string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	if strings.TrimSpace(name) != "" {
		return name
	}
	return fallback
}

func sidebarThreadStatus(status string, state string) string {
	if strings.TrimSpace(status) != "" {
		return status
	}
	if strings.TrimSpace(state) != "" {
		return state
	}
	return "unknown"
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func statusLabel(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		trimmed = "unknown"
	}
	words := strings.Split(strings.ReplaceAll(trimmed, "_", " "), " ")
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func threadChildrenStyle(depth int) string {
	if depth < 0 {
		depth = 0
	}
	return fmt.Sprintf("margin-left: %drem;", depth+1)
}

func sessionStatusTone(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "error", "create_failed":
		return "text-destructive"
	case "needs_attention":
		return "text-amber-500"
	case "running":
		return "text-blue-500"
	case "queued", "pending", "committing", "initializing", "reinitializing", "cloning", "pulling_image", "creating_sandbox":
		return "text-yellow-500"
	case "idle", "ready", "completed", "committed":
		return "text-green-500"
	case "removing":
		return "text-orange-500"
	default:
		return "text-muted-foreground"
	}
}

func isSpinningSessionStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "queued", "pending", "committing", "initializing", "reinitializing", "cloning", "pulling_image", "creating_sandbox", "removing":
		return true
	default:
		return false
	}
}

func sessionStatusClass(showLabel bool, className string) string {
	base := "inline-flex items-center"
	if showLabel {
		base += " gap-1.5"
	}
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func sessionStatusIconClass(status string, className string) string {
	base := "inline-flex items-center " + sessionStatusTone(status)
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func sessionStatusLabelClass(className string) string {
	base := "text-sm text-muted-foreground"
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func threadStateLabel(state string) string {
	switch state {
	case "interrupted":
		return "Interrupted"
	case "cancelled":
		return "Cancelled"
	default:
		return ""
	}
}

func threadStateTone(state string) string {
	switch state {
	case "interrupted":
		return "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300"
	case "cancelled":
		return "border-current/15 bg-current/10 text-current/75"
	default:
		return ""
	}
}
