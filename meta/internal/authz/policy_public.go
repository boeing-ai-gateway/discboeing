package authz

import "context"

var publicActions = map[Action]struct{}{
	ActionOAuthAuthorize:    {},
	ActionOAuthMetadataRead: {},
	ActionOAuthToken:        {},
	ActionOIDCDiscoveryRead: {},
	ActionOIDCJWKSRead:      {},
	ActionUserWhoami:        {},
}

// PublicAuthorizer grants access to public unauthenticated actions.
type PublicAuthorizer struct{}

// Authorize grants requests whose action is listed in publicActions.
func (PublicAuthorizer) Authorize(_ context.Context, info RequestInfo) bool {
	_, ok := publicActions[info.Action]
	return ok
}
