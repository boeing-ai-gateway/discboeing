package parts

import (
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func servicePanelActive(snapshot viewmodel.DockPanelSnapshot) viewmodel.DockService {
	services := snapshot.VisibleServices
	if len(services) == 0 {
		return viewmodel.DockService{}
	}
	for _, service := range services {
		if service.ID == snapshot.ActiveServiceID {
			return service
		}
	}
	return services[0]
}

func servicePanelLabel(service viewmodel.DockService) string {
	if service.Label != "" {
		return service.Label
	}
	if service.Name != "" {
		return service.Name
	}
	if service.ID != "" {
		return service.ID
	}
	return "Service"
}

func servicePanelStatus(service viewmodel.DockService) string {
	if service.Status != "" {
		return strings.ToLower(service.Status)
	}
	return "stopped"
}

func servicePanelHasHTTP(service viewmodel.DockService) bool {
	return service.URL != "" || service.HTTPPort > 0 || service.HTTPSPort > 0
}

func servicePanelBaseURL(snapshot viewmodel.DockPanelSnapshot, service viewmodel.DockService) string {
	if service.URL != "" {
		return service.URL
	}
	if !servicePanelHasHTTP(service) || snapshot.SessionID == "" || service.ID == "" {
		return ""
	}
	protocol := "http"
	if service.HTTPSPort > 0 {
		protocol = "https"
	}
	return protocol + "://" + snapshot.SessionID + "-svc-" + service.ID
}

func servicePanelPath(service viewmodel.DockService) string {
	path := strings.TrimSpace(service.URLPath)
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func servicePanelURL(snapshot viewmodel.DockPanelSnapshot, service viewmodel.DockService) string {
	base := servicePanelBaseURL(snapshot, service)
	if base == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + servicePanelPath(service)
}

func servicePanelPassive(service viewmodel.DockService) bool {
	return service.Passive
}

func servicePanelRunnable(service viewmodel.DockService) bool {
	return service.ID != "" && !service.Passive
}

func servicePanelRunning(service viewmodel.DockService) bool {
	return servicePanelStatus(service) == "running"
}

func servicePanelBusy(service viewmodel.DockService) bool {
	switch servicePanelStatus(service) {
	case "starting", "stopping":
		return true
	default:
		return false
	}
}

func servicePanelCanStart(service viewmodel.DockService) bool {
	return servicePanelRunnable(service) && servicePanelStatus(service) == "stopped"
}

func servicePanelCanStop(service viewmodel.DockService) bool {
	return servicePanelRunnable(service) && servicePanelStatus(service) == "running"
}

func servicePanelCanRestart(service viewmodel.DockService) bool {
	return servicePanelCanStop(service)
}

func servicePanelShowPreview(snapshot viewmodel.DockPanelSnapshot, service viewmodel.DockService) bool {
	mode := snapshot.Services.ViewMode
	return servicePanelHasHTTP(service) && (servicePanelPassive(service) || mode == "" || mode == "preview")
}

func servicePanelShowTabs(service viewmodel.DockService) bool {
	return servicePanelHasHTTP(service) && !servicePanelPassive(service)
}

func servicePanelActionLabel(service viewmodel.DockService) string {
	switch servicePanelStatus(service) {
	case "starting":
		return "Starting"
	case "stopping":
		return "Stopping"
	case "running":
		return "Stop"
	default:
		return "Start"
	}
}

func servicePanelStatusLabel(service viewmodel.DockService) string {
	if service.ID == "" {
		return "No services"
	}
	if service.Passive {
		return "External service"
	}
	switch servicePanelStatus(service) {
	case "starting":
		return "Starting"
	case "stopping":
		return "Stopping"
	case "running":
		return "Running"
	case "stopped":
		if service.ExitCode != nil && *service.ExitCode != 0 {
			return "Stopped (exit " + strconv.Itoa(*service.ExitCode) + ")"
		}
	}
	return "Stopped"
}

func servicePanelStatusTextClass(service viewmodel.DockService) string {
	base := "flex items-center gap-1 text-[11px]"
	if service.ID == "" {
		return base + " text-muted-foreground"
	}
	if service.Passive || servicePanelStatus(service) == "running" {
		return base + " text-green-500"
	}
	if servicePanelBusy(service) {
		return base + " text-yellow-500"
	}
	if service.ExitCode != nil && *service.ExitCode != 0 {
		return base + " text-red-500"
	}
	return base + " text-muted-foreground"
}

func servicePanelDotClass(service viewmodel.DockService) string {
	base := "size-2 shrink-0 rounded-full"
	if service.Passive {
		return base + " bg-green-500"
	}
	switch servicePanelStatus(service) {
	case "running":
		return base + " bg-green-500"
	case "starting", "stopping":
		return base + " bg-yellow-500"
	case "stopped":
		if service.ExitCode != nil && *service.ExitCode != 0 {
			return base + " bg-red-500"
		}
	}
	return base + " bg-sidebar-foreground/30"
}

func servicePanelTabClass(active bool) string {
	base := "flex shrink-0 items-center gap-2 rounded-md border px-3 py-1.5 text-sm transition"
	if active {
		return base + " border-sidebar-border bg-background text-foreground shadow-sm"
	}
	return base + " border-transparent bg-sidebar-accent/60 text-sidebar-foreground/75 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
}

func servicePanelViewButtonClass(active bool) string {
	base := "inline-flex h-6 items-center gap-1 border-r border-border px-2 text-[11px] text-sidebar-foreground/70 last:border-r-0 hover:text-sidebar-foreground"
	if active {
		return base + " bg-background text-foreground"
	}
	return base
}

func servicePanelViewportButtonClass(active bool) string {
	base := "inline-flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground"
	if active {
		return base + " bg-secondary text-secondary-foreground"
	}
	return base
}

func servicePanelFrameWidth(snapshot viewmodel.DockPanelSnapshot) string {
	switch snapshot.Services.Viewport {
	case "mobile":
		return "390px"
	case "tablet":
		return "768px"
	default:
		return ""
	}
}

func servicePanelPreviewShellClass(snapshot viewmodel.DockPanelSnapshot) string {
	base := "relative flex h-full min-h-0 flex-1"
	if servicePanelFrameWidth(snapshot) != "" {
		return base + " overflow-auto justify-center bg-muted/30"
	}
	return base
}

func servicePanelFrameStyle(snapshot viewmodel.DockPanelSnapshot) string {
	if width := servicePanelFrameWidth(snapshot); width != "" {
		return "width: " + width + ";"
	}
	return "width: 100%;"
}

func servicePanelLogClass(event viewmodel.ServiceLogEvent) string {
	base := "break-all whitespace-pre-wrap"
	switch event.Type {
	case "stderr":
		return base + " text-red-400"
	case "exit":
		return base + " mt-2 font-semibold text-yellow-400"
	case "error":
		return base + " font-semibold text-red-500"
	default:
		return base
	}
}

func servicePanelMaximizeTitle(snapshot viewmodel.DockPanelSnapshot) string {
	if snapshot.DockMaximized {
		return "Restore split view"
	}
	return "Maximize service panel"
}
