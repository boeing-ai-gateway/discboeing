package spinner

import "strings"

func spinnerClass(className string) string {
	base := "size-4 animate-spin"
	if strings.TrimSpace(className) == "" {
		return base
	}
	return base + " " + strings.TrimSpace(className)
}
