package parts

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func vscodePanelService(snapshot viewmodel.DockPanelSnapshot) viewmodel.DockService {
	if snapshot.VSCode.Service.ID != "" {
		return snapshot.VSCode.Service
	}
	for _, service := range snapshot.VisibleServices {
		name := strings.ToLower(service.ID + " " + service.Name + " " + service.Label)
		if strings.Contains(name, "vscode") || strings.Contains(name, "code-server") || strings.Contains(name, "editor") {
			return service
		}
	}
	return viewmodel.DockService{}
}

func vscodePanelMaximizeTitle(snapshot viewmodel.DockPanelSnapshot) string {
	if snapshot.DockMaximized {
		return "Restore split view"
	}
	return "Maximize editor panel"
}

func vscodePanelPath(service viewmodel.DockService) string {
	path := strings.TrimSpace(service.URLPath)
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func vscodePanelBaseURL(snapshot viewmodel.DockPanelSnapshot, service viewmodel.DockService) string {
	if service.URL != "" {
		return service.URL
	}
	if service.ID == "" || snapshot.SessionID == "" {
		return ""
	}
	protocol := "http"
	if service.HTTPSPort > 0 {
		protocol = "https"
	}
	return protocol + "://" + snapshot.SessionID + "-svc-" + service.ID
}

func vscodePanelURL(snapshot viewmodel.DockPanelSnapshot, service viewmodel.DockService) string {
	base := vscodePanelBaseURL(snapshot, service)
	if base == "" {
		return ""
	}
	url := strings.TrimRight(base, "/") + vscodePanelPath(service)
	if snapshot.VSCode.AuthToken == "" {
		return url
	}
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return url + separator + "token=" + snapshot.VSCode.AuthToken
}

func vscodePanelLoading(snapshot viewmodel.DockPanelSnapshot) bool {
	return snapshot.VSCode.Loading
}
