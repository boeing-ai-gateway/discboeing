package parts

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func composerQueueCompletedCount(entries []viewmodel.PlanEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.Status == "completed" {
			count++
		}
	}
	return count
}

func composerQueueControlLabel(entries []viewmodel.PlanEntry) string {
	return "Queue: " + strconv.Itoa(composerQueueCompletedCount(entries)) + " of " + strconv.Itoa(len(entries)) + " completed"
}
