package parts

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

const sessionCommandCustomCredentialOption = "__custom__"

func sessionCommandCredentialsOpen(snapshot viewmodel.SessionCommandCredentialsDialogSnapshot) bool {
	return snapshot.Open && len(snapshot.Requests) > 0
}

func sessionCommandCredentialCommandLabel(snapshot viewmodel.SessionCommandCredentialsDialogSnapshot) string {
	if snapshot.CommandLabel != "" {
		return snapshot.CommandLabel
	}
	return "Command"
}

func sessionCommandCredentialRequestName(request viewmodel.SessionCommandCredentialRequest) string {
	if request.Name != "" {
		return request.Name
	}
	if request.EnvVar != "" {
		return request.EnvVar
	}
	return "Credential"
}

func sessionCommandCredentialSelectedLabel(request viewmodel.SessionCommandCredentialRequest) string {
	if request.SelectedLabel != "" {
		return request.SelectedLabel
	}
	for _, option := range request.Options {
		if option.Value == request.SelectedOption && option.Label != "" {
			return option.Label
		}
	}
	if request.SelectedOption == sessionCommandCustomCredentialOption {
		return "Custom credential"
	}
	return "Choose a credential"
}

func sessionCommandCredentialSelectedKind(request viewmodel.SessionCommandCredentialRequest) string {
	if request.SelectedOption == sessionCommandCustomCredentialOption {
		return "custom"
	}
	for _, option := range request.Options {
		if option.Value == request.SelectedOption {
			return option.Kind
		}
	}
	return ""
}

func sessionCommandCredentialIsCustom(request viewmodel.SessionCommandCredentialRequest) bool {
	return sessionCommandCredentialSelectedKind(request) == "custom"
}

func sessionCommandCredentialIsOAuth(request viewmodel.SessionCommandCredentialRequest) bool {
	return sessionCommandCredentialSelectedKind(request) == "oauth"
}

func sessionCommandCredentialOAuthProvider(request viewmodel.SessionCommandCredentialRequest) string {
	if request.OAuthProviderName != "" {
		return request.OAuthProviderName
	}
	if label := sessionCommandCredentialSelectedLabel(request); label != "" {
		return label
	}
	return "provider"
}

func sessionCommandCredentialValidityPreset(request viewmodel.SessionCommandCredentialRequest) string {
	if request.ValidityPreset != "" {
		return request.ValidityPreset
	}
	return "1_hour"
}

func sessionCommandCredentialValidityPresetLabel(value string) string {
	switch value {
	case "15_minutes":
		return "15 minutes"
	case "1_day":
		return "1 day"
	case "1_week":
		return "1 week"
	case "custom":
		return "Custom"
	default:
		return "1 hour"
	}
}

func sessionCommandCredentialValidityUnit(request viewmodel.SessionCommandCredentialRequest) string {
	if request.ValidityUnit != "" {
		return request.ValidityUnit
	}
	return "hours"
}

func sessionCommandCredentialValidityUnitLabel(value string) string {
	switch value {
	case "days":
		return "Days"
	case "weeks":
		return "Weeks"
	case "never":
		return "Never expires"
	default:
		return "Hours"
	}
}

func sessionCommandCredentialValidityValue(request viewmodel.SessionCommandCredentialRequest) string {
	if request.ValidityValue != "" {
		return request.ValidityValue
	}
	return "1"
}

func sessionCommandCredentialSelectID(request viewmodel.SessionCommandCredentialRequest) string {
	if request.EnvVar != "" {
		return "command-credential-" + request.EnvVar
	}
	return "command-credential"
}

func sessionCommandCredentialValidityID(request viewmodel.SessionCommandCredentialRequest) string {
	if request.EnvVar != "" {
		return "validity-" + request.EnvVar
	}
	return "validity"
}
