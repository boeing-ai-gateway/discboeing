package parts

import (
	"strconv"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func projectSettingsResourceDescription(snapshot viewmodel.ProjectSettingsTabSnapshot) string {
	if snapshot.ProviderName != "" {
		return "Adjust runtime resources for " + snapshot.ProviderName + "."
	}
	return "Adjust runtime resources for the current sandbox provider."
}

func projectSettingsInspectionDescription(snapshot viewmodel.ProjectSettingsTabSnapshot) string {
	if snapshot.ProviderName != "" {
		return "Open the troubleshooting container for " + snapshot.ProviderName + "."
	}
	return "Open the troubleshooting container launched from the sandbox image."
}

func projectSettingsStatus(status string) string {
	switch status {
	case "idle", "loading", "ready", "error", "unsupported":
		return status
	default:
		return "idle"
	}
}

func projectSettingsResourcesUnsupportedMessage(snapshot viewmodel.ProjectResourcesSnapshot) string {
	if snapshot.Error != "" {
		return snapshot.Error
	}
	return "Resource controls are not available for the current provider."
}

func projectSettingsResourcesErrorMessage(snapshot viewmodel.ProjectResourcesSnapshot) string {
	if snapshot.Error != "" {
		return snapshot.Error
	}
	return "Failed to load resources."
}

func projectSettingsInspectionUnsupportedMessage(snapshot viewmodel.ProjectInspectionSnapshot) string {
	if snapshot.Error != "" {
		return snapshot.Error
	}
	return "Inspection shell access is not available for the current provider."
}

func projectSettingsInspectionErrorMessage(snapshot viewmodel.ProjectInspectionSnapshot) string {
	if snapshot.Error != "" {
		return snapshot.Error
	}
	return "Failed to load inspection shell access."
}

func projectSettingsCPUPlural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func projectSettingsInt(value int) string {
	return strconv.Itoa(value)
}

func projectSettingsCanSave(resources viewmodel.ProjectResourcesSnapshot) bool {
	return projectSettingsStatus(resources.Status) == "ready" && resources.ValidDrafts && !resources.DiskDecrease && resources.Dirty && !resources.SavePending
}

func projectSettingsTerminalSnapshot(snapshot viewmodel.ProjectSettingsTabSnapshot) viewmodel.ProjectInspectionTerminalSnapshot {
	terminal := snapshot.Terminal
	if terminal.ProjectID == "" {
		terminal.ProjectID = "current"
	}
	if terminal.ProviderID == "" {
		terminal.ProviderID = snapshot.ProviderID
	}
	if terminal.Title == "" {
		if snapshot.ProviderName != "" {
			terminal.Title = snapshot.ProviderName + " inspection shell"
		} else {
			terminal.Title = "Inspection shell"
		}
	}
	if terminal.Description == "" {
		if snapshot.ProviderName != "" {
			terminal.Description = "Troubleshooting shell for " + snapshot.ProviderName + "."
		} else {
			terminal.Description = "Troubleshooting shell for the inspection container."
		}
	}
	return terminal
}
