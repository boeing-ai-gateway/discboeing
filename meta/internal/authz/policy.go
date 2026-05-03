package authz

import "github.com/obot-platform/discobot/meta/internal/store"

// NewMetaAuthorizer creates the default Meta router authorizer chain.
func NewMetaAuthorizer(_ *store.Store) Authorizers {
	return Authorizers{
		PublicAuthorizer{},
		BootstrapAuthorizer{},
	}
}
