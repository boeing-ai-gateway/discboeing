package parts

import (
	"strconv"
	"strings"
)

func composerServiceTierLabel(tier string) string {
	switch strings.ToLower(tier) {
	case "priority", "fast":
		return "Fast"
	default:
		return "Standard"
	}
}

func composerServiceTierDescription(tier string) string {
	switch strings.ToLower(tier) {
	case "priority", "fast":
		return "Use the provider priority service tier"
	default:
		return "Use the " + tier + " service tier"
	}
}

func composerServiceTierButtonClass(value string) string {
	base := "desktop-no-drag inline-flex h-6 items-center gap-1.5 rounded-md px-2 text-xs hover:bg-accent hover:text-accent-foreground"
	if value != "" {
		return base + " bg-secondary text-secondary-foreground"
	}
	return base + " text-muted-foreground"
}

func composerServiceTierSelected(value string, tier string) string {
	if tier == "" {
		return strconv.FormatBool(value == "")
	}
	return strconv.FormatBool(strings.EqualFold(value, tier))
}
