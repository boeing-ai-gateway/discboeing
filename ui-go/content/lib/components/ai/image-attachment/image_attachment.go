package imageattachment

import "strings"

func classNames(parts ...string) string {
	classes := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			classes = append(classes, trimmed)
		}
	}
	return strings.Join(classes, " ")
}

func imageFilename(filename string) string {
	if strings.TrimSpace(filename) == "" {
		return "Image attachment"
	}
	return filename
}
