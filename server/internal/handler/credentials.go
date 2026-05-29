package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/server/internal/keyvalidator"
	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/oauth"
	"github.com/obot-platform/discobot/server/internal/providers"
	"github.com/obot-platform/discobot/server/internal/service"
)

// GetCredentialTypes returns the credential choices used by the current UI.
func (h *Handler) GetCredentialTypes(w http.ResponseWriter, _ *http.Request) {
	h.JSON(w, http.StatusOK, map[string]any{"credentialTypes": providers.GetCredentialTypes()})
}

func credentialVisibilityFromAPI(visibility api.CredentialVisibility) service.CredentialVisibility {
	return service.CredentialVisibility{
		Tools:    visibility.Tools,
		Console:  visibility.Console,
		Services: visibility.Services,
		Hooks:    visibility.Hooks,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringSliceValue(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}

// ListCredentials returns all credentials for a project (safe info only)
func (h *Handler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	credentials, err := h.credentialService.List(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list credentials")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"credentials": credentials})
}

// CreateCredential creates or updates a credential
func (h *Handler) CreateCredential(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req api.CreateCredentialRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name != "" {
		req.Name = strings.TrimSpace(req.Name)
	}
	credentialID := ""
	if req.CredentialId != nil {
		credentialID = *req.CredentialId
	}
	provider := ""
	if req.Provider != nil {
		provider = *req.Provider
	}
	apiKey := ""
	if req.ApiKey != nil {
		apiKey = *req.ApiKey
	}
	description := stringValue(req.Description)

	visibility := service.CredentialVisibility{}
	inactive := false

	var existingCredential *service.CredentialInfo
	if credentialID != "" {
		info, err := h.credentialService.GetByID(r.Context(), projectID, credentialID)
		if err != nil {
			if errors.Is(err, service.ErrCredentialNotFound) {
				h.Error(w, http.StatusNotFound, "Credential not found")
				return
			}
			h.Error(w, http.StatusInternalServerError, "Failed to load credential")
			return
		}
		existingCredential = info
		if provider == "custom" && !strings.HasPrefix(existingCredential.Provider, "custom:") {
			h.Error(w, http.StatusBadRequest, "Credential provider mismatch")
			return
		}
		if provider != "" && provider != "custom" && provider != existingCredential.Provider {
			h.Error(w, http.StatusBadRequest, "Credential provider mismatch")
			return
		}
		if req.AuthType != "" && req.AuthType != existingCredential.AuthType {
			h.Error(w, http.StatusBadRequest, "Credential auth type mismatch")
			return
		}
		if provider == "" {
			provider = existingCredential.Provider
		}
		if req.AuthType == "" {
			req.AuthType = existingCredential.AuthType
		}
		visibility = existingCredential.Visibility
		inactive = existingCredential.Inactive
	}
	if req.AgentVisible != nil {
		visibility.Tools = *req.AgentVisible
	}
	if req.Visibility != nil {
		visibility = credentialVisibilityFromAPI(*req.Visibility)
	}
	if req.Inactive != nil {
		inactive = *req.Inactive
	}

	var envVars []service.SecretEnvVar
	if req.EnvVars != nil {
		envVars = make([]service.SecretEnvVar, 0, len(*req.EnvVars))
		for _, envVar := range *req.EnvVars {
			envVars = append(envVars, service.SecretEnvVar{Key: envVar.Key, Value: envVar.Value, OriginalKey: stringValue(envVar.OriginalKey)})
		}
	}

	if len(envVars) > 0 || provider == "custom" || provider == "" {
		info, err := h.credentialService.SetCustomCredential(r.Context(), projectID, credentialID, req.Name, description, envVars, visibility, inactive)
		if err != nil {
			if errors.Is(err, service.ErrCredentialNotFound) {
				h.Error(w, http.StatusNotFound, "Credential not found")
				return
			}
			h.Error(w, http.StatusInternalServerError, "Failed to create credential")
			return
		}
		h.JSON(w, http.StatusOK, info)
		return
	}

	if provider == "" {
		h.Error(w, http.StatusBadRequest, "provider is required")
		return
	}

	if req.AuthType == "" || req.AuthType == service.AuthTypeAPIKey {
		if apiKey == "" {
			if credentialID != "" {
				info, err := h.credentialService.UpdateMetadata(r.Context(), projectID, credentialID, req.Name, description, visibility, inactive)
				if err != nil {
					if errors.Is(err, service.ErrCredentialNotFound) {
						h.Error(w, http.StatusNotFound, "Credential not found")
						return
					}
					h.Error(w, http.StatusInternalServerError, "Failed to update credential")
					return
				}
				h.JSON(w, http.StatusOK, info)
				return
			}
			h.Error(w, http.StatusBadRequest, "api_key is required for api_key auth type")
			return
		}

		info, err := h.credentialService.SetAPIKeyWithMetadata(r.Context(), projectID, provider, req.Name, description, apiKey, visibility, inactive)
		if err != nil {
			if errors.Is(err, keyvalidator.ErrValidationFailed) {
				h.Error(w, http.StatusBadRequest, err.Error())
				return
			}
			if errors.Is(err, service.ErrInvalidProvider) {
				h.Error(w, http.StatusBadRequest, "Invalid provider")
				return
			}
			h.Error(w, http.StatusInternalServerError, "Failed to create credential")
			return
		}

		h.JSON(w, http.StatusOK, info)
		return
	}

	if req.AuthType == service.AuthTypeID {
		if apiKey == "" {
			if credentialID != "" {
				info, err := h.credentialService.UpdateMetadata(r.Context(), projectID, credentialID, req.Name, description, visibility, inactive)
				if err != nil {
					if errors.Is(err, service.ErrCredentialNotFound) {
						h.Error(w, http.StatusNotFound, "Credential not found")
						return
					}
					h.Error(w, http.StatusInternalServerError, "Failed to update credential")
					return
				}
				h.JSON(w, http.StatusOK, info)
				return
			}
			h.Error(w, http.StatusBadRequest, "api_key is required for id auth type")
			return
		}

		info, err := h.credentialService.SetIDWithMetadata(r.Context(), projectID, provider, req.Name, description, apiKey, visibility, inactive)
		if err != nil {
			if errors.Is(err, service.ErrInvalidProvider) {
				h.Error(w, http.StatusBadRequest, "Invalid provider")
				return
			}
			h.Error(w, http.StatusInternalServerError, "Failed to create credential")
			return
		}

		h.JSON(w, http.StatusOK, info)
		return
	}

	if req.AuthType == service.AuthTypeOAuth && credentialID != "" {
		info, err := h.credentialService.UpdateMetadata(r.Context(), projectID, credentialID, req.Name, description, visibility, inactive)
		if err != nil {
			if errors.Is(err, service.ErrCredentialNotFound) {
				h.Error(w, http.StatusNotFound, "Credential not found")
				return
			}
			h.Error(w, http.StatusInternalServerError, "Failed to update credential")
			return
		}
		h.JSON(w, http.StatusOK, info)
		return
	}

	h.Error(w, http.StatusBadRequest, "OAuth credentials must be set via OAuth flow endpoints")
}

// GetCredential returns a single credential.
func (h *Handler) GetCredential(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	identifier := chi.URLParam(r, "provider")

	info, err := h.credentialService.Get(r.Context(), projectID, identifier)
	if err != nil {
		info, err = h.credentialService.GetByID(r.Context(), projectID, identifier)
	}
	if err != nil {
		if errors.Is(err, service.ErrCredentialNotFound) {
			h.Error(w, http.StatusNotFound, "Credential not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get credential")
		return
	}

	h.JSON(w, http.StatusOK, info)
}

// ValidateCredentials validates all credentials for a project.
func (h *Handler) ValidateCredentials(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	validations, err := h.credentialService.ValidateAll(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to validate credentials")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"validations": validations})
}

// ValidateCredential validates a single stored credential.
func (h *Handler) ValidateCredential(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	identifier := chi.URLParam(r, "provider")

	validation, err := h.credentialService.ValidateByProvider(r.Context(), projectID, identifier)
	if err != nil {
		validation, err = h.credentialService.ValidateByID(r.Context(), projectID, identifier)
	}
	if err != nil {
		if errors.Is(err, service.ErrCredentialNotFound) {
			h.Error(w, http.StatusNotFound, "Credential not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to validate credential")
		return
	}

	h.JSON(w, http.StatusOK, validation)
}

// DeleteCredential deletes a credential.
func (h *Handler) DeleteCredential(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	identifier := chi.URLParam(r, "provider")

	credential, err := h.credentialService.Get(r.Context(), projectID, identifier)
	if err != nil {
		credential, err = h.credentialService.GetByID(r.Context(), projectID, identifier)
	}
	if err != nil {
		if errors.Is(err, service.ErrCredentialNotFound) {
			h.Error(w, http.StatusNotFound, "Credential not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to delete credential")
		return
	}

	providerRefs, err := h.store.CountSandboxProviderInstancesReferencingCredential(r.Context(), projectID, credential.ID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to check credential usage")
		return
	}
	if providerRefs > 0 {
		h.Error(w, http.StatusConflict, "Credential is used by one or more sandbox providers")
		return
	}

	if err := h.credentialService.Delete(r.Context(), projectID, credential.ID); err != nil {
		if errors.Is(err, service.ErrCredentialNotFound) {
			h.Error(w, http.StatusNotFound, "Credential not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to delete credential")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type setSessionCredentialAssignmentRequest struct {
	CredentialID        string                         `json:"credentialId"`
	SessionCredentialID string                         `json:"sessionCredentialId,omitempty"`
	EnvVar              string                         `json:"envVar,omitempty"`
	SourceEnvVar        string                         `json:"sourceEnvVar,omitempty"`
	AgentVisible        bool                           `json:"agentVisible"`
	Visibility          *service.CredentialVisibility  `json:"visibility,omitempty"`
	Uses                []service.SessionCredentialUse `json:"uses,omitempty"`
}

// ListSessionCredentialAssignments returns credentials assigned to a session.
func (h *Handler) ListSessionCredentialAssignments(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	assignments, err := h.credentialService.ListSessionAssignments(r.Context(), projectID, sessionID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list session credentials")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"credentials": assignments})
}

// SetSessionCredentialAssignments replaces credentials assigned to a session.
func (h *Handler) SetSessionCredentialAssignments(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	sessionID := chi.URLParam(r, "sessionId")

	var req struct {
		Credentials []setSessionCredentialAssignmentRequest `json:"credentials"`
	}
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	assignments := make([]service.SessionCredentialAssignmentInfo, 0, len(req.Credentials))
	for _, credential := range req.Credentials {
		visibility := service.CredentialVisibility{Tools: credential.AgentVisible}
		if credential.Visibility != nil {
			visibility = *credential.Visibility
		}
		assignments = append(assignments, service.SessionCredentialAssignmentInfo{
			CredentialID:        credential.CredentialID,
			SessionCredentialID: credential.SessionCredentialID,
			EnvVar:              credential.EnvVar,
			SourceEnvVar:        credential.SourceEnvVar,
			AgentVisible:        visibility.Tools,
			Visibility:          visibility,
			Uses:                credential.Uses,
		})
	}

	updated, err := h.credentialService.SetSessionAssignments(r.Context(), projectID, sessionID, assignments)
	if err != nil {
		if errors.Is(err, service.ErrCredentialNotFound) {
			h.Error(w, http.StatusNotFound, "Credential not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to set session credentials")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"credentials": updated})
}

// RefreshCredential manually refreshes OAuth tokens for a credential
func (h *Handler) RefreshCredential(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	provider := chi.URLParam(r, "provider")

	tokens, err := h.credentialService.RefreshOAuthTokens(r.Context(), projectID, provider)
	if err != nil {
		if errors.Is(err, service.ErrCredentialNotFound) {
			h.Error(w, http.StatusNotFound, "Credential not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to refresh token: "+err.Error())
		return
	}

	// Return success response with new expiration time
	response := map[string]any{
		"success":   true,
		"expiresAt": tokens.ExpiresAt,
	}
	if !tokens.ExpiresAt.IsZero() {
		response["expiresIn"] = int(time.Until(tokens.ExpiresAt).Seconds())
	}

	h.JSON(w, http.StatusOK, response)
}

// AnthropicAuthorize generates PKCE and returns OAuth URL
func (h *Handler) AnthropicAuthorize(w http.ResponseWriter, _ *http.Request) {
	provider := oauth.NewAnthropicProvider(h.cfg.AnthropicClientID)
	authResp, err := provider.Authorize()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to generate authorization URL")
		return
	}

	h.JSON(w, http.StatusOK, authResp)
}

// AnthropicExchange exchanges code for tokens or accepts direct access tokens
func (h *Handler) AnthropicExchange(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req api.AnthropicExchangeRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Code == "" {
		h.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	var oauthCred *service.OAuthCredential
	var expiresAt time.Time

	// Check if this is a direct access token from 'claude setup-token'
	if strings.HasPrefix(req.Code, "sk-ant-oat0") {
		// Direct access token - store it with 1 year expiration
		expiresAt = time.Now().Add(365 * 24 * time.Hour)
		oauthCred = &service.OAuthCredential{
			AccessToken: req.Code,
			TokenType:   "Bearer",
			ExpiresAt:   expiresAt,
			// No refresh token for direct tokens
		}
	} else {
		// Regular OAuth flow - exchange authorization code for tokens
		if req.Verifier == "" {
			h.Error(w, http.StatusBadRequest, "verifier is required for authorization code")
			return
		}

		provider := oauth.NewAnthropicProvider(h.cfg.AnthropicClientID)
		tokenResp, err := provider.Exchange(r.Context(), req.Code, req.Verifier)
		if err != nil {
			// Return as JSON with success: false so frontend can display the error
			h.JSON(w, http.StatusOK, map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		expiresAt = tokenResp.ExpiresAt
		oauthCred = &service.OAuthCredential{
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			TokenType:    tokenResp.TokenType,
			ExpiresAt:    tokenResp.ExpiresAt,
			Scope:        tokenResp.Scope,
		}
	}

	// Store the credential (works for both OAuth and direct tokens)
	info, err := h.credentialService.SetOAuthTokens(r.Context(), projectID, service.ProviderAnthropic, "Anthropic OAuth", oauthCred)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to store credential")
		return
	}

	// Return success response with credential info
	response := map[string]any{
		"success":    true,
		"credential": info,
		"expiresAt":  expiresAt,
	}
	if !expiresAt.IsZero() {
		response["expiresIn"] = int(time.Until(expiresAt).Seconds())
	}

	h.JSON(w, http.StatusOK, response)
}

// GitHubCopilotDeviceCode initiates device flow
func (h *Handler) GitHubCopilotDeviceCode(w http.ResponseWriter, r *http.Request) {
	var req api.GitHubCopilotDeviceCodeRequest
	_ = h.DecodeJSON(r, &req)

	// Determine domain based on deployment type
	domain := oauth.DefaultGitHubDomain
	if stringValue(req.DeploymentType) == "enterprise" && stringValue(req.EnterpriseUrl) != "" {
		// Extract domain from enterprise URL
		domain = stringValue(req.EnterpriseUrl)
		// Strip protocol if present
		if idx := strings.Index(domain, "://"); idx != -1 {
			domain = domain[idx+3:]
		}
		// Strip trailing slash and path
		if idx := strings.Index(domain, "/"); idx != -1 {
			domain = domain[:idx]
		}
	}

	provider := oauth.NewGitHubCopilotProvider(h.cfg.GitHubCopilotClientID, domain)
	deviceResp, err := provider.RequestDeviceCode(r.Context())
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to request device code: "+err.Error())
		return
	}

	// Convert to camelCase for frontend
	h.JSON(w, http.StatusOK, api.GitHubCopilotDeviceCodeResponse{
		DeviceCode:      deviceResp.DeviceCode,
		UserCode:        deviceResp.UserCode,
		VerificationUri: deviceResp.VerificationURI,
		ExpiresIn:       deviceResp.ExpiresIn,
		Interval:        deviceResp.Interval,
		Domain:          domain,
	})
}

// GitHubCopilotPoll polls for device authorization
func (h *Handler) GitHubCopilotPoll(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req api.GitHubCopilotPollRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.DeviceCode == "" {
		h.Error(w, http.StatusBadRequest, "deviceCode is required")
		return
	}

	// Use domain from request, default to github.com
	domain := req.Domain
	if domain == "" {
		domain = oauth.DefaultGitHubDomain
	}

	provider := oauth.NewGitHubCopilotProvider(h.cfg.GitHubCopilotClientID, domain)
	pollResp, err := provider.PollForToken(r.Context(), req.DeviceCode)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Poll request failed: "+err.Error())
		return
	}

	// Check if authorization is still pending
	if pollResp.IsAuthorizationPending() {
		h.JSON(w, http.StatusAccepted, map[string]string{
			"status": "pending",
			"error":  "authorization_pending",
		})
		return
	}

	// Check for slow down request
	if pollResp.IsSlowDown() {
		h.JSON(w, http.StatusTooManyRequests, map[string]string{
			"status": "slow_down",
			"error":  "slow_down",
		})
		return
	}

	// Check for expired token
	if pollResp.IsExpired() {
		h.Error(w, http.StatusGone, "Device code expired")
		return
	}

	// Check for access denied
	if pollResp.IsAccessDenied() {
		h.Error(w, http.StatusForbidden, "Access denied by user")
		return
	}

	// Check for other errors
	if pollResp.Error != "" {
		h.Error(w, http.StatusBadRequest, pollResp.ErrorDesc)
		return
	}

	// We have a token! Store it
	if !pollResp.HasToken() {
		h.Error(w, http.StatusInternalServerError, "Unexpected response: no token received")
		return
	}

	oauthCred := &service.OAuthCredential{
		AccessToken:  pollResp.AccessToken,
		RefreshToken: pollResp.RefreshToken,
		TokenType:    pollResp.TokenType,
		Scope:        pollResp.Scope,
	}

	info, err := h.credentialService.SetOAuthTokens(r.Context(), projectID, service.ProviderGitHubCopilot, "GitHub Copilot", oauthCred)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to store credential")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"credential": info,
	})
}

func normalizeGitHubDomain(raw string) string {
	domain := strings.TrimSpace(raw)
	if domain == "" {
		return oauth.DefaultGitHubDomain
	}
	if idx := strings.Index(domain, "://"); idx != -1 {
		domain = domain[idx+3:]
	}
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	return domain
}

// GitHubDeviceCode initiates device flow for GitHub git operations (repo scope)
func (h *Handler) GitHubDeviceCode(w http.ResponseWriter, r *http.Request) {
	var req api.GitHubDeviceCodeRequest
	// Allow empty body, default to github.com
	_ = h.DecodeJSON(r, &req)

	domain := normalizeGitHubDomain(stringValue(req.EnterpriseUrl))

	if h.cfg.GitHubOAuthClientID == "" {
		h.Error(w, http.StatusServiceUnavailable, "GitHub OAuth not configured")
		return
	}

	provider := oauth.NewGitHubProvider(h.cfg.GitHubOAuthClientID, h.cfg.GitHubOAuthClientSecret, domain, stringSliceValue(req.Scopes))
	deviceResp, err := provider.RequestDeviceCode(r.Context())
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to request device code: "+err.Error())
		return
	}

	h.JSON(w, http.StatusOK, api.GitHubCopilotDeviceCodeResponse{
		DeviceCode:      deviceResp.DeviceCode,
		UserCode:        deviceResp.UserCode,
		VerificationUri: deviceResp.VerificationURI,
		ExpiresIn:       deviceResp.ExpiresIn,
		Interval:        deviceResp.Interval,
		Domain:          domain,
	})
}

// GitHubAuthorize starts the standard authorization-code flow for GitHub OAuth.
func (h *Handler) GitHubAuthorize(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if h.cfg.GitHubOAuthClientID == "" || h.cfg.GitHubOAuthClientSecret == "" {
		h.Error(w, http.StatusServiceUnavailable, "GitHub OAuth not configured")
		return
	}

	var req api.GitHubAuthorizeRequest
	_ = h.DecodeJSON(r, &req)

	domain := normalizeGitHubDomain(stringValue(req.EnterpriseUrl))
	redirectURI := strings.TrimSpace(stringValue(req.RedirectUri))
	if redirectURI == "" {
		redirectURI = "http://127.0.0.1:1455/auth/callback"
	}

	provider := oauth.NewGitHubProvider(h.cfg.GitHubOAuthClientID, h.cfg.GitHubOAuthClientSecret, domain, stringSliceValue(req.Scopes))
	authResp, err := provider.Authorize(redirectURI)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to generate authorization URL")
		return
	}

	visibility := service.CredentialVisibility{}
	if req.Visibility != nil {
		visibility = credentialVisibilityFromAPI(*req.Visibility)
	}
	inactive := false
	if req.Inactive != nil {
		inactive = *req.Inactive
	}

	callbackListening := false
	if h.oauthCallbackServer != nil {
		callbackListening = h.oauthCallbackServer.Start()
		if callbackListening {
			h.oauthCallbackServer.RegisterPendingGitHub(
				authResp.State,
				authResp.Verifier,
				projectID,
				redirectURI,
				domain,
				stringValue(req.CredentialId),
				stringValue(req.Name),
				stringValue(req.Description),
				visibility,
				inactive,
			)
		}
	}

	h.JSON(w, http.StatusOK, api.GitHubAuthorizeResponse{
		Url:               authResp.URL,
		Verifier:          authResp.Verifier,
		State:             authResp.State,
		RedirectUri:       redirectURI,
		CallbackListening: callbackListening,
	})
}

// GitHubPoll polls for GitHub device authorization
func (h *Handler) GitHubPoll(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req api.GitHubPollRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.DeviceCode == "" {
		h.Error(w, http.StatusBadRequest, "deviceCode is required")
		return
	}

	domain := normalizeGitHubDomain(req.Domain)

	provider := oauth.NewGitHubProvider(h.cfg.GitHubOAuthClientID, h.cfg.GitHubOAuthClientSecret, domain, nil)
	pollResp, err := provider.PollForToken(r.Context(), req.DeviceCode)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Poll request failed: "+err.Error())
		return
	}

	if pollResp.IsAuthorizationPending() {
		h.JSON(w, http.StatusAccepted, map[string]string{"status": "pending", "error": "authorization_pending"})
		return
	}

	if pollResp.IsSlowDown() {
		h.JSON(w, http.StatusTooManyRequests, map[string]string{"status": "slow_down", "error": "slow_down"})
		return
	}

	if pollResp.IsExpired() {
		h.Error(w, http.StatusGone, "Device code expired")
		return
	}

	if pollResp.IsAccessDenied() {
		h.Error(w, http.StatusForbidden, "Access denied by user")
		return
	}

	if pollResp.Error != "" {
		h.Error(w, http.StatusBadRequest, pollResp.ErrorDesc)
		return
	}

	if !pollResp.HasToken() {
		h.Error(w, http.StatusInternalServerError, "Unexpected response: no token received")
		return
	}

	oauthCred := &service.OAuthCredential{
		AccessToken: pollResp.AccessToken,
		TokenType:   pollResp.TokenType,
		Scope:       pollResp.Scope,
		// GitHub device flow does not issue refresh tokens
	}

	visibility := service.CredentialVisibility{}
	if req.Visibility != nil {
		visibility = credentialVisibilityFromAPI(*req.Visibility)
	}
	inactive := false
	if req.Inactive != nil {
		inactive = *req.Inactive
	}

	info, err := h.credentialService.SetOAuthTokensWithMetadata(
		r.Context(),
		projectID,
		stringValue(req.CredentialId),
		service.ProviderGitHub,
		stringValue(req.Name),
		stringValue(req.Description),
		visibility,
		inactive,
		oauthCred,
	)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to store credential")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{
		"status":     "success",
		"credential": info,
	})
}

// GitHubExchange exchanges a standard GitHub OAuth authorization code for tokens.
func (h *Handler) GitHubExchange(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if h.cfg.GitHubOAuthClientID == "" || h.cfg.GitHubOAuthClientSecret == "" {
		h.Error(w, http.StatusServiceUnavailable, "GitHub OAuth not configured")
		return
	}

	var req api.GitHubExchangeRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	redirectURI := strings.TrimSpace(stringValue(req.RedirectUri))
	req.Verifier = strings.TrimSpace(req.Verifier)
	if req.Code == "" {
		h.Error(w, http.StatusBadRequest, "code is required")
		return
	}
	if redirectURI == "" {
		redirectURI = "http://127.0.0.1:1455/auth/callback"
	}
	if req.Verifier == "" {
		h.Error(w, http.StatusBadRequest, "verifier is required")
		return
	}

	domain := normalizeGitHubDomain(stringValue(req.EnterpriseUrl))
	provider := oauth.NewGitHubProvider(h.cfg.GitHubOAuthClientID, h.cfg.GitHubOAuthClientSecret, domain, nil)
	tokenResp, err := provider.Exchange(r.Context(), req.Code, redirectURI, req.Verifier)
	if err != nil {
		h.Error(w, http.StatusBadRequest, "Token exchange failed: "+err.Error())
		return
	}

	visibility := service.CredentialVisibility{}
	if req.Visibility != nil {
		visibility = credentialVisibilityFromAPI(*req.Visibility)
	}
	inactive := false
	if req.Inactive != nil {
		inactive = *req.Inactive
	}

	info, err := h.credentialService.SetOAuthTokensWithMetadata(
		r.Context(),
		projectID,
		stringValue(req.CredentialId),
		service.ProviderGitHub,
		stringValue(req.Name),
		stringValue(req.Description),
		visibility,
		inactive,
		&service.OAuthCredential{
			AccessToken: tokenResp.AccessToken,
			TokenType:   tokenResp.TokenType,
			Scope:       tokenResp.Scope,
		},
	)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to store credential")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"credential": info,
	})
}

// GitHubCallbackStatus reports whether the localhost:1455 callback completed.
func (h *Handler) GitHubCallbackStatus(w http.ResponseWriter, r *http.Request) {
	var req api.GitHubCallbackStatusRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.State = strings.TrimSpace(req.State)
	if req.State == "" {
		h.Error(w, http.StatusBadRequest, "state is required")
		return
	}

	status := "pending"
	errMsg := ""
	if h.oauthCallbackServer != nil {
		status, errMsg = h.oauthCallbackServer.Status(req.State)
	}

	h.JSON(w, http.StatusOK, map[string]string{
		"status": status,
		"error":  errMsg,
	})
}

// PostMCPToken stores an MCP OAuth token posted by the agent after completing
// an OAuth exchange. The token is keyed by resource URL so it can be reused
// across sessions that share the same MCP server URL.
// Body: { "url": "https://api.example.com/mcp",
//
//	"accessToken": "...", "refreshToken": "...", "expiresAt": 1234567890 }
func (h *Handler) PostMCPToken(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var body service.MCPTokenData
	if err := h.DecodeJSON(r, &body); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.URL == "" {
		h.Error(w, http.StatusBadRequest, "url is required")
		return
	}
	if body.AccessToken == "" {
		h.Error(w, http.StatusBadRequest, "accessToken is required")
		return
	}

	if err := h.credentialService.StoreMCPToken(r.Context(), projectID, body); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to store MCP token")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CodexDeviceCode initiates the device-code flow for Codex/OpenAI OAuth.
func (h *Handler) CodexDeviceCode(w http.ResponseWriter, r *http.Request) {
	if h.cfg.CodexClientID == "" {
		h.Error(w, http.StatusServiceUnavailable, "Codex OAuth not configured")
		return
	}

	provider := oauth.NewCodexProvider(h.cfg.CodexClientID)
	deviceResp, err := provider.RequestDeviceCode(r.Context())
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to request device code: "+err.Error())
		return
	}

	interval, err := strconv.Atoi(deviceResp.Interval)
	if err != nil || interval < 1 {
		interval = 5
	}

	h.JSON(w, http.StatusOK, api.CodexDeviceCodeResponse{
		DeviceAuthId:    deviceResp.DeviceAuthID,
		UserCode:        deviceResp.UserCode,
		VerificationUri: oauth.CodexDevicePageURL,
		Interval:        interval,
	})
}

// CodexAuthorize starts the standard authorization-code flow for Codex/OpenAI OAuth.
func (h *Handler) CodexAuthorize(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if h.cfg.CodexClientID == "" {
		h.Error(w, http.StatusServiceUnavailable, "Codex OAuth not configured")
		return
	}

	var req api.CodexAuthorizeRequest
	_ = h.DecodeJSON(r, &req)

	redirectURI := strings.TrimSpace(req.RedirectUri)
	if redirectURI == "" {
		redirectURI = "http://localhost:1455/auth/callback"
	}

	provider := oauth.NewCodexProvider(h.cfg.CodexClientID)
	authResp, err := provider.Authorize(redirectURI)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to generate authorization URL")
		return
	}

	callbackListening := false
	if h.oauthCallbackServer != nil {
		callbackListening = h.oauthCallbackServer.Start()
		if callbackListening {
			h.oauthCallbackServer.RegisterPendingCodex(authResp.State, authResp.Verifier, projectID, redirectURI)
		}
	}

	h.JSON(w, http.StatusOK, api.CodexAuthorizeResponse{
		Url:               authResp.URL,
		Verifier:          authResp.Verifier,
		State:             authResp.State,
		RedirectUri:       redirectURI,
		CallbackListening: callbackListening,
	})
}

// CodexPoll polls the device-code flow for completion and stores the resulting credential.
func (h *Handler) CodexPoll(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req api.CodexPollRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.DeviceAuthId == "" {
		h.Error(w, http.StatusBadRequest, "deviceAuthId is required")
		return
	}
	if req.UserCode == "" {
		h.Error(w, http.StatusBadRequest, "userCode is required")
		return
	}

	provider := oauth.NewCodexProvider(h.cfg.CodexClientID)
	pollResp, statusCode, err := provider.PollDeviceCode(r.Context(), req.DeviceAuthId, req.UserCode)
	if err != nil {
		h.Error(w, http.StatusBadRequest, "Poll request failed: "+err.Error())
		return
	}
	if statusCode == http.StatusForbidden || statusCode == http.StatusNotFound {
		h.JSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
		return
	}
	if pollResp.AuthorizationCode == "" {
		h.Error(w, http.StatusInternalServerError, "Unexpected response: no authorization code received")
		return
	}
	if pollResp.CodeVerifier == "" {
		h.Error(w, http.StatusInternalServerError, "Unexpected response: no code verifier received")
		return
	}

	tokenResp, err := provider.Exchange(r.Context(), pollResp.AuthorizationCode, oauth.CodexDeviceCallbackURI, pollResp.CodeVerifier)
	if err != nil {
		h.Error(w, http.StatusBadRequest, "Token exchange failed: "+err.Error())
		return
	}

	// Store the tokens as a credential
	oauthCred := &service.OAuthCredential{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    tokenResp.ExpiresAt,
		Scope:        tokenResp.Scope,
	}

	info, err := h.credentialService.SetOAuthTokens(r.Context(), projectID, service.ProviderCodex, "OpenAI Codex", oauthCred)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to store credential")
		return
	}

	// Return credential info with token expiration
	response := map[string]any{
		"status":     "success",
		"credential": info,
		"expiresAt":  tokenResp.ExpiresAt,
	}
	if !tokenResp.ExpiresAt.IsZero() {
		response["expiresIn"] = int(time.Until(tokenResp.ExpiresAt).Seconds())
	}

	h.JSON(w, http.StatusOK, response)
}

