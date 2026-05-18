package parts

import (
	"net/url"
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
	base := "desktop-no-drag inline-flex h-6 items-center justify-center gap-1.5 rounded-md px-2 text-xs font-medium whitespace-nowrap transition-all outline-none hover:bg-accent hover:text-accent-foreground focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50"
	if value != "" {
		return base + " bg-secondary text-secondary-foreground"
	}
	return base
}

func composerServiceTierSelected(value string, tier string) string {
	if tier == "" {
		return strconv.FormatBool(value == "")
	}
	return strconv.FormatBool(strings.EqualFold(value, tier))
}

func composerServiceTierOptionCommand(tier string) string {
	return "@post('/ui/commands/composer-service-tier?tier=" + url.QueryEscape(tier) + "')"
}
