package message

import "strings"

type BranchView struct {
	MessageID string
	Current   int
	Total     int
}

func classNames(parts ...string) string {
	classes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			classes = append(classes, trimmed)
		}
	}
	return strings.Join(classes, " ")
}

func messageClass(from string, className string) string {
	base := "group flex w-full flex-col gap-2"
	if from == "user" {
		base += " is-user ml-auto max-w-[95%] justify-end"
	} else {
		base += " is-assistant"
	}
	return classNames(base, className)
}

func branchTotal(branch BranchView) int {
	if branch.Total < 1 {
		return 1
	}
	return branch.Total
}

func branchPage(branch BranchView) int {
	if branch.Current < 0 {
		return 1
	}
	if branch.Current >= branchTotal(branch) {
		return branchTotal(branch)
	}
	return branch.Current + 1
}

func branchButtonClass(className string) string {
	return classNames("focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive inline-flex size-8 shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap transition-all outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50 dark:hover:bg-accent/50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4", className)
}
