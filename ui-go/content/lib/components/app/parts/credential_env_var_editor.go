package parts

import (
	"net/url"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func credentialEnvVarValueType(row viewmodel.CredentialEnvVarRow) string {
	if row.ValueFocused {
		return "text"
	}

	return "password"
}

func credentialEnvVarActionCommand(rowID string, action string) string {
	values := url.Values{}
	values.Set("action", action)
	if strings.TrimSpace(rowID) != "" {
		values.Set("row", rowID)
	}
	return "@post('/ui/commands/credentials/env-var?" + values.Encode() + "')"
}

func credentialEnvVarValuePlaceholder(row viewmodel.CredentialEnvVarRow) string {
	if row.HasStoredValue {
		return "Enter a new value"
	}

	return "value"
}

func credentialEnvVarValueHelp(row viewmodel.CredentialEnvVarRow) string {
	if row.HasStoredValue {
		return "Saving will replace the stored value."
	}

	return "This value will be stored securely."
}

func credentialEnvVarShowInput(row viewmodel.CredentialEnvVarRow) bool {
	return !row.HasStoredValue || row.ReplaceValue
}

func credentialEnvVarInputID(rowID string, field string) string {
	replacer := strings.NewReplacer(" ", "-", "_", "-", ".", "-", "/", "-")
	return "credential-env-var-" + replacer.Replace(rowID) + "-" + field
}