// CodexExchange exchanges a standard OAuth authorization code for tokens.
func (h *Handler) CodexExchange(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if h.cfg.CodexClientID == "" {
		h.Error(w, http.StatusServiceUnavailable, "Codex OAuth not configured")
		return
	}

	var req api.CodexExchangeRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	redirectURI := strings.TrimSpace(req.RedirectUri)
	req.Verifier = strings.TrimSpace(req.Verifier)

	if req.Code == "" {
		h.Error(w, http.StatusBadRequest, "code is required")
		return
	}
	if redirectURI == "" {
		redirectURI = "http://localhost:1455/auth/callback"
	}
	if req.Verifier == "" {
		h.Error(w, http.StatusBadRequest, "verifier is required")
		return
	}

	provider := oauth.NewCodexProvider(h.cfg.CodexClientID)
	tokenResp, err := provider.Exchange(r.Context(), req.Code, redirectURI, req.Verifier)
	if err != nil {
		h.Error(w, http.StatusBadRequest, "Token exchange failed: "+err.Error())
		return
	}

	oauthCred := &service.OAuthCredential{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    tokenResp.ExpiresAt,
		Scope:        tokenResp.Scope,
	}

	info, err := h.credentialService.SetOAuthTokens(r.Context(), projectID, service.ProviderCodex, "OpenAI Codex", oauthCred)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to store credential")
		return
	}

	response := map[string]any{
		"success":    true,
		"credential": info,
		"expiresAt":  tokenResp.ExpiresAt,
	}
	if !tokenResp.ExpiresAt.IsZero() {
		response["expiresIn"] = int(time.Until(tokenResp.ExpiresAt).Seconds())
	}

	h.JSON(w, http.StatusOK, response)
}

// CodexCallbackStatus reports whether the localhost:1455 callback completed.
func (h *Handler) CodexCallbackStatus(w http.ResponseWriter, r *http.Request) {
	var req api.CodexCallbackStatusRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.State = strings.TrimSpace(req.State)
	if req.State == "" {
		h.Error(w, http.StatusBadRequest, "state is required")
		return
	}

	status := "pending"
	errMsg := ""
	if h.oauthCallbackServer != nil {
		status, errMsg = h.oauthCallbackServer.Status(req.State)
	}

	h.JSON(w, http.StatusOK, map[string]string{
		"status": status,
		"error":  errMsg,
	})
}
