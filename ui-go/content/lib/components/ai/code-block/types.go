package codeblock

import "strings"

type View struct {
	Code            string
	Language        string
	ShowLineNumbers bool
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

func codeLines(code string) []string {
	return strings.Split(code, "\n")
}

func codeLineClass(showLineNumbers bool) string {
	if showLineNumbers {
		return "block whitespace-pre before:content-[counter(line)] before:inline-block before:[counter-increment:line] before:w-8 before:mr-4 before:text-right before:text-muted-foreground/50 before:font-mono before:select-none"
	}
	return "block whitespace-pre"
}

func codeElementClass(showLineNumbers bool) string {
	if showLineNumbers {
		return "font-mono text-sm whitespace-normal [counter-increment:line_0] [counter-reset:line]"
	}
	return "font-mono text-sm whitespace-normal"
}

//nolint:unused // Referenced from generated templ code; golangci-lint filters generated files.
func copyButtonClass(className string) string {
	return classNames("inline-flex size-9 shrink-0 items-center justify-center rounded-md text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-hidden disabled:pointer-events-none disabled:opacity-50", className)
}

func copyButtonLabel(label string) string {
	if strings.TrimSpace(label) == "" {
		return "Copy code"
	}
	return strings.TrimSpace(label)
}
