package app

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func credentialManagerTitle(credential viewmodel.ConfiguredCredential) string {
	if strings.TrimSpace(credential.Name) != "" {
		return credential.Name
	}
	if strings.TrimSpace(credential.TypeLabel) != "" {
		return credential.TypeLabel
	}
	if strings.TrimSpace(credential.ID) != "" {
		return credential.ID
	}
	return "Credential"
}

func credentialsActionCommand(id string, action string) string {
	values := url.Values{}
	values.Set("action", action)
	if strings.TrimSpace(id) != "" {
		values.Set("id", id)
	}
	return "@post('/ui/commands/credentials/action?" + values.Encode() + "')"
}

func credentialManagerSubtitle(credential viewmodel.ConfiguredCredential) string {
	parts := []string{}
	if credential.TypeLabel != "" {
		parts = append(parts, credential.TypeLabel)
	}
	if credential.Inactive {
		parts = append(parts, "inactive")
	} else {
		parts = append(parts, credentialManagerVisibilitySummary(credential.Visibility))
	}
	if credential.Summary != "" {
		parts = append(parts, credential.Summary)
	} else if len(credential.Scopes) > 0 {
		parts = append(parts, "OAuth · "+strings.Join(credential.Scopes, ", "))
	} else if len(credential.EnvKeys) > 0 {
		parts = append(parts, strings.Join(credential.EnvKeys, " · "))
	}
	return strings.Join(parts, " · ")
}

func credentialManagerVisibilitySummary(visibility viewmodel.CredentialVisibility) string {
	contexts := []string{}
	if visibility.Tools {
		contexts = append(contexts, "tools")
	}
	if visibility.Console {
		contexts = append(contexts, "console")
	}
	if visibility.Services {
		contexts = append(contexts, "services")
	}
	if visibility.Hooks {
		contexts = append(contexts, "hooks")
	}
	if len(contexts) == 0 {
		return "internal only"
	}
	return strings.Join(contexts, ", ")
}

func credentialManagerMonogram(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "?"
	}
	return strings.ToUpper(value[:1])
}

func credentialManagerDialogTitle(snapshot viewmodel.CredentialsManagerSnapshot) string {
	if snapshot.EditorMode == "edit" {
		return "Edit credential"
	}
	if snapshot.SelectedProvider != "" {
		return "New " + snapshot.SelectedProvider
	}
	return "New credential"
}

func credentialManagerHasProviders(snapshot viewmodel.CredentialsManagerSnapshot) bool {
	for _, group := range snapshot.ProviderGroups {
		if len(group.Options) > 0 {
			return true
		}
	}
	return false
}

func credentialManagerHasOAuthScopes(snapshot viewmodel.CredentialsManagerSnapshot) bool {
	return len(snapshot.OAuthScopes.SimpleOptions) > 0 ||
		len(snapshot.OAuthScopes.DefaultOptions) > 0 ||
		len(snapshot.OAuthScopes.AdvancedGroups) > 0
}

func credentialManagerCountLabel(count int) string {
	if count == 1 {
		return "1 credential"
	}
	return strconv.Itoa(count) + " credentials"
}
