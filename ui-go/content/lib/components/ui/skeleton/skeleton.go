package skeleton

import "strings"

func skeletonClass(className string) string {
	base := "bg-accent animate-pulse rounded-md"
	if strings.TrimSpace(className) == "" {
		return base
	}
	return base + " " + strings.TrimSpace(className)
}
