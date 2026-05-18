package parts

import (
	"fmt"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func startupTaskStatusLabel(task viewmodel.StartupTask) string {
	switch task.State {
	case "pending":
		return "Pending"
	case "in_progress":
		return "In progress"
	case "failed":
		return "Failed"
	default:
		return "Completed"
	}
}

func startupTaskBadgeClass(task viewmodel.StartupTask) string {
	base := "inline-flex shrink-0 items-center rounded-full border px-2 py-0.5 text-xs font-medium"
	switch task.State {
	case "failed":
		return base + " border-destructive/30 bg-destructive/10 text-destructive"
	case "in_progress":
		return base + " border-transparent bg-secondary text-secondary-foreground"
	default:
		return base + " border-border text-muted-foreground"
	}
}

func startupTaskProgress(task viewmodel.StartupTask) (int, bool) {
	if task.Progress != nil {
		return *task.Progress, true
	}
	if task.BytesDownloaded != nil && task.TotalBytes != nil && *task.TotalBytes > 0 {
		return int((*task.BytesDownloaded * 100) / *task.TotalBytes), true
	}
	return 0, false
}

func startupTaskDetail(task viewmodel.StartupTask) string {
	if task.Error != "" {
		return task.Error
	}
	if task.CurrentOperation != "" {
		return task.CurrentOperation
	}
	if task.BytesDownloaded != nil && task.TotalBytes != nil && *task.TotalBytes > 0 {
		return fmt.Sprintf("%s of %s", formatBytes(*task.BytesDownloaded), formatBytes(*task.TotalBytes))
	}
	return ""
}

func startupTaskDetailClass(task viewmodel.StartupTask) string {
	if task.State == "failed" {
		return "mt-0.5 line-clamp-2 text-xs text-destructive"
	}
	return "mt-0.5 line-clamp-2 text-xs text-muted-foreground"
}

func formatBytes(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(value)
	unitIndex := -1
	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	if size >= 10 {
		return fmt.Sprintf("%.0f %s", size, units[unitIndex])
	}
	return fmt.Sprintf("%.1f %s", size, units[unitIndex])
}
