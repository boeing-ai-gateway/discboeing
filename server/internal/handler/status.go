package handler

import (
	"net/http"
	"os"
	"runtime"

	"github.com/obot-platform/discobot/server/client"
	"github.com/obot-platform/discobot/server/internal/startup"
	"github.com/obot-platform/discobot/server/internal/version"
)

// GetServerConfig returns public server configuration
func (h *Handler) GetServerConfig(w http.ResponseWriter, _ *http.Request) {
	h.JSON(w, http.StatusOK, client.ServerConfig{
		SSHPort:       h.cfg.SSHPort,
		HTTPPort:      h.cfg.Port,
		HTTPSPort:     h.cfg.HTTPSPort,
		HTTPSTLSMode:  h.cfg.HTTPSTLSMode,
		PublicBaseURL: h.cfg.PublicBaseURL(),
	})
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
	h.JSON(w, http.StatusOK, client.SystemStatusResponse{
		OK:       true,
		Messages: []client.StatusMessage{},
	})
}

func mapSystemStatus(status startup.SystemStatusResponse) client.SystemStatusResponse {
	out := client.SystemStatusResponse{
		OK:       status.OK,
		Messages: make([]client.StatusMessage, 0, len(status.Messages)),
	}
	for _, message := range status.Messages {
		out.Messages = append(out.Messages, client.StatusMessage{
			ID:      message.ID,
			Level:   client.StatusMessageLevel(message.Level),
			Title:   message.Title,
			Message: message.Message,
		})
	}
	if len(status.StartupTasks) > 0 {
		out.StartupTasks = make([]*client.StartupTask, 0, len(status.StartupTasks))
		for _, task := range status.StartupTasks {
			if task == nil {
				continue
			}
			out.StartupTasks = append(out.StartupTasks, &client.StartupTask{
				ID:               task.ID,
				Name:             task.Name,
				State:            string(task.State),
				Progress:         task.Progress,
				CurrentOperation: task.CurrentOperation,
				BytesDownloaded:  task.BytesDownloaded,
				TotalBytes:       task.TotalBytes,
				Error:            task.Error,
				StartedAt:        task.StartedAt,
				CompletedAt:      task.CompletedAt,
			})
		}
	}
	return out
}

// GetSupportInfo returns comprehensive diagnostic information for debugging
func (h *Handler) GetSupportInfo(w http.ResponseWriter, _ *http.Request) {
	// Get runtime info
	runtimeInfo := client.RuntimeInfo{
		OS:           runtime.GOOS,
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

	configInfo := client.ConfigInfo{
		Port:               h.cfg.Port,
		HTTPSPort:          h.cfg.HTTPSPort,
		HTTPSTLSMode:       h.cfg.HTTPSTLSMode,
		DatabaseDriver:     h.cfg.DatabaseDriver,
		AuthEnabled:        h.cfg.AuthEnabled,
		WorkspaceDir:       h.cfg.WorkspaceDir,
		SandboxImage:       h.cfg.SandboxImage,
		SandboxImageRemote: h.cfg.SandboxImageRemote,
		DesktopMode:        h.cfg.DesktopMode,
		DesktopRuntime:     h.cfg.DesktopRuntime,
		SSHEnabled:         h.cfg.SSHEnabled,
		SSHPort:            h.cfg.SSHPort,
		DispatcherEnabled:  h.cfg.DispatcherEnabled,
		AvailableProviders: availableProviders,
	}

	// Add VZ info if on macOS
	if runtime.GOOS == "darwin" {
		vzInfo := &client.VZInfo{
			ImageRef:     h.cfg.VZImageRef,
			DataDir:      h.cfg.VZDataDir,
			CPUCount:     h.cfg.VZCPUCount,
			MemoryMB:     h.cfg.VZMemoryMB,
			DataDiskGB:   h.cfg.VZDataDiskGB,
			KernelPath:   h.cfg.VZKernelPath,
			InitrdPath:   h.cfg.VZInitrdPath,
			BaseDiskPath: h.cfg.VZBaseDiskPath,
		}

		// Get disk usage for VZ data directory
		if diskUsage := getDiskUsage(h.cfg.VZDataDir); diskUsage != nil {
			vzInfo.DiskUsage = diskUsage
		}

		// Scan for data disk files
		vzInfo.DataDisks = getDataDiskFiles(h.cfg.VZDataDir)

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
	systemStatus := client.SystemStatusResponse{}
	if h.sandboxService != nil {
		h.sandboxService.RefreshProviderStatuses()
	}
	if h.systemManager != nil {
		systemStatus = mapSystemStatus(h.systemManager.GetSystemStatus())
	}

	response := client.SupportInfoResponse{
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
