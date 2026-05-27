package client

// CredentialVisibility controls where a credential may be exposed.
type CredentialVisibility struct {
	Tools    bool `json:"tools"`
	Console  bool `json:"console"`
	Services bool `json:"services"`
	Hooks    bool `json:"hooks"`
}

// CredentialEnvVar is an environment variable stored with a custom credential.
type CredentialEnvVar struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	OriginalKey string `json:"originalKey,omitempty"`
}

// CreateCredentialRequest is the request body for creating or updating a credential.
type CreateCredentialRequest struct {
	Provider     string                `json:"provider,omitempty"`
	CredentialID string                `json:"credentialId,omitempty"`
	Name         string                `json:"name"`
	Description  string                `json:"description,omitempty"`
	AuthType     string                `json:"authType"` // "api_key", "id", or "oauth"
	APIKey       string                `json:"apiKey,omitempty"`
	EnvVars      []CredentialEnvVar    `json:"envVars,omitempty"`
	AgentVisible *bool                 `json:"agentVisible,omitempty"`
	Visibility   *CredentialVisibility `json:"visibility,omitempty"`
	Inactive     *bool                 `json:"inactive,omitempty"`
}

// AnthropicExchangeRequest is the request for exchanging an Anthropic OAuth code.
type AnthropicExchangeRequest struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"verifier"`
}

// GitHubCopilotDeviceCodeRequest is the request for initiating device flow.
type GitHubCopilotDeviceCodeRequest struct {
	DeploymentType string `json:"deploymentType"` // "github.com" or "enterprise"
	EnterpriseURL  string `json:"enterpriseUrl,omitempty"`
}

// GitHubCopilotPollRequest is the request for polling device authorization.
type GitHubCopilotPollRequest struct {
	DeviceCode string `json:"deviceCode"`
	Domain     string `json:"domain"`
}

// GitHubCopilotDeviceCodeResponse is the device-code response for GitHub flows.
type GitHubCopilotDeviceCodeResponse struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
	Domain          string `json:"domain"`
}

// GitHubCopilotPollResponse is the response for poll requests.
type GitHubCopilotPollResponse struct {
	Status string `json:"status"` // "pending", "success", or "error"
	Error  string `json:"error,omitempty"`
}

// GitHubDeviceCodeRequest is the request for initiating GitHub device flow.
type GitHubDeviceCodeRequest struct {
	EnterpriseURL string   `json:"enterpriseUrl,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
}

// GitHubAuthorizeRequest is the request for starting GitHub authorization-code OAuth.
type GitHubAuthorizeRequest struct {
	EnterpriseURL string                `json:"enterpriseUrl,omitempty"`
	RedirectURI   string                `json:"redirectUri,omitempty"`
	Scopes        []string              `json:"scopes,omitempty"`
	CredentialID  string                `json:"credentialId,omitempty"`
	Name          string                `json:"name,omitempty"`
	Description   string                `json:"description,omitempty"`
	Visibility    *CredentialVisibility `json:"visibility,omitempty"`
	Inactive      *bool                 `json:"inactive,omitempty"`
}

// GitHubAuthorizeResponse is returned after starting GitHub authorization-code OAuth.
type GitHubAuthorizeResponse struct {
	URL               string `json:"url"`
	Verifier          string `json:"verifier"`
	State             string `json:"state"`
	RedirectURI       string `json:"redirectUri"`
	CallbackListening bool   `json:"callbackListening"`
}

// GitHubPollRequest is the request for polling GitHub device authorization.
type GitHubPollRequest struct {
	DeviceCode   string                `json:"deviceCode"`
	Domain       string                `json:"domain"`
	CredentialID string                `json:"credentialId,omitempty"`
	Name         string                `json:"name,omitempty"`
	Description  string                `json:"description,omitempty"`
	Visibility   *CredentialVisibility `json:"visibility,omitempty"`
	Inactive     *bool                 `json:"inactive,omitempty"`
}

// GitHubExchangeRequest is the request for exchanging a GitHub OAuth code.
type GitHubExchangeRequest struct {
	Code          string                `json:"code"`
	RedirectURI   string                `json:"redirectUri,omitempty"`
	CodeVerifier  string                `json:"verifier"`
	EnterpriseURL string                `json:"enterpriseUrl,omitempty"`
	CredentialID  string                `json:"credentialId,omitempty"`
	Name          string                `json:"name,omitempty"`
	Description   string                `json:"description,omitempty"`
	Visibility    *CredentialVisibility `json:"visibility,omitempty"`
	Inactive      *bool                 `json:"inactive,omitempty"`
}

// GitHubExchangeResponse is returned after a GitHub OAuth exchange.
type GitHubExchangeResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// GitHubCallbackStatusRequest is the request for checking GitHub callback status.
type GitHubCallbackStatusRequest struct {
	State string `json:"state"`
}

// CodexDeviceCodeResponse is returned after starting Codex device flow.
type CodexDeviceCodeResponse struct {
	DeviceAuthID    string `json:"deviceAuthId"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	Interval        int    `json:"interval"`
}

// CodexAuthorizeRequest is the request for starting Codex authorization-code OAuth.
type CodexAuthorizeRequest struct {
	RedirectURI string `json:"redirectUri"`
}

// CodexAuthorizeResponse is returned after starting Codex authorization-code OAuth.
type CodexAuthorizeResponse struct {
	URL               string `json:"url"`
	Verifier          string `json:"verifier"`
	State             string `json:"state"`
	RedirectURI       string `json:"redirectUri"`
	CallbackListening bool   `json:"callbackListening"`
}

// CodexExchangeRequest is the request for exchanging a Codex OAuth code.
type CodexExchangeRequest struct {
	Code         string `json:"code"`
	RedirectURI  string `json:"redirectUri"`
	CodeVerifier string `json:"verifier"`
}

// CodexExchangeResponse is returned after a Codex OAuth exchange.
type CodexExchangeResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// CodexPollRequest is the request for polling Codex device flow.
type CodexPollRequest struct {
	DeviceAuthID string `json:"deviceAuthId"`
	UserCode     string `json:"userCode"`
}

// CodexCallbackStatusRequest is the request for checking Codex callback status.
type CodexCallbackStatusRequest struct {
	State string `json:"state"`
}
