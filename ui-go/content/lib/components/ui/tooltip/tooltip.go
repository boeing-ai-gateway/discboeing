package tooltip

import "strings"

func joinClass(parts ...string) string {
	classes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			classes = append(classes, trimmed)
		}
	}
	return strings.Join(classes, " ")
}

func state(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}

func side(value string) string {
	switch value {
	case "top", "bottom", "left", "right":
		return value
	default:
		return "top"
	}
}

func contentClass(className string) string {
	return joinClass("bg-foreground text-background animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-end-2 data-[side=right]:slide-in-from-start-2 data-[side=top]:slide-in-from-bottom-2 z-50 w-fit origin-(--bits-tooltip-content-transform-origin) rounded-md px-3 py-1.5 text-xs text-balance", className)
}

func arrowClass(className string) string {
	return joinClass("bg-foreground z-50 size-2.5 rotate-45 rounded-[2px] data-[side=top]:translate-x-1/2 data-[side=top]:translate-y-[calc(-50%_+_2px)] data-[side=bottom]:-translate-x-1/2 data-[side=bottom]:-translate-y-[calc(-50%_+_1px)] data-[side=right]:translate-x-[calc(50%_+_2px)] data-[side=right]:translate-y-1/2 data-[side=left]:-translate-y-[calc(50%_-_3px)]", className)
}
