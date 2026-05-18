package parts

import (
	"regexp"
	"strings"
)

var (
	providerIconScriptPattern        = regexp.MustCompile(`(?is)<script[\s\S]*?>[\s\S]*?</script>`)
	providerIconForeignObjectPattern = regexp.MustCompile(`(?is)<foreignObject[\s\S]*?>[\s\S]*?</foreignObject>`)
	providerIconEmbeddedPattern      = regexp.MustCompile(`(?is)<(iframe|object|embed)[\s\S]*?>[\s\S]*?</(iframe|object|embed)>`)
	providerIconEventAttrPattern     = regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	providerIconJSRefPattern         = regexp.MustCompile(`(?i)\s(href|xlink:href)\s*=\s*("[^"]*javascript:[^"]*"|'[^']*javascript:[^']*'|[^\s>]*javascript:[^\s>]*)`)
)

func providerIconClass(className string) string {
	base := "inline-flex size-7 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-muted text-muted-foreground"
	if className != "" {
		base += " " + className
	}
	return base
}

func providerIconTrimmed(icon string) string {
	return strings.TrimSpace(icon)
}

func providerIconIsImageReference(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") || strings.HasPrefix(lower, "data:image/") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../")
}

func providerIconIsInlineSVG(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "<svg")
}

func providerIconSanitizeInlineSVG(value string) string {
	value = providerIconScriptPattern.ReplaceAllString(value, "")
	value = providerIconForeignObjectPattern.ReplaceAllString(value, "")
	value = providerIconEmbeddedPattern.ReplaceAllString(value, "")
	value = providerIconEventAttrPattern.ReplaceAllString(value, "")
	value = providerIconJSRefPattern.ReplaceAllString(value, "")
	return value
}

func providerIconInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "P"
	}
	var initials strings.Builder
	for index, part := range parts {
		if index >= 2 {
			break
		}
		initials.WriteString(strings.ToUpper(part[:1]))
	}
	if initials.Len() == 0 {
		return "P"
	}
	return initials.String()
}
