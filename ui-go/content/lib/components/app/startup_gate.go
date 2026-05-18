package app

import (
	"fmt"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func startupGateShellClass(startup viewmodel.StartupSnapshot) string {
	if startupGateShellVisible(startup) {
		return "transition-opacity duration-200 opacity-100"
	}
	return "pointer-events-none select-none transition-opacity duration-200 opacity-0"
}

func startupGateShellVisible(startup viewmodel.StartupSnapshot) bool {
	return startup.Ready || startup.Phase == "" || startup.Phase == "ready"
}

func startupGatePhase(startup viewmodel.StartupSnapshot) string {
	if startup.Phase != "" {
		return startup.Phase
	}
	return "ready"
}

func startupGateShowAuth(startup viewmodel.StartupSnapshot) bool {
	return startup.Phase == "auth"
}

func startupGateShowOverlay(startup viewmodel.StartupSnapshot) bool {
	return !startupGateShellVisible(startup) && !startupGateShowAuth(startup)
}

func startupGateHeadline(startup viewmodel.StartupSnapshot) string {
	if hasFailedStartupTask(startup.VisibleTasks) {
		return "A startup task needs attention"
	}
	if hasPendingStartupTask(startup.VisibleTasks) {
		return "Completing startup tasks"
	}
	switch startup.Phase {
	case "error":
		return "Discobot could not connect to the backend"
	case "auth":
		return "Sign in to Discobot"
	case "loading":
		return "Loading the workspace shell"
	case "waiting":
		if startup.RetryCount > 0 {
			return "Retrying backend startup checks"
		}
		return "Waiting for the backend API"
	case "ready":
		return "Discobot is ready"
	default:
		return "Booting the desktop shell"
	}
}

func startupGateDetail(startup viewmodel.StartupSnapshot) string {
	if hasFailedStartupTask(startup.VisibleTasks) {
		return "Discobot connected to the backend, but one of the startup tasks reported an error."
	}
	if hasPendingStartupTask(startup.VisibleTasks) {
		if len(startup.VisibleTasks) == 1 {
			return "Discobot is waiting for one backend startup task to finish."
		}
		return fmt.Sprintf("Discobot is waiting for %d backend startup tasks to finish.", len(startup.VisibleTasks))
	}
	switch startup.Phase {
	case "error":
		return "The shell is still waiting for the server to become available before it can render."
	case "auth":
		return "The backend is ready, but you need to authenticate before the workspace shell can load."
	case "loading":
		return "The backend is ready, and Discobot is fetching the first set of app data."
	case "waiting":
		if startup.RetryCount > 0 {
			return "Discobot is polling the live status endpoint until the backend is reachable."
		}
		return "Discobot is waiting for the backend status endpoint before it reveals the shell."
	case "ready":
		return "Startup checks passed and the workspace shell is ready to render."
	default:
		return "Discobot is initializing the desktop runtime and preparing to contact the backend."
	}
}

func startupGateStatusLabel(startup viewmodel.StartupSnapshot) string {
	if hasFailedStartupTask(startup.VisibleTasks) {
		return "Task failed"
	}
	if hasRunningStartupTask(startup.VisibleTasks) {
		return "Running tasks"
	}
	if hasPendingStartupTask(startup.VisibleTasks) {
		return "Queued tasks"
	}
	switch startup.Phase {
	case "error":
		return "Needs attention"
	case "auth":
		return "Authentication required"
	case "loading":
		return "Hydrating data"
	case "waiting":
		if startup.RetryCount > 0 {
			return "Retrying API"
		}
		return "Waiting for API"
	case "ready":
		return "Ready"
	default:
		return "Initializing"
	}
}

func startupGateProgress(startup viewmodel.StartupSnapshot) int {
	if len(startup.VisibleTasks) > 0 {
		total := 0
		for _, task := range startup.VisibleTasks {
			total += startupGateTaskProgress(task)
		}
		return total / len(startup.VisibleTasks)
	}
	switch startup.Phase {
	case "ready", "auth":
		return 100
	case "loading":
		return 80
	case "waiting":
		return 30
	default:
		return 15
	}
}

func startupGateAPIState(startup viewmodel.StartupSnapshot) string {
	switch {
	case startup.Phase == "auth" || startup.Phase == "loading" || startup.Phase == "ready" || len(startup.VisibleTasks) > 0:
		return "online"
	case startup.RetryCount > 0:
		return "retrying"
	default:
		return "offline"
	}
}

func startupGateAPIClass(apiState string) string {
	switch apiState {
	case "online":
		return "text-emerald-600 dark:text-emerald-400"
	case "retrying":
		return "text-blue-600 dark:text-blue-400"
	default:
		return "text-destructive"
	}
}

func startupGateAPILabel(apiState string) string {
	switch apiState {
	case "online":
		return "reachable"
	case "retrying":
		return "retrying"
	default:
		return "offline"
	}
}

func startupGateTaskState(task viewmodel.StartupTask) string {
	switch task.State {
	case "in_progress":
		return "running"
	case "completed":
		return "done"
	case "failed":
		return "failed"
	default:
		return "pending"
	}
}

func startupGateTaskDotClass(task viewmodel.StartupTask) string {
	switch startupGateTaskState(task) {
	case "done":
		return "bg-emerald-500"
	case "running":
		return "bg-blue-500 animate-pulse"
	case "failed":
		return "bg-destructive"
	default:
		return "bg-muted-foreground/30"
	}
}

func startupGateTaskTone(task viewmodel.StartupTask) string {
	switch startupGateTaskState(task) {
	case "done":
		return "text-emerald-600 dark:text-emerald-400"
	case "running":
		return "text-blue-600 dark:text-blue-400"
	case "failed":
		return "text-destructive"
	default:
		return "text-muted-foreground"
	}
}

func startupGateTaskDetail(task viewmodel.StartupTask) string {
	if task.Error != "" {
		return task.Error
	}
	if task.CurrentOperation != "" {
		return task.CurrentOperation
	}
	if task.BytesDownloaded != nil && task.TotalBytes != nil && *task.TotalBytes > 0 {
		return fmt.Sprintf("%d of %d bytes", *task.BytesDownloaded, *task.TotalBytes)
	}
	return "Waiting for the server to report more detail."
}

func startupGateTaskProgress(task viewmodel.StartupTask) int {
	if task.Progress != nil {
		return *task.Progress
	}
	if task.BytesDownloaded != nil && task.TotalBytes != nil && *task.TotalBytes > 0 {
		return int((*task.BytesDownloaded * 100) / *task.TotalBytes)
	}
	switch task.State {
	case "completed":
		return 100
	case "in_progress":
		return 50
	default:
		return 0
	}
}

func hasFailedStartupTask(tasks []viewmodel.StartupTask) bool {
	for _, task := range tasks {
		if task.State == "failed" {
			return true
		}
	}
	return false
}

func hasRunningStartupTask(tasks []viewmodel.StartupTask) bool {
	for _, task := range tasks {
		if task.State == "in_progress" {
			return true
		}
	}
	return false
}

func hasPendingStartupTask(tasks []viewmodel.StartupTask) bool {
	for _, task := range tasks {
		if task.State == "pending" || task.State == "in_progress" {
			return true
		}
	}
	return false
}
