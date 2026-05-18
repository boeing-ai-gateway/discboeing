package parts

import "strings"

func threadStateBadgeLabel(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "interrupted":
		return "Interrupted"
	case "cancelled":
		return "Cancelled"
	default:
		return ""
	}
}

func threadStateBadgeTone(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "interrupted":
		return "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-300"
	case "cancelled":
		return "border-current/15 bg-current/10 text-current/75"
	default:
		return ""
	}
}

func threadStateBadgeClass(state string, className string) string {
	base := "inline-flex shrink-0 items-center rounded-full border px-1.5 py-0.5 text-[10px] font-medium"
	if tone := threadStateBadgeTone(state); tone != "" {
		base += " " + tone
	}
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}
