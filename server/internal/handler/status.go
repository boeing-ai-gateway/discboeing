package handler

import (
	"net/http"
	"os"
	"runtime"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/server/internal/startup"
	"github.com/obot-platform/discobot/server/internal/version"
)

// GetServerConfig returns public server configuration
func (h *Handler) GetServerConfig(w http.ResponseWriter, _ *http.Request) {
	response := api.ServerConfig{
		SSHPort:       h.cfg.SSHPort,
		HTTPPort:      h.cfg.Port,
		PublicBaseURL: h.cfg.PublicBaseURL(),
	}
	if h.cfg.HTTPSPort != 0 {
		response.HTTPSPort = &h.cfg.HTTPSPort
	}
	if h.cfg.HTTPSTLSMode != "" {
		response.HTTPSTLSMode = &h.cfg.HTTPSTLSMode
	}
	h.JSON(w, http.StatusOK, response)
}

// GetSystemStatus checks system requirements and returns status (including startup tasks)
func (h *Handler) GetSystemStatus(w http.ResponseWriter, _ *http.Request) {
	// Refresh provider status first so providers can reconcile any stale startup tasks.
	if h.sandboxService != nil {
		h.sandboxService.RefreshProviderStatuses()
	}

	// Use system manager to get complete system status
	if h.systemManager != nil {
		status := h.systemManager.GetSystemStatus()
		h.JSON(w, http.StatusOK, mapSystemStatus(status))
		return
	}

	// Fallback if system manager is not available
	h.JSON(w, http.StatusOK, api.SystemStatusResponse{
		Ok:       true,
		Messages: []api.StatusMessage{},
	})
}

func mapSystemStatus(status startup.SystemStatusResponse) api.SystemStatusResponse {
	out := api.SystemStatusResponse{
		Ok:       status.OK,
		Messages: make([]api.StatusMessage, 0, len(status.Messages)),
	}
	for _, message := range status.Messages {
		out.Messages = append(out.Messages, api.StatusMessage{
			ID:      message.ID,
			Level:   api.StatusMessageLevel(message.Level),
			Title:   message.Title,
			Message: message.Message,
		})
	}
	if len(status.StartupTasks) > 0 {
		startupTasks := make([]api.StartupTask, 0, len(status.StartupTasks))
		for _, task := range status.StartupTasks {
			if task == nil {
				continue
			}
			startupTask := api.StartupTask{
				ID:              task.ID,
				Name:            task.Name,
				State:           string(task.State),
				Progress:        task.Progress,
				BytesDownloaded: task.BytesDownloaded,
				TotalBytes:      task.TotalBytes,
				StartedAt:       task.StartedAt,
				CompletedAt:     task.CompletedAt,
			}
			if task.CurrentOperation != "" {
				startupTask.CurrentOperation = &task.CurrentOperation
			}
			if task.Error != "" {
				startupTask.Error = &task.Error
			}
			startupTasks = append(startupTasks, startupTask)
		}
		if len(startupTasks) > 0 {
			out.StartupTasks = &startupTasks
		}
	}
	return out
}

// GetSupportInfo returns comprehensive diagnostic information for debugging
func (h *Handler) GetSupportInfo(w http.ResponseWriter, _ *http.Request) {
	// Get runtime info
	runtimeInfo := api.RuntimeInfo{
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
	}

	// Get sanitized config info
	var availableProviders []string
	if h.sandboxService != nil {
		availableProviders = h.sandboxService.ListProviderNames()
	}

	configInfo := api.ConfigInfo{
		Port:               h.cfg.Port,
		DatabaseDriver:     h.cfg.DatabaseDriver,
		AuthEnabled:        h.cfg.AuthEnabled,
		WorkspaceDir:       h.cfg.WorkspaceDir,
		SandboxImage:       h.cfg.SandboxImage,
		DesktopMode:        h.cfg.DesktopMode,
		SSHEnabled:         h.cfg.SSHEnabled,
		SSHPort:            h.cfg.SSHPort,
		DispatcherEnabled:  h.cfg.DispatcherEnabled,
		AvailableProviders: availableProviders,
	}
	if h.cfg.HTTPSPort != 0 {
		configInfo.HTTPSPort = &h.cfg.HTTPSPort
	}
	if h.cfg.HTTPSTLSMode != "" {
		configInfo.HTTPSTLSMode = &h.cfg.HTTPSTLSMode
	}
	if h.cfg.SandboxImageRemote != "" {
		configInfo.SandboxImageRemote = &h.cfg.SandboxImageRemote
	}
	if h.cfg.DesktopRuntime != "" {
		configInfo.DesktopRuntime = &h.cfg.DesktopRuntime
	}

	// Add VZ info if on macOS
	if runtime.GOOS == "darwin" {
		vzInfo := &api.VZInfo{
			ImageRef:   h.cfg.VZImageRef,
			DataDir:    h.cfg.VZDataDir,
			CPUCount:   h.cfg.VZCPUCount,
			MemoryMb:   h.cfg.VZMemoryMB,
			DataDiskGb: h.cfg.VZDataDiskGB,
		}
		if h.cfg.VZKernelPath != "" {
			vzInfo.KernelPath = &h.cfg.VZKernelPath
		}
		if h.cfg.VZInitrdPath != "" {
			vzInfo.InitrdPath = &h.cfg.VZInitrdPath
		}
		if h.cfg.VZBaseDiskPath != "" {
			vzInfo.BaseDiskPath = &h.cfg.VZBaseDiskPath
		}

		// Get disk usage for VZ data directory
		if diskUsage := getDiskUsage(h.cfg.VZDataDir); diskUsage != nil {
			vzInfo.DiskUsage = diskUsage
		}

		// Scan for data disk files
		if dataDisks := getDataDiskFiles(h.cfg.VZDataDir); len(dataDisks) > 0 {
			vzInfo.DataDisks = &dataDisks
		}

		configInfo.VZ = vzInfo
	}

	// Read server log file
	logPath := h.cfg.ServerLogPath
	logContent := ""
	logExists := false

	if logData, err := os.ReadFile(logPath); err == nil {
		logContent = string(logData)
		logExists = true
	}

	// Get system status from system manager
	systemStatus := api.SystemStatusResponse{}
	if h.sandboxService != nil {
		h.sandboxService.RefreshProviderStatuses()
	}
	if h.systemManager != nil {
		systemStatus = mapSystemStatus(h.systemManager.GetSystemStatus())
	}

	response := api.SupportInfoResponse{
		Version:    version.Get(),
		Runtime:    runtimeInfo,
		Config:     configInfo,
		ServerLog:  logContent,
		LogPath:    logPath,
		LogExists:  logExists,
		SystemInfo: systemStatus,
	}

	h.JSON(w, http.StatusOK, response)
}

// getDiskUsage returns filesystem usage statistics for a given path
// Platform-specific implementations in status_unix.go and status_windows.go

// getDataDiskFiles scans for project data disk images and returns their size info.
// Platform-specific implementations in status_unix.go and status_windows.go
