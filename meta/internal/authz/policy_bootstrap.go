package authz

import "context"

var bootstrapActions = map[Action]struct{}{
	ActionOAuthApplicationCreate: {},
	ActionOAuthApplicationDelete: {},
	ActionOAuthApplicationList:   {},
	ActionOAuthApplicationRead:   {},
	ActionOAuthApplicationUpdate: {},
}

// BootstrapAuthorizer grants setup-mode bootstrap principals limited access to
// OAuth application management in the organization that issued the bootstrap
// token. Bootstrap tokens are created only while that organization is
// uninitialized and are revoked once an owner or admin exists.
type BootstrapAuthorizer struct{}

// Authorize grants authenticated bootstrap principals for OAuth application
// CRUD operations scoped to their own organization.
func (BootstrapAuthorizer) Authorize(_ context.Context, info RequestInfo) bool {
	if info.User == nil || !info.User.IsAuthenticated() || !hasExtra(info.User.Extra, "principal.type", "bootstrap") {
		return false
	}
	if _, ok := bootstrapActions[info.Action]; !ok {
		return false
	}
	organizationDomain := info.Params["organizationDomain"]
	return organizationDomain != "" && hasExtra(info.User.Extra, "organization.domain", organizationDomain)
}

func hasExtra(extra map[string][]string, key, value string) bool {
	for _, got := range extra[key] {
		if got == value {
			return true
		}
	}
	return false
}
