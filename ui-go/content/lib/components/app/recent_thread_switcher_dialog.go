package app

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func recentThreadSwitcherKey(thread viewmodel.SidebarThreadItem) string {
	return thread.SessionID + ":" + thread.ID
}

func recentThreadSwitcherItemClass(selected bool) string {
	base := "flex w-full items-start gap-3 rounded-xl px-3 py-3 text-left transition-colors"
	if selected {
		return base + " bg-accent text-accent-foreground shadow-sm"
	}
	return base + " text-foreground/85 hover:bg-accent/70 hover:text-accent-foreground"
}

func recentThreadSwitcherHelpText(helpText string) string {
	if helpText != "" {
		return helpText
	}
	return "↑↓ navigate · Enter open · Esc close"
}

func recentThreadSwitcherItemID(key string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-", " ", "-")
	return "recent-thread-" + replacer.Replace(key)
}

func recentThreadSwitcherActiveID(key string) string {
	if key == "" {
		return ""
	}
	return recentThreadSwitcherItemID(key)
}
