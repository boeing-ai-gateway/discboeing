package command

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// SettingsAction handles server-owned global settings dialog actions.
func (h *Handler) SettingsAction(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if action == "" {
		http.Error(w, "missing action", http.StatusBadRequest)
		return
	}

	if err := h.saveSettingsView(r, func(view *viewmodel.ShellSnapshot) error {
		settings := &view.Header.Settings
		ensureSettingsDefaults(settings, view.Header.ShowRefreshButton)

		switch action {
		case "open":
			settings.Open = true
			if !validSettingsTab(settings.ActiveTab, settings.ShowUpdateTab) {
				settings.ActiveTab = "appearance"
			}
		case "close":
			settings.Open = false
		case "tab":
			tab := strings.TrimSpace(r.URL.Query().Get("tab"))
			if !validSettingsTab(tab, settings.ShowUpdateTab) {
				return fmt.Errorf("unknown settings tab %q", tab)
			}
			settings.ActiveTab = tab
		case "recent-limit":
			limit, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
			if err != nil || (limit != 1 && limit != 3 && limit != 5 && limit != 10) {
				return fmt.Errorf("unknown recent limit %q", r.URL.Query().Get("limit"))
			}
			settings.RecentThreadsLimit = limit
			view.Sidebar.ShowRecentThreads = limit > 1 && len(view.Sidebar.RecentThreads) > 0
			view.Sidebar.ShowAllHeader = view.Sidebar.ShowRecentThreads
		case "toggle":
			setting := strings.TrimSpace(r.URL.Query().Get("setting"))
			switch setting {
			case "show-refresh", "show-refresh-button":
				settings.ShowRefreshButton = !settings.ShowRefreshButton
				view.Header.ShowRefreshButton = settings.ShowRefreshButton
			case "show-editor", "show-editor-button":
				settings.ShowEditorButton = !settings.ShowEditorButton
			case "full-width", "full-width-conversation":
				if settings.ChatWidthMode == "full" {
					settings.ChatWidthMode = "constrained"
				} else {
					settings.ChatWidthMode = "full"
				}
			case "auto-scroll":
				settings.AutoScrollOnStream = !settings.AutoScrollOnStream
			default:
				return fmt.Errorf("unknown settings toggle %q", setting)
			}
		case "support-open":
			settings.SupportInfo.Open = true
			settings.SupportInfo.Status = "ready"
			settings.SupportInfo.JSON = "{\n  \"app\": \"ui-go\",\n  \"runtime\": \"server-owned settings dialog\",\n  \"status\": \"ready\"\n}"
			settings.SupportInfo.Error = ""
		case "support-close":
			settings.SupportInfo.Open = false
		default:
			return fmt.Errorf("unknown settings action %q", action)
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to handle settings action", "action", action, "error", err)
		http.Error(w, "failed to update settings", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) saveSettingsView(r *http.Request, update func(*viewmodel.ShellSnapshot) error) error {
	session, ok := h.session(r)
	if !ok {
		return fmt.Errorf("missing session")
	}
	var updateErr error
	session.Save(func(view *viewmodel.ShellSnapshot) {
		updateErr = update(view)
	})
	return updateErr
}

func ensureSettingsDefaults(settings *viewmodel.SettingsDialogSnapshot, showRefreshButton bool) {
	if settings.ActiveTab == "" {
		settings.ActiveTab = "appearance"
	}
	if settings.Theme == "" {
		settings.Theme = "system"
	}
	if settings.ResolvedTheme == "" {
		settings.ResolvedTheme = resolvedTheme(settings.Theme)
	}
	if settings.ColorScheme == "" {
		settings.ColorScheme = "default"
	}
	if settings.ActiveThemeName == "" {
		settings.ActiveThemeName = "Default"
	}
	if len(settings.AvailableThemes) == 0 {
		settings.AvailableThemes = []viewmodel.ThemeOption{
			{ID: "default", Name: "Default", Mode: "system"},
			{ID: "flexoki", Name: "Flexoki", Mode: "system"},
			{ID: "nord", Name: "Nord", Mode: "dark"},
			{ID: "tokyo-night", Name: "Tokyo Night", Mode: "dark"},
			{ID: "solarized", Name: "Solarized", Mode: "light"},
			{ID: "dracula", Name: "Dracula", Mode: "dark"},
			{ID: "catppuccin-mocha", Name: "Catppuccin Mocha", Mode: "dark"},
			{ID: "catppuccin-macchiato", Name: "Catppuccin Macchiato", Mode: "dark"},
			{ID: "catppuccin-frappe", Name: "Catppuccin Frappé", Mode: "dark"},
			{ID: "alucard", Name: "Alucard", Mode: "light"},
			{ID: "catppuccin-latte", Name: "Catppuccin Latte", Mode: "light"},
		}
	}
	if settings.RecentThreadsLimit == 0 {
		settings.RecentThreadsLimit = 5
	}
	settings.ShowRefreshButton = showRefreshButton
	if settings.ChatWidthMode == "" {
		settings.ChatWidthMode = "constrained"
	}
	if len(settings.Models) == 0 {
		settings.Models = []viewmodel.ModelOption{{ID: "fake-model", Name: "Fake Model", Provider: "fake"}}
	}
	ensureCredentialsDefaults(&settings.Credentials)
}

func validSettingsTab(tab string, showUpdateTab bool) bool {
	switch tab {
	case "appearance", "chat", "providers", "credentials":
		return true
	case "update":
		return showUpdateTab
	default:
		return false
	}
}

func resolvedTheme(theme string) string {
	if theme == "light" || theme == "dark" {
		return theme
	}
	return "system"
}
