package tool

import "strings"

type View struct {
	Open       bool
	Queued     bool
	ShowBorder bool
}

type HeaderView struct {
	Type         string
	State        string
	Title        string
	ToolName     string
	ShowIcon     bool
	Raw          bool
	CanToggleRaw bool
	CanCollapse  bool
	Queued       bool
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

func toolClass(view View, className string) string {
	base := "group group/tool not-prose mb-4 w-full rounded-md"
	if view.ShowBorder {
		base += " border"
	}
	return classNames(base, className)
}

func derivedName(header HeaderView) string {
	if header.Type == "dynamic-tool" {
		if header.ToolName != "" {
			return header.ToolName
		}
		return "tool"
	}
	parts := strings.Split(header.Type, "-")
	if len(parts) <= 1 {
		return header.Type
	}
	return strings.Join(parts[1:], "-")
}

func displayText(header HeaderView) string {
	if header.Title != "" {
		return header.Title
	}
	return derivedName(header)
}

func splitVerb(header HeaderView) (string, string) {
	text := displayText(header)
	verb, rest, ok := strings.Cut(text, ": ")
	if !ok {
		return "", text
	}
	return verb, rest
}

func effectiveState(state string, queued bool) string {
	if queued && isToolRunningState(state) {
		return "queued"
	}
	return state
}

func toolStatusLabel(state string) string {
	switch state {
	case "input-streaming":
		return "Preparing"
	case "input-available":
		return "Running"
	case "queued":
		return "Queued"
	case "approval-requested":
		return "Awaiting Approval"
	case "approval-responded":
		return "Responded"
	case "output-available":
		return "Completed"
	case "output-error":
		return "Error"
	case "output-denied":
		return "Denied"
	default:
		return state
	}
}

func isToolRunningState(state string) bool {
	return state == "input-streaming" || state == "input-available"
}

func isToolPreparingState(state string) bool {
	return state == "input-streaming"
}

func rawTitle(raw bool) string {
	if raw {
		return "Show optimized view"
	}
	return "Show raw view"
}
