package parts

import (
	"net/url"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func oauthWizardActionCommand(action string) string {
	values := url.Values{}
	values.Set("action", action)
	return "@post('/ui/commands/credentials/oauth-wizard?" + values.Encode() + "')"
}

func oauthWizardSelectKindCommand(kind string) string {
	values := url.Values{}
	values.Set("action", "select-kind")
	values.Set("kind", kind)
	return "@post('/ui/commands/credentials/oauth-wizard?" + values.Encode() + "')"
}

func oauthWizardCloseLabel(snapshot viewmodel.CredentialOAuthWizardSnapshot) string {
	if snapshot.CloseLabel != "" {
		return snapshot.CloseLabel
	}

	return "Close"
}

func oauthWizardProviderName(snapshot viewmodel.CredentialOAuthWizardSnapshot) string {
	if snapshot.ProviderName != "" {
		return snapshot.ProviderName
	}

	return "provider"
}

func oauthWizardKind(snapshot viewmodel.CredentialOAuthWizardSnapshot) string {
	if snapshot.SelectedOAuthKind == "device_code" {
		return "device_code"
	}

	return "authorization_code"
}

func oauthWizardFlowButtonClass(active bool) string {
	className := "rounded-lg border p-4 text-left transition-colors "
	if active {
		return className + "border-primary bg-primary/5"
	}

	return className + "border-border hover:bg-muted/60"
}

func oauthWizardStepNumber(hasScopeOptions bool) string {
	if hasScopeOptions {
		return "3."
	}

	return "2."
}

func oauthWizardOpenSignOnLabel(starting bool) string {
	if starting {
		return "Preparing…"
	}

	return "Open Sign-On Link"
}

func oauthWizardDeviceStartLabel(starting bool) string {
	if starting {
		return "Starting…"
	}

	return "Get device code"
}

func oauthWizardConnectLabel(polling bool) string {
	if polling {
		return "Connecting…"
	}

	return "Connect with pasted code"
}

func oauthWizardPollingLabel(snapshot viewmodel.CredentialOAuthWizardSnapshot) string {
	if snapshot.PollingOAuth && snapshot.WaitingForProviderLabel != "" {
		return snapshot.WaitingForProviderLabel
	}

	return "I entered the code"
}
