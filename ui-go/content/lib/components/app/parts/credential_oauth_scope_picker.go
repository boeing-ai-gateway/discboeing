package parts

import (
	"net/url"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func oauthScopePickerLabel(snapshot viewmodel.CredentialOAuthScopePickerSnapshot) string {
	if snapshot.Label != "" {
		return snapshot.Label
	}

	return "Requested scopes"
}

func oauthScopesActionCommand(action string) string {
	values := url.Values{}
	values.Set("action", action)
	return "@post('/ui/commands/credentials/oauth-scopes?" + values.Encode() + "')"
}

func oauthScopesModeCommand(mode string) string {
	values := url.Values{}
	values.Set("action", "mode")
	values.Set("mode", mode)
	return "@post('/ui/commands/credentials/oauth-scopes?" + values.Encode() + "')"
}

func oauthScopesSetEnabledCommand(scope string) string {
	values := url.Values{}
	values.Set("action", "set-enabled")
	values.Set("scope", scope)
	return "@post('/ui/commands/credentials/oauth-scopes?" + values.Encode() + "')"
}

func oauthScopePickerMode(snapshot viewmodel.CredentialOAuthScopePickerSnapshot) string {
	if snapshot.Mode == "advanced" {
		return "advanced"
	}

	return "simple"
}

func oauthScopeModeButtonClass(active bool) string {
	className := "inline-flex h-7 items-center rounded-md px-2 text-xs font-medium shadow-xs disabled:pointer-events-none disabled:opacity-50 "
	if active {
		return className + "bg-primary text-primary-foreground"
	}

	return className + "border border-input bg-background hover:bg-accent hover:text-accent-foreground"
}

func oauthScopeOptionTitle(option viewmodel.CredentialOAuthScopeOption) string {
	if option.SimpleLabel != "" {
		return option.SimpleLabel
	}

	return option.Label
}

func oauthScopeOptionSummary(option viewmodel.CredentialOAuthScopeOption) string {
	if option.SimpleHelpText != "" {
		return option.SimpleHelpText
	}
	if option.Description != "" {
		return option.Description
	}

	return option.Label
}

func oauthScopePickerAdvancedClass(useBulletSummary bool) string {
	className := "max-h-[18rem] space-y-3 overflow-y-auto rounded-md border border-border bg-background p-3"
	if !useBulletSummary {
		return className + " text-sm"
	}

	return className
}

func oauthScopeInputID(value string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-", ".", "-", " ", "-", "_", "-")
	return "oauth-scope-" + replacer.Replace(value)
}

func oauthScopeGroupID(group string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-", ".", "-", " ", "-", "_", "-")
	return "oauth-scope-group-" + replacer.Replace(strings.ToLower(group))
}
