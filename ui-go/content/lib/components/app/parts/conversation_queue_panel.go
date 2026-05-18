package parts

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func conversationQueueCompletedCount(entries []viewmodel.PlanEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.Status == "completed" {
			count++
		}
	}
	return count
}

func conversationQueueEntryText(entry viewmodel.PlanEntry) string {
	if entry.Content != "" {
		return entry.Content
	}
	return entry.Title
}

func conversationQueueRowClass(status string) string {
	base := "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm"
	if status == "in_progress" {
		return base + " bg-blue-500/10"
	}
	return base + " hover:bg-muted/50"
}

func conversationQueueTextClass(status string) string {
	base := "flex-1 truncate"
	if status == "completed" {
		return base + " text-muted-foreground line-through"
	}
	return base + " text-foreground"
}
