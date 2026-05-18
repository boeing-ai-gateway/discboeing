package reasoning

import (
	"fmt"
	"strings"
)

type View struct {
	IsStreaming bool
	IsOpen      bool
	PreviewText string
	Duration    *int
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

func thinkingMessage(view View) string {
	if view.PreviewText != "" && !view.IsStreaming {
		return view.PreviewText
	}
	if view.IsStreaming || (view.Duration != nil && *view.Duration == 0) {
		return "Thinking..."
	}
	if view.Duration == nil {
		return "Thought for a few seconds"
	}
	return fmt.Sprintf("Thought for %d seconds", *view.Duration)
}

func openState(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}

func chevronClass(open bool) string {
	if open {
		return "size-4 text-muted-foreground transition-transform rotate-180"
	}
	return "size-4 text-muted-foreground transition-transform rotate-0"
}
