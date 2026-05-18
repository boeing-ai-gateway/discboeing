package parts

import (
	"net/url"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func credentialListItemCommand(id string, action string) string {
	return "@post('/ui/commands/credentials/action?id=" + url.QueryEscape(id) + "&action=" + url.QueryEscape(action) + "')"
}

func credentialListItemToggleLabel(credential viewmodel.ConfiguredCredential, title string) string {
	if credential.Inactive {
		return "Enable " + title
	}

	return "Disable " + title
}

func credentialListItemToggleTooltip(credential viewmodel.ConfiguredCredential, title string) string {
	if credential.Toggling {
		return "Updating " + title
	}

	return credentialListItemToggleLabel(credential, title)
}

func credentialListItemSwitchClass(credential viewmodel.ConfiguredCredential) string {
	className := "relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-transparent transition-colors disabled:cursor-not-allowed disabled:opacity-50 "
	if credential.Inactive {
		return className + "bg-input"
	}

	return className + "bg-primary"
}

func credentialListItemSwitchThumbClass(credential viewmodel.ConfiguredCredential) string {
	className := "pointer-events-none block size-4 rounded-full bg-background shadow-lg ring-0 transition-transform "
	if credential.Inactive {
		return className + "translate-x-0.5"
	}

	return className + "translate-x-4"
}
