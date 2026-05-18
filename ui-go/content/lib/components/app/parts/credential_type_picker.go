package parts

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func credentialTypePickerGroupCount(groups []viewmodel.CredentialProviderGroup) string {
	count := 0
	for _, group := range groups {
		if len(group.Options) > 0 {
			count++
		}
	}
	return strconv.Itoa(count)
}

func credentialTypeChooseCommand(option viewmodel.CredentialProviderOption) string {
	values := url.Values{}
	values.Set("action", "choose-provider")
	values.Set("provider", option.ID)
	return "@post('/ui/commands/credentials/action?" + values.Encode() + "')"
}

func credentialTypePickerOptionCount(groups []viewmodel.CredentialProviderGroup) string {
	count := 0
	for _, group := range groups {
		count += len(group.Options)
	}
	return strconv.Itoa(count)
}

func credentialTypePickerGroupID(index int, name string) string {
	return "credential-type-group-" + strconv.Itoa(index) + "-" + credentialTypePickerSlug(name)
}

func credentialTypePickerOptionLabel(option viewmodel.CredentialProviderOption) string {
	parts := []string{"Choose", option.Label}
	if option.Description != "" {
		parts = append(parts, option.Description)
	}
	return strings.Join(parts, ": ")
}

func credentialTypePickerSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
