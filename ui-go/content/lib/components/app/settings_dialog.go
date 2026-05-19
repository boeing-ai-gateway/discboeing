package app

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func settingsCloseCommand() string {
	return "@post('" + settingsCloseURL() + "')"
}

func settingsCloseURL() string {
	return "/ui/commands/settings/action?action=close"
}

func settingsTabCommand(tab string) string {
	return "@post('/ui/commands/settings/action?action=tab&tab=" + url.QueryEscape(tab) + "')"
}

func settingsRecentLimitCommand(limit int) string {
	return "@post('/ui/commands/settings/action?action=recent-limit&limit=" + strconv.Itoa(limit) + "')"
}

func settingsToggleCommand(setting string) string {
	return "@post('/ui/commands/settings/action?action=toggle&setting=" + url.QueryEscape(setting) + "')"
}

func settingsSupportOpenCommand() string {
	return "@post('/ui/commands/settings/action?action=support-open')"
}

func settingsThemeSignals(snapshot viewmodel.SettingsDialogSnapshot) string {
	mode := snapshot.Theme
	if mode != "light" && mode != "dark" && mode != "system" {
		mode = "system"
	}
	resolved := snapshot.ResolvedTheme
	if resolved != "light" && resolved != "dark" {
		resolved = "dark"
	}
	scheme := snapshot.ColorScheme
	if scheme == "" {
		scheme = "default"
	}
	return fmt.Sprintf(
		"{_themeMode: %s, _themeResolved: %s, _themeLightColorScheme: %s, _themeDarkColorScheme: %s}",
		strconv.Quote(mode),
		strconv.Quote(resolved),
		strconv.Quote(scheme),
		strconv.Quote(scheme),
	)
}

func settingsThemeInitExpression() string {
	return "$_themeMode = window.uiGoThemeMode(); $_themeResolved = window.uiGoThemeResolve($_themeMode); $_themeLightColorScheme = window.uiGoThemeColorScheme('light'); $_themeDarkColorScheme = window.uiGoThemeColorScheme('dark'); window.uiGoThemeApply($_themeMode, $_themeResolved === 'dark' ? $_themeDarkColorScheme : $_themeLightColorScheme)"
}

func settingsThemeSystemChangeExpression() string {
	return "if ($_themeMode === 'system') { $_themeResolved = window.uiGoThemeResolve($_themeMode); window.uiGoThemeApply($_themeMode, $_themeResolved === 'dark' ? $_themeDarkColorScheme : $_themeLightColorScheme) }"
}

func settingsThemeModeButtonClass() string {
	return "rounded-full border px-3 py-1 text-sm capitalize"
}

func settingsThemeModeButtonClassExpression(mode string) string {
	quoted := strconv.Quote(mode)
	return "{'border-primary bg-primary text-primary-foreground shadow-sm': $_themeMode === " + quoted + ", 'border-transparent bg-transparent text-muted-foreground': $_themeMode !== " + quoted + "}"
}

func settingsThemeModeClickExpression(mode string) string {
	quoted := strconv.Quote(mode)
	return "$_themeMode = " + quoted + "; $_themeResolved = window.uiGoThemeResolve($_themeMode); window.uiGoThemeApply($_themeMode, $_themeResolved === 'dark' ? $_themeDarkColorScheme : $_themeLightColorScheme)"
}

func settingsThemeSelectClass() string {
	return "h-9 w-56 rounded-md border border-input bg-background px-3 text-sm"
}

func settingsThemeSelectClassExpression(mode string) string {
	quoted := strconv.Quote(mode)
	return "{'hidden': $_themeResolved !== " + quoted + "}"
}

func settingsThemeSelectChangeExpression(mode string) string {
	quoted := strconv.Quote(mode)
	return "if ($_themeResolved === " + quoted + ") { window.uiGoThemeApply($_themeMode, $_themeResolved === 'dark' ? $_themeDarkColorScheme : $_themeLightColorScheme) }"
}

func settingsTabActive(snapshot viewmodel.SettingsDialogSnapshot, tab string) bool {
	return settingsActiveTab(snapshot) == tab
}

func settingsActiveTab(snapshot viewmodel.SettingsDialogSnapshot) string {
	if snapshot.ActiveTab != "" {
		return snapshot.ActiveTab
	}
	return "appearance"
}

func settingsTabClass(active bool) string {
	base := "inline-flex h-9 items-center justify-center rounded-md px-3 text-sm font-medium"
	if active {
		return base + " bg-background text-foreground shadow-xs"
	}
	return base + " text-muted-foreground hover:text-foreground"
}

func settingsPanelClass(snapshot viewmodel.SettingsDialogSnapshot, tab string) string {
	if settingsTabActive(snapshot, tab) {
		return "mt-0 h-full"
	}
	return "hidden"
}

func settingsUpdateProgress(update viewmodel.UpdateSnapshot) int {
	if update.TotalBytes <= 0 {
		return 0
	}
	pct := int((update.DownloadedBytes * 100) / update.TotalBytes)
	if pct > 100 {
		return 100
	}
	return pct
}

func settingsFormatBytes(bytes int64) string {
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

func settingsUpdateMessage(update viewmodel.UpdateSnapshot) string {
	switch update.Status {
	case "ready":
		if update.IsIgnored {
			return "Version " + update.AvailableVersion + " available (ignored)."
		}
		return "Version " + update.AvailableVersion + " is ready to install."
	case "checking":
		return "Checking for updates..."
	case "downloading":
		if update.TotalBytes > 0 {
			return "Downloading update... " + settingsFormatBytes(update.DownloadedBytes) + " / " + settingsFormatBytes(update.TotalBytes)
		}
		return "Downloading update... " + settingsFormatBytes(update.DownloadedBytes)
	case "installing":
		return "Installing update..."
	case "error":
		return "Update failed: " + update.Error
	default:
		return "You're on the latest version."
	}
}

func settingsUpdateCheckIconClass(update viewmodel.UpdateSnapshot) string {
	if update.Status == "checking" {
		return "size-3.5 animate-spin"
	}
	return "size-3.5"
}

func settingsTabID(tab string) string {
	return "settings-tab-" + tab
}

func settingsPanelID(tab string) string {
	return "settings-panel-" + tab
}

func settingsSwitchKey(title string) string {
	key := strings.ToLower(strings.TrimSpace(title))
	key = strings.NewReplacer(" ", "-", "/", "-", ".", "", ":", "").Replace(key)
	return key
}

func settingsModelProviders(models []viewmodel.ModelOption) []string {
	seen := map[string]bool{}
	providers := []string{}
	for _, model := range models {
		provider := strings.TrimSpace(model.Provider)
		if provider == "" {
			provider = "Other"
		}
		if !seen[provider] {
			seen[provider] = true
			providers = append(providers, provider)
		}
	}
	return providers
}
