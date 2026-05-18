package parts

import "strings"

func sessionStatusNormalized(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return "unknown"
	}
	return strings.ToLower(trimmed)
}

func sessionStatusLabel(status string) string {
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

func sessionStatusTone(status string) string {
	switch sessionStatusNormalized(status) {
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

func sessionStatusSpinning(status string) bool {
	switch sessionStatusNormalized(status) {
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
