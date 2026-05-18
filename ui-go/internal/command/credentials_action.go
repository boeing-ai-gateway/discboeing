package command

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// CredentialsAction handles server-owned credential list row actions.
func (h *Handler) CredentialsAction(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if action == "" {
		http.Error(w, "missing action", http.StatusBadRequest)
		return
	}

	if err := h.saveCredentialView(r, func(view *viewmodel.ShellSnapshot) error {
		credentials := &view.Header.Settings.Credentials
		ensureCredentialsDefaults(credentials)

		credentials.Error = ""
		switch action {
		case "open-new":
			credentials.EditorOpen = true
			credentials.EditorMode = "create"
			credentials.SelectedProvider = ""
			credentials.EnvVarRows = nil
			credentials.OAuthScopes = viewmodel.CredentialOAuthScopePickerSnapshot{}
		case "close-editor":
			credentials.EditorOpen = false
			credentials.EditorMode = ""
			credentials.SelectedProvider = ""
			credentials.EnvVarRows = nil
			credentials.OAuthScopes = viewmodel.CredentialOAuthScopePickerSnapshot{}
		case "choose-provider":
			providerID := strings.TrimSpace(r.URL.Query().Get("provider"))
			option, ok := credentialProviderOption(credentials.ProviderGroups, providerID)
			if !ok {
				return fmt.Errorf("unknown credential provider %q", providerID)
			}
			credentials.EditorOpen = true
			credentials.EditorMode = "create"
			credentials.SelectedProvider = option.Label
			if option.AuthType == "env" {
				credentials.EnvVarRows = []viewmodel.CredentialEnvVarRow{newCredentialEnvVarRow(credentials.EnvVarRows)}
				credentials.OAuthScopes = viewmodel.CredentialOAuthScopePickerSnapshot{}
				credentials.OAuthWizard = viewmodel.CredentialOAuthWizardSnapshot{}
			} else {
				credentials.EnvVarRows = nil
				credentials.OAuthScopes = defaultCredentialOAuthScopes(option.Label)
				if option.AuthType == "oauth" {
					credentials.OAuthWizard = defaultCredentialOAuthWizard(option.Label, credentials.OAuthScopes)
				}
			}
		case "toggle-inactive":
			index, ok := credentialIndex(credentials.Credentials, id)
			if !ok {
				return fmt.Errorf("credential %q not found", id)
			}
			credentials.Credentials[index].Inactive = !credentials.Credentials[index].Inactive
		case "edit":
			index, ok := credentialIndex(credentials.Credentials, id)
			if !ok {
				return fmt.Errorf("credential %q not found", id)
			}
			credentials.EditorOpen = true
			credentials.EditorMode = "edit"
			credentials.SelectedProvider = credentials.Credentials[index].TypeLabel
			credentials.EnvVarRows = credentialRowsFromKeys(credentials.Credentials[index].EnvKeys)
			if len(credentials.Credentials[index].Scopes) > 0 {
				credentials.OAuthScopes = defaultCredentialOAuthScopes(credentials.Credentials[index].TypeLabel)
			} else {
				credentials.OAuthScopes = viewmodel.CredentialOAuthScopePickerSnapshot{}
			}
		case "delete":
			index, ok := credentialIndex(credentials.Credentials, id)
			if !ok {
				return fmt.Errorf("credential %q not found", id)
			}
			credentials.Credentials = append(credentials.Credentials[:index], credentials.Credentials[index+1:]...)
			if credentials.EditorOpen {
				credentials.EditorOpen = false
				credentials.EditorMode = ""
				credentials.SelectedProvider = ""
			}
		default:
			return fmt.Errorf("unknown credential action %q", action)
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to handle credential action", "id", id, "action", action, "error", err)
		http.Error(w, "failed to update credential", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CredentialEnvVarAction handles server-owned environment-variable editor row controls.
func (h *Handler) CredentialEnvVarAction(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	rowID := strings.TrimSpace(r.URL.Query().Get("row"))
	if action == "" {
		http.Error(w, "missing action", http.StatusBadRequest)
		return
	}

	if err := h.saveCredentialView(r, func(view *viewmodel.ShellSnapshot) error {
		credentials := &view.Header.Settings.Credentials
		ensureCredentialsDefaults(credentials)
		credentials.Error = ""
		switch action {
		case "add-row":
			credentials.EnvVarRows = append(credentials.EnvVarRows, newCredentialEnvVarRow(credentials.EnvVarRows))
		case "show-value-input":
			index, ok := credentialEnvVarRowIndex(credentials.EnvVarRows, rowID)
			if !ok {
				return fmt.Errorf("environment variable row %q not found", rowID)
			}
			credentials.EnvVarRows[index].ReplaceValue = true
			credentials.EnvVarRows[index].ValueFocused = true
		case "hide-value-input":
			index, ok := credentialEnvVarRowIndex(credentials.EnvVarRows, rowID)
			if !ok {
				return fmt.Errorf("environment variable row %q not found", rowID)
			}
			credentials.EnvVarRows[index].ReplaceValue = false
			credentials.EnvVarRows[index].ValueFocused = false
			credentials.EnvVarRows[index].Value = ""
		case "remove-row":
			index, ok := credentialEnvVarRowIndex(credentials.EnvVarRows, rowID)
			if !ok {
				return fmt.Errorf("environment variable row %q not found", rowID)
			}
			if len(credentials.EnvVarRows) <= 1 {
				credentials.EnvVarRows[0] = newCredentialEnvVarRow(nil)
			} else {
				credentials.EnvVarRows = append(credentials.EnvVarRows[:index], credentials.EnvVarRows[index+1:]...)
			}
		default:
			return fmt.Errorf("unknown environment variable action %q", action)
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to handle credential environment variable action", "row", rowID, "action", action, "error", err)
		http.Error(w, "failed to update environment variables", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CredentialOAuthScopesAction handles server-owned OAuth scope picker controls.
func (h *Handler) CredentialOAuthScopesAction(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if action == "" {
		http.Error(w, "missing action", http.StatusBadRequest)
		return
	}

	if err := h.saveCredentialView(r, func(view *viewmodel.ShellSnapshot) error {
		credentials := &view.Header.Settings.Credentials
		ensureCredentialsDefaults(credentials)
		scopes := &credentials.OAuthScopes
		if len(scopes.SimpleOptions) == 0 && len(scopes.AdvancedGroups) == 0 {
			*scopes = defaultCredentialOAuthScopes(credentials.SelectedProvider)
		}
		credentials.Error = ""
		switch action {
		case "reset-defaults":
			resetOAuthScopeDefaults(scopes)
			scopes.Mode = "simple"
		case "mode":
			mode := strings.TrimSpace(r.URL.Query().Get("mode"))
			if mode != "simple" && mode != "advanced" {
				return fmt.Errorf("unknown OAuth scope mode %q", mode)
			}
			scopes.Mode = mode
		case "customize":
			scopes.Mode = "advanced"
		case "set-enabled":
			scope := strings.TrimSpace(r.URL.Query().Get("scope"))
			if scope == "" {
				return fmt.Errorf("missing OAuth scope")
			}
			if !toggleOAuthScope(scopes, scope) {
				return fmt.Errorf("OAuth scope %q not found", scope)
			}
		default:
			return fmt.Errorf("unknown OAuth scope action %q", action)
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to handle credential OAuth scope action", "action", action, "error", err)
		http.Error(w, "failed to update OAuth scopes", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CredentialOAuthWizardAction handles the temporary server-owned OAuth wizard shell.
func (h *Handler) CredentialOAuthWizardAction(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	if action == "" {
		http.Error(w, "missing action", http.StatusBadRequest)
		return
	}

	if err := h.saveCredentialView(r, func(view *viewmodel.ShellSnapshot) error {
		credentials := &view.Header.Settings.Credentials
		ensureCredentialsDefaults(credentials)
		wizard := &credentials.OAuthWizard
		if wizard.ProviderName == "" {
			*wizard = defaultCredentialOAuthWizard(credentials.SelectedProvider, credentials.OAuthScopes)
		}
		wizard.ErrorMessage = ""
		switch action {
		case "select-kind":
			kind := strings.TrimSpace(r.URL.Query().Get("kind"))
			if kind != "authorization_code" && kind != "device_code" {
				return fmt.Errorf("unknown OAuth kind %q", kind)
			}
			wizard.SelectedOAuthKind = kind
		case "open-auth-url":
			wizard.SelectedOAuthKind = "authorization_code"
			wizard.StartingOAuth = false
			wizard.OAuthAuthURL = "https://github.com/login/oauth/authorize?client_id=ui-go-demo&scope=repo"
			wizard.CopiedOAuthAuthURL = false
		case "use-device-code":
			wizard.SelectedOAuthKind = "device_code"
		case "copy-auth-url":
			if wizard.OAuthAuthURL == "" {
				return fmt.Errorf("no OAuth authorization URL to copy")
			}
			wizard.CopiedOAuthAuthURL = true
		case "submit-code":
			wizard.PollingOAuth = false
			wizard.Open = false
		case "start-device":
			wizard.SelectedOAuthKind = "device_code"
			wizard.StartingOAuth = false
			wizard.OAuthVerificationURL = "https://github.com/login/device"
			wizard.OAuthUserCodeDraft = "UI-GO"
			wizard.CopiedOAuthCode = false
		case "open-verification-url":
			if wizard.OAuthVerificationURL == "" {
				return fmt.Errorf("no OAuth verification URL to open")
			}
		case "copy-code":
			if wizard.OAuthUserCodeDraft == "" {
				return fmt.Errorf("no OAuth device code to copy")
			}
			wizard.CopiedOAuthCode = true
		case "start-polling":
			if wizard.OAuthUserCodeDraft == "" {
				return fmt.Errorf("missing OAuth device code")
			}
			wizard.PollingOAuth = !wizard.PollingOAuth
		case "close":
			wizard.Open = false
			wizard.PollingOAuth = false
			wizard.StartingOAuth = false
		default:
			return fmt.Errorf("unknown OAuth wizard action %q", action)
		}
		return nil
	}); err != nil {
		h.logger.Warn("failed to handle credential OAuth wizard action", "action", action, "error", err)
		http.Error(w, "failed to update OAuth wizard", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) saveCredentialView(r *http.Request, update func(*viewmodel.ShellSnapshot) error) error {
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

func credentialIndex(credentials []viewmodel.ConfiguredCredential, id string) (int, bool) {
	for index, credential := range credentials {
		if credential.ID == id {
			return index, true
		}
	}
	return 0, false
}

func ensureCredentialsDefaults(credentials *viewmodel.CredentialsManagerSnapshot) {
	if len(credentials.ProviderGroups) == 0 {
		credentials.ProviderGroups = []viewmodel.CredentialProviderGroup{
			{
				Name: "API keys",
				Options: []viewmodel.CredentialProviderOption{
					{ID: "anthropic", Label: "Anthropic", Description: "Store an Anthropic API key.", Monogram: "A", AuthType: "secret"},
					{ID: "openai", Label: "OpenAI", Description: "Store an OpenAI API key.", Monogram: "O", AuthType: "secret"},
					{ID: "tavily", Label: "Tavily", Description: "Store a Tavily search API key.", Monogram: "T", AuthType: "secret"},
				},
			},
			{
				Name: "OAuth",
				Options: []viewmodel.CredentialProviderOption{
					{ID: "github", Label: "GitHub", Description: "Connect GitHub with OAuth scopes.", Monogram: "G", AuthType: "oauth"},
				},
			},
			{
				Name: "Custom",
				Options: []viewmodel.CredentialProviderOption{
					{ID: "env-vars", Label: "Environment variables", Description: "Create a custom bundle of environment variables.", Monogram: "ENV", AuthType: "env"},
				},
			},
		}
	}
}

func credentialProviderOption(groups []viewmodel.CredentialProviderGroup, id string) (viewmodel.CredentialProviderOption, bool) {
	for _, group := range groups {
		for _, option := range group.Options {
			if option.ID == id {
				return option, true
			}
		}
	}
	return viewmodel.CredentialProviderOption{}, false
}

func newCredentialEnvVarRow(rows []viewmodel.CredentialEnvVarRow) viewmodel.CredentialEnvVarRow {
	maxID := 0
	for _, row := range rows {
		if number, ok := strings.CutPrefix(row.ID, "env-"); ok {
			parsed, err := strconv.Atoi(number)
			if err == nil && parsed > maxID {
				maxID = parsed
			}
		}
	}
	next := maxID + 1
	return viewmodel.CredentialEnvVarRow{
		ID:           "env-" + strconv.Itoa(next),
		Key:          "ENV_VAR_" + strconv.Itoa(next),
		ReplaceValue: true,
		ValueFocused: true,
	}
}

func credentialRowsFromKeys(keys []string) []viewmodel.CredentialEnvVarRow {
	rows := make([]viewmodel.CredentialEnvVarRow, 0, len(keys))
	for index, key := range keys {
		rows = append(rows, viewmodel.CredentialEnvVarRow{
			ID:             "env-" + strconv.Itoa(index+1),
			Key:            key,
			HasStoredValue: true,
		})
	}
	return rows
}

func credentialEnvVarRowIndex(rows []viewmodel.CredentialEnvVarRow, id string) (int, bool) {
	for index, row := range rows {
		if row.ID == id {
			return index, true
		}
	}
	return 0, false
}

func defaultCredentialOAuthScopes(provider string) viewmodel.CredentialOAuthScopePickerSnapshot {
	if provider != "GitHub" {
		return viewmodel.CredentialOAuthScopePickerSnapshot{}
	}
	return viewmodel.CredentialOAuthScopePickerSnapshot{
		Label:            "GitHub scopes",
		Mode:             "default",
		CanResetDefaults: true,
		DefaultOptions: []viewmodel.CredentialOAuthScopeOption{
			{Value: "repo", Label: "Repository access", SimpleLabel: "Repos", Description: "Read and write repository data.", SimpleHelpText: "Read and write repository data.", Enabled: true},
			{Value: "workflow", Label: "Workflow access", SimpleLabel: "Actions", Description: "Read and update GitHub Actions workflows.", SimpleHelpText: "Read and update GitHub Actions workflows.", Enabled: false},
		},
		SimpleOptions: []viewmodel.CredentialOAuthScopeOption{
			{Value: "repo", Label: "Repository access", SimpleLabel: "Repos", Description: "Read and write repository data.", SimpleHelpText: "Read and write repository data.", Enabled: true},
			{Value: "workflow", Label: "Workflow access", SimpleLabel: "Actions", Description: "Read and update GitHub Actions workflows.", SimpleHelpText: "Read and update GitHub Actions workflows.", Enabled: false},
		},
		AdvancedGroups: []viewmodel.CredentialOAuthScopeGroup{
			{
				Group: "Repository",
				Scopes: []viewmodel.CredentialOAuthScopeOption{
					{Value: "repo", Label: "repo", Description: "Full control of private repositories.", Access: "write", Enabled: true},
					{Value: "public_repo", Label: "public_repo", Description: "Access public repositories.", Access: "read", Enabled: false},
				},
			},
			{
				Group: "Automation",
				Scopes: []viewmodel.CredentialOAuthScopeOption{
					{Value: "workflow", Label: "workflow", Description: "Update GitHub Actions workflow files.", Access: "write", Enabled: false},
				},
			},
		},
	}
}

func defaultCredentialOAuthWizard(provider string, scopes viewmodel.CredentialOAuthScopePickerSnapshot) viewmodel.CredentialOAuthWizardSnapshot {
	if provider == "" {
		provider = "OAuth provider"
	}
	if len(scopes.SimpleOptions) == 0 && len(scopes.AdvancedGroups) == 0 {
		scopes = defaultCredentialOAuthScopes(provider)
	}
	return viewmodel.CredentialOAuthWizardSnapshot{
		Open:                     true,
		Title:                    "Connect " + provider,
		ProviderName:             provider,
		OpenVerificationLabel:    "Open verification page",
		WaitingForProviderLabel:  "Waiting for approval…",
		DeviceIntroLine1:         "Request a device code from " + provider + ".",
		DeviceIntroLine2:         "Open the verification page and enter the code.",
		DeviceReturnText:         "Return here after approving access.",
		CloseLabel:               "Close",
		SelectedOAuthKind:        "authorization_code",
		HasScopeOptions:          len(scopes.SimpleOptions) > 0 || len(scopes.AdvancedGroups) > 0,
		OAuthScopePickerSnapshot: scopes,
	}
}

func resetOAuthScopeDefaults(scopes *viewmodel.CredentialOAuthScopePickerSnapshot) {
	defaultEnabled := map[string]bool{}
	for _, option := range scopes.DefaultOptions {
		defaultEnabled[option.Value] = option.Enabled
	}
	for index := range scopes.SimpleOptions {
		scopes.SimpleOptions[index].Enabled = defaultEnabled[scopes.SimpleOptions[index].Value]
	}
	for groupIndex := range scopes.AdvancedGroups {
		for scopeIndex := range scopes.AdvancedGroups[groupIndex].Scopes {
			option := &scopes.AdvancedGroups[groupIndex].Scopes[scopeIndex]
			option.Enabled = defaultEnabled[option.Value]
		}
	}
}

func toggleOAuthScope(scopes *viewmodel.CredentialOAuthScopePickerSnapshot, scope string) bool {
	nextEnabled := false
	found := false
	for index := range scopes.SimpleOptions {
		if scopes.SimpleOptions[index].Value == scope {
			scopes.SimpleOptions[index].Enabled = !scopes.SimpleOptions[index].Enabled
			nextEnabled = scopes.SimpleOptions[index].Enabled
			found = true
			break
		}
	}
	for groupIndex := range scopes.AdvancedGroups {
		for scopeIndex := range scopes.AdvancedGroups[groupIndex].Scopes {
			if scopes.AdvancedGroups[groupIndex].Scopes[scopeIndex].Value == scope {
				if found {
					scopes.AdvancedGroups[groupIndex].Scopes[scopeIndex].Enabled = nextEnabled
				} else {
					scopes.AdvancedGroups[groupIndex].Scopes[scopeIndex].Enabled = !scopes.AdvancedGroups[groupIndex].Scopes[scopeIndex].Enabled
					nextEnabled = scopes.AdvancedGroups[groupIndex].Scopes[scopeIndex].Enabled
					found = true
				}
			}
		}
	}
	for index := range scopes.DefaultOptions {
		if scopes.DefaultOptions[index].Value == scope {
			scopes.DefaultOptions[index].Enabled = nextEnabled
		}
	}
	return found
}
