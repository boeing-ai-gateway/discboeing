package card

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

func cardClass(className string) string {
	return joinClass("bg-card text-card-foreground flex flex-col gap-6 rounded-xl border py-6 shadow-sm", className)
}

func cardHeaderClass(className string) string {
	return joinClass("@container/card-header grid auto-rows-min grid-rows-[auto_auto] items-start gap-1.5 px-6 has-data-[slot=card-action]:grid-cols-[1fr_auto] [.border-b]:pb-6", className)
}

func cardTitleClass(className string) string {
	return joinClass("leading-none font-semibold", className)
}

func cardDescriptionClass(className string) string {
	return joinClass("text-muted-foreground text-sm", className)
}

func cardActionClass(className string) string {
	return joinClass("col-start-2 row-span-2 row-start-1 self-start justify-self-end", className)
}

func cardContentClass(className string) string {
	return joinClass("px-6", className)
}

func cardFooterClass(className string) string {
	return joinClass("flex items-center px-6 [.border-t]:pt-6", className)
}
