package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var defaultGitHubScopes = []string{"repo", "read:user", "user:email"}

type GitHubProvider struct {
	*GitHubCopilotProvider
	ClientSecret string
}

type GitHubAuthorizeResponse struct {
	URL                 string `json:"url"`
	State               string `json:"state"`
	Verifier            string `json:"verifier"`
	CodeChallenge       string `json:"codeChallenge"`
	CodeChallengeMethod string `json:"codeChallengeMethod"`
}

type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`

	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

func normalizeGitHubScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return append([]string(nil), defaultGitHubScopes...)
	}
	result := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		normalized := strings.TrimSpace(scope)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return append([]string(nil), defaultGitHubScopes...)
	}
	return result
}

// NewGitHubProvider creates an OAuth provider for GitHub git operations.
// If domain is empty, defaults to github.com.
func NewGitHubProvider(clientID, clientSecret, domain string, scopes []string) *GitHubProvider {
	if domain == "" {
		domain = DefaultGitHubDomain
	}
	return &GitHubProvider{
		GitHubCopilotProvider: &GitHubCopilotProvider{
			ClientID: clientID,
			Domain:   domain,
			Scopes:   normalizeGitHubScopes(scopes),
		},
		ClientSecret: clientSecret,
	}
}

func (p *GitHubProvider) authorizeURL() string {
	if p.Domain == DefaultGitHubDomain {
		return "https://github.com/login/oauth/authorize"
	}
	return fmt.Sprintf("https://%s/login/oauth/authorize", p.Domain)
}

func (p *GitHubProvider) tokenURL() string {
	if p.Domain == DefaultGitHubDomain {
		return "https://github.com/login/oauth/access_token"
	}
	return fmt.Sprintf("https://%s/login/oauth/access_token", p.Domain)
}

func (p *GitHubProvider) Authorize(redirectURI string) (*GitHubAuthorizeResponse, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}
	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(p.Scopes, " "))
	params.Set("state", state)
	params.Set("code_challenge", pkce.CodeChallenge)
	params.Set("code_challenge_method", pkce.CodeChallengeMethod)

	return &GitHubAuthorizeResponse{
		URL:                 p.authorizeURL() + "?" + params.Encode(),
		State:               state,
		Verifier:            pkce.CodeVerifier,
		CodeChallenge:       pkce.CodeChallenge,
		CodeChallengeMethod: pkce.CodeChallengeMethod,
	}, nil
}

func (p *GitHubProvider) Exchange(ctx context.Context, code, redirectURI, verifier string) (*GitHubTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, "POST", p.tokenURL(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var tokenResp GitHubTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}
	if tokenResp.Error != "" {
		if tokenResp.ErrorDescription != "" {
			return nil, fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDescription)
		}
		return nil, fmt.Errorf("%s", tokenResp.Error)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github token exchange failed with status %d", resp.StatusCode)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("missing access token in response")
	}
	return &tokenResp, nil
}
