package splitdropdownbutton

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

func useSharedOutlineBorder(variant string) bool {
	return variant == "" || variant == "outline"
}

func groupClass(variant string, className string) string {
	if useSharedOutlineBorder(variant) {
		return joinClass("inline-flex items-center overflow-hidden rounded-md border border-border bg-background p-0.5 shadow-xs", className)
	}
	return joinClass("inline-flex items-center", className)
}

func primaryButtonClass(variant string) string {
	if useSharedOutlineBorder(variant) {
		return "rounded-l-[calc(var(--radius)-1px)] rounded-r-none border-0 bg-transparent shadow-none dark:bg-transparent"
	}
	return "rounded-r-none"
}

func triggerButtonClass(variant string) string {
	if useSharedOutlineBorder(variant) {
		return "rounded-r-[calc(var(--radius)-1px)] rounded-l-none border-0 border-l border-border bg-transparent px-2 shadow-none dark:bg-transparent"
	}
	return "rounded-l-none border-l-0 px-2"
}

func iconClass(size string) string {
	if size == "xs" || size == "icon-xs" || size == "" {
		return "size-3.5"
	}
	return "size-4"
}
