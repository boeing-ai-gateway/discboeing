package oauth

import "strings"

var defaultGitHubScopes = []string{"repo", "read:user", "user:email"}

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

// NewGitHubProvider creates a device flow provider for GitHub git operations.
// If domain is empty, defaults to github.com.
func NewGitHubProvider(clientID, domain string, scopes []string) *GitHubCopilotProvider {
	if domain == "" {
		domain = DefaultGitHubDomain
	}
	return &GitHubCopilotProvider{
		ClientID: clientID,
		Domain:   domain,
		Scopes:   normalizeGitHubScopes(scopes),
	}
}
