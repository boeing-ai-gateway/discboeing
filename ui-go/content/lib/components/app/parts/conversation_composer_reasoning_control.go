package parts

import (
	"net/url"
	"strconv"
	"strings"
)

func composerReasoningLabel(level string) string {
	switch level {
	case "", "default":
		return "Default"
	case "xhigh":
		return "X-High"
	default:
		return upperFirst(level)
	}
}

func composerResolvedReasoning(value string, defaultValue string) string {
	if value == "" || value == "default" {
		return defaultValue
	}
	return value
}

func composerDefaultReasoningLabel(defaultValue string) string {
	if defaultValue == "" {
		return "Default"
	}
	return composerReasoningLabel(defaultValue) + " (default)"
}

func composerDefaultReasoningDescription(defaultValue string) string {
	if defaultValue == "" {
		return "Use the model default"
	}
	return "Use the model default (" + composerReasoningLabel(defaultValue) + ")"
}

func composerReasoningDescription(level string) string {
	if level == "none" {
		return "Use no reasoning effort"
	}
	return "Use " + level + " reasoning effort"
}

func composerReasoningSelected(value string, level string) string {
	if level == "default" {
		return strconv.FormatBool(value == "" || value == "default")
	}
	return strconv.FormatBool(value == level)
}

func composerReasoningLevelCount(levels []string, defaultValue string) string {
	count := 0
	for _, level := range levels {
		if level != defaultValue {
			count++
		}
	}
	return strconv.Itoa(count)
}

func composerReasoningOptionCommand(level string) string {
	return "@post('/ui/commands/composer-reasoning?reasoning=" + url.QueryEscape(level) + "')"
}

func upperFirst(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
