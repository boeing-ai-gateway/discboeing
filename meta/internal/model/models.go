// Package model contains the Meta service database model.
//
// These structs are the authoritative table-level description of Meta's
// persistent data model. The design documents in meta/docs/ describe the same
// model at a higher level, but field-level details should live here so schema,
// relationships, ownership rules, and grant semantics are maintained with the
// code.
package model

import (
	"time"

	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/id"
)

const (
	OrganizationStatusActive   = "active"
	OrganizationStatusDisabled = "disabled"
)

const (
	OrganizationRoleOwner  = "owner"
	OrganizationRoleAdmin  = "admin"
	OrganizationRoleMember = "member"
)

const (
	OrganizationBootstrapTokenStatusActive  = "active"
	OrganizationBootstrapTokenStatusRevoked = "revoked"
	OrganizationBootstrapTokenStatusExpired = "expired"
)

const (
	OAuthApplicationStatusActive   = "active"
	OAuthApplicationStatusDisabled = "disabled"
	OAuthApplicationStatusDeleted  = "deleted"
)

const (
	OAuthApplicationProviderGitHub = "github"
	OAuthApplicationProviderGoogle = "google"
)

const (
	PublicOrganizationDomain = "public"
)

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

const (
	UserSessionKindBrowser      = "browser"
	UserSessionKindDesktop      = "desktop"
	UserSessionKindAPIToken     = "api_token"
	UserSessionKindRefreshToken = "refresh_token"
)

const (
	UserSessionStatusActive  = "active"
	UserSessionStatusRevoked = "revoked"
	UserSessionStatusExpired = "expired"
)

const (
	UserDeviceStatusPending  = "pending"
	UserDeviceStatusActive   = "active"
	UserDeviceStatusDisabled = "disabled"
	UserDeviceStatusDeleted  = "deleted"
)

const (
	GroupStatusActive   = "active"
	GroupStatusDisabled = "disabled"
)

const (
	GroupMemberRoleAdmin  = "admin"
	GroupMemberRoleMember = "member"
)

const (
	ProjectStatusActive   = "active"
	ProjectStatusArchived = "archived"
	ProjectStatusDeleted  = "deleted"
)

const (
	ProjectMemberTypeUser  = "user"
	ProjectMemberTypeGroup = "group"
)

const (
	ProjectRoleOwner  = "owner"
	ProjectRoleAdmin  = "admin"
	ProjectRoleMember = "member"
	ProjectRoleViewer = "viewer"
)

const (
	AgentSessionRoleOwner  = "owner"
	AgentSessionRoleEditor = "editor"
	AgentSessionRoleViewer = "viewer"
)

const (
	PublicKeyStatusActive   = "active"
	PublicKeyStatusDisabled = "disabled"
	PublicKeyStatusDeleted  = "deleted"
)

const (
	SecretTypeAPIKey      = "api_key"
	SecretTypeOAuth       = "oauth"
	SecretTypeToken       = "token"
	SecretTypeEnvironment = "environment"
)

const (
	SecretOwnerTypeUser    = "user"
	SecretOwnerTypeGroup   = "group"
	SecretOwnerTypeProject = "project"
)

const (
	SecretVersionStateActive   = "active"
	SecretVersionStateDisabled = "disabled"
	SecretVersionStateDeleted  = "deleted"
)

const (
	SecretRecipientTypeUser         = "user"
	SecretRecipientTypeAgentSession = "agent_session"
)

const (
	SecretBindingTargetProject      = "project"
	SecretBindingTargetAgentSession = "agent_session"
)

const (
	SecretScopeLLM      = "llm"
	SecretScopeHooks    = "hooks"
	SecretScopeServices = "services"
	SecretScopeTools    = "tools"
)

const (
	JWTSigningKeyBackendDBLocal       = "db-local"
	JWTSigningKeyBackendAWSKMS        = "aws-kms"
	JWTSigningKeyBackendGCPKMS        = "gcp-kms"
	JWTSigningKeyBackendAzureKeyVault = "azure-key-vault"
)

const (
	JWTSigningKeyStatusNext     = "next"
	JWTSigningKeyStatusActive   = "active"
	JWTSigningKeyStatusRetired  = "retired"
	JWTSigningKeyStatusDisabled = "disabled"
)

const (
	AuditResultSuccess = "success"
	AuditResultDenied  = "denied"
	AuditResultError   = "error"
)

// Organization is the top-level Meta hierarchy boundary.
//
// Every project, group, and secret belongs to an organization. Users join
// organizations through OrganizationMember rows. Most API routes are scoped
// under an organization domain, with the special public organization available
// through shortcut routes that omit the organization path segment.
//
// Domain is the route-facing organization identifier, for example `example.com`.
// The database still uses ID as the stable primary key. The well-known public
// organization uses PublicOrganizationDomain.
type Organization struct {
	ID          string         `gorm:"primaryKey;type:text" json:"id"`
	Name        string         `gorm:"not null;type:text" json:"name"`
	Domain      string         `gorm:"not null;type:text;uniqueIndex" json:"domain"`
	Description *string        `gorm:"type:text" json:"description,omitempty"`
	Status      string         `gorm:"not null;type:text;default:active;index" json:"status"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Members           []OrganizationMember         `gorm:"foreignKey:OrganizationID" json:"-"`
	Groups            []Group                      `gorm:"foreignKey:OrganizationID" json:"-"`
	Projects          []Project                    `gorm:"foreignKey:OrganizationID" json:"-"`
	Secrets           []Secret                     `gorm:"foreignKey:OrganizationID" json:"-"`
	OAuthApplications []OAuthApplication           `gorm:"foreignKey:OrganizationID" json:"-"`
	BootstrapTokens   []OrganizationBootstrapToken `gorm:"foreignKey:OrganizationID" json:"-"`
}

func (Organization) TableName() string { return "organizations" }

func (o *Organization) BeforeCreate(_ *gorm.DB) error {
	if o.ID == "" {
		o.ID = id.MustNew(id.TypeOrganization)
	}
	if o.Status == "" {
		o.Status = OrganizationStatusActive
	}
	return nil
}

// OrganizationMember grants a user membership in an organization.
//
// Organization members can see and use organization-level resources according to
// their role and concrete project/group/session/secret rules. Organization roles
// are intentionally simple: owners have full control, admins manage
// organization-level resources, and members can participate where project/group
// membership permits.
type OrganizationMember struct {
	ID             string         `gorm:"primaryKey;type:text" json:"id"`
	OrganizationID string         `gorm:"column:organization_id;not null;type:text;uniqueIndex:idx_organization_user;index" json:"organizationId"`
	UserID         string         `gorm:"column:user_id;not null;type:text;uniqueIndex:idx_organization_user;index" json:"userId"`
	Role           string         `gorm:"not null;type:text;default:member" json:"role"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
	User         *User         `gorm:"foreignKey:UserID" json:"-"`
}

func (OrganizationMember) TableName() string { return "organization_members" }

func (m *OrganizationMember) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = id.MustNew(id.TypeOrganizationMember)
	}
	if m.Role == "" {
		m.Role = OrganizationRoleMember
	}
	return nil
}

// OrganizationBootstrapToken stores a high-entropy bootstrap token for one
// organization.
//
// Bootstrap tokens are setup principals, not human users. They authenticate only
// the narrow bootstrap role for an organization, initially so an administrator
// can configure the first OAuth provider before any real organization admin has
// logged in. Store only TokenHash; the raw token is printed once on creation.
// TokenHash is a SHA-256 lookup hash of a 256-bit random token, not a recoverable
// encrypted field, because Meta never needs to decrypt or display the token again.
type OrganizationBootstrapToken struct {
	ID             string         `gorm:"primaryKey;type:text" json:"id"`
	OrganizationID string         `gorm:"column:organization_id;not null;type:text;index" json:"organizationId"`
	TokenHash      string         `gorm:"column:token_hash;not null;type:text;uniqueIndex" json:"-"`
	Status         string         `gorm:"not null;type:text;default:active;index" json:"status"`
	Description    *string        `gorm:"type:text" json:"description,omitempty"`
	ExpiresAt      *time.Time     `gorm:"column:expires_at;index" json:"expiresAt,omitempty"`
	LastUsedAt     *time.Time     `gorm:"column:last_used_at" json:"lastUsedAt,omitempty"`
	RevokedAt      *time.Time     `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
}

func (OrganizationBootstrapToken) TableName() string { return "organization_bootstrap_tokens" }

func (t *OrganizationBootstrapToken) BeforeCreate(_ *gorm.DB) error {
	if t.ID == "" {
		t.ID = id.MustNew(id.TypeOrganizationBootstrapToken)
	}
	if t.Status == "" {
		t.Status = OrganizationBootstrapTokenStatusActive
	}
	return nil
}

// OAuthApplication is an organization-scoped OAuth/OIDC client application.
//
// OAuth applications belong to exactly one organization. They represent login
// clients and other OAuth clients that can request tokens from Meta. Keeping
// them organization-scoped lets each organization configure its own allowed
// redirect URIs, grant types, response types, and scopes.
//
// The public organization's owners/admins effectively administer the default
// public login surface for the Meta service. Public users may join the public
// organization as members, but they do not manage OAuth applications unless they
// are promoted to an organization admin/owner.
type OAuthApplication struct {
	ID                      string         `gorm:"primaryKey;type:text" json:"id"`
	OrganizationID          string         `gorm:"column:organization_id;not null;type:text;index" json:"organizationId"`
	Provider                string         `gorm:"not null;type:text;index" json:"provider"`
	ClientID                string         `gorm:"column:client_id;not null;type:text;uniqueIndex" json:"clientId"`
	ClientSecretEncrypted   []byte         `gorm:"column:client_secret_encrypted" json:"-"`
	Name                    string         `gorm:"not null;type:text" json:"name"`
	RedirectURIsJSON        []byte         `gorm:"column:redirect_uris_json;not null" json:"-"`
	GrantTypesJSON          []byte         `gorm:"column:grant_types_json;not null" json:"-"`
	ResponseTypesJSON       []byte         `gorm:"column:response_types_json;not null" json:"-"`
	Scopes                  string         `gorm:"not null;type:text" json:"scopes"`
	ProviderConfigJSON      []byte         `gorm:"column:provider_config_json" json:"-"`
	TokenEndpointAuthMethod string         `gorm:"column:token_endpoint_auth_method;not null;type:text" json:"tokenEndpointAuthMethod"`
	Status                  string         `gorm:"not null;type:text;default:active;index" json:"status"`
	CreatedByPrincipal      string         `gorm:"column:created_by_principal;not null;type:text;index" json:"createdByPrincipal"`
	CreatedAt               time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt               time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt               gorm.DeletedAt `gorm:"index" json:"-"`

	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
}

func (OAuthApplication) TableName() string { return "oauth_applications" }

func (a *OAuthApplication) BeforeCreate(_ *gorm.DB) error {
	if a.ID == "" {
		a.ID = id.MustNew(id.TypeOAuthApplication)
	}
	if a.Status == "" {
		a.Status = OAuthApplicationStatusActive
	}
	return nil
}

// User represents a human Discobot account.
//
// Users are the root identity for people using Discobot. A user can authenticate
// through one or more external identities, establish login sessions, approve
// client devices, belong to groups, join projects, join agent sessions directly,
// and own personal secrets.
//
// The user's devices, not the Meta service, hold private keys used to unwrap
// secret content-encryption keys. Meta stores only user/device metadata and the
// public keys needed for recipient wrapping.
type User struct {
	ID            string         `gorm:"primaryKey;type:text" json:"id"`
	PrimaryEmail  string         `gorm:"column:primary_email;not null;type:text;uniqueIndex" json:"primaryEmail"`
	EmailVerified bool           `gorm:"column:email_verified;not null;default:false" json:"emailVerified"`
	DisplayName   *string        `gorm:"column:display_name;type:text" json:"displayName,omitempty"`
	AvatarURL     *string        `gorm:"column:avatar_url;type:text" json:"avatarUrl,omitempty"`
	Status        string         `gorm:"not null;type:text;default:active;index" json:"status"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	Identities          []UserIdentity       `gorm:"foreignKey:UserID" json:"-"`
	Sessions            []UserSession        `gorm:"foreignKey:UserID" json:"-"`
	Devices             []UserDevice         `gorm:"foreignKey:UserID" json:"-"`
	GroupMemberships    []GroupMember        `gorm:"foreignKey:UserID" json:"-"`
	AgentMemberships    []AgentSessionMember `gorm:"foreignKey:UserID" json:"-"`
	CreatedAgentSession []AgentSession       `gorm:"foreignKey:CreatedByUserID" json:"-"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = id.MustNew(id.TypeUser)
	}
	if u.Status == "" {
		u.Status = UserStatusActive
	}
	return nil
}

// UserIdentity links a user to an external login identity.
//
// The Provider and Subject pair is the stable upstream identity key. Examples
// include Google/GitHub/OIDC subject identifiers. UserIdentity rows let Meta act
// as Discobot's identity authority while still federating login through external
// providers.
//
// ClaimsJSON stores a non-authoritative provider claim snapshot for debugging or
// display. Authorization should use canonical Meta users, groups, memberships,
// and sessions rather than trusting mutable provider claims directly.
type UserIdentity struct {
	ID            string         `gorm:"primaryKey;type:text" json:"id"`
	UserID        string         `gorm:"column:user_id;not null;type:text;index" json:"userId"`
	Provider      string         `gorm:"not null;type:text;uniqueIndex:idx_user_identity_provider_subject" json:"provider"`
	Subject       string         `gorm:"not null;type:text;uniqueIndex:idx_user_identity_provider_subject" json:"subject"`
	Email         string         `gorm:"not null;type:text" json:"email"`
	EmailVerified bool           `gorm:"column:email_verified;not null;default:false" json:"emailVerified"`
	ClaimsJSON    []byte         `gorm:"column:claims_json" json:"-"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (UserIdentity) TableName() string { return "user_identities" }

func (i *UserIdentity) BeforeCreate(_ *gorm.DB) error {
	if i.ID == "" {
		i.ID = id.MustNew(id.TypeUserIdentity)
	}
	return nil
}

// UserSession represents authenticated login state for a user.
//
// A user session is a browser, desktop, API-token, or refresh-token session for
// a human user. It is distinct from AgentSession, which represents a Discobot
// coding-agent runtime.
//
// UserSession supports account/session-management UX: users can list active
// sessions, see their last use, and revoke sessions. TokenHash stores only a hash
// of the opaque session/token secret. Raw user-session tokens must not be stored
// in Meta.
//
// DeviceID optionally links a login session to a UserDevice. A session can be
// valid for API authentication even when the associated device is pending, but a
// pending device cannot unwrap existing secrets until an active device approves
// it and creates the necessary recipient wraps.
type UserSession struct {
	ID            string         `gorm:"primaryKey;type:text" json:"id"`
	UserID        string         `gorm:"column:user_id;not null;type:text;index" json:"userId"`
	DeviceID      *string        `gorm:"column:device_id;type:text;index" json:"deviceId,omitempty"`
	Kind          string         `gorm:"not null;type:text;index" json:"kind"`
	TokenHash     string         `gorm:"column:token_hash;not null;type:text;uniqueIndex" json:"-"`
	DisplayName   *string        `gorm:"column:display_name;type:text" json:"displayName,omitempty"`
	UserAgent     *string        `gorm:"column:user_agent;type:text" json:"userAgent,omitempty"`
	IPAddress     *string        `gorm:"column:ip_address;type:text" json:"ipAddress,omitempty"`
	Status        string         `gorm:"not null;type:text;default:active;index" json:"status"`
	ExpiresAt     time.Time      `gorm:"column:expires_at;not null;index" json:"expiresAt"`
	LastSeenAt    *time.Time     `gorm:"column:last_seen_at" json:"lastSeenAt,omitempty"`
	RevokedAt     *time.Time     `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	RevokedReason *string        `gorm:"column:revoked_reason;type:text" json:"revokedReason,omitempty"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	User   *User       `gorm:"foreignKey:UserID" json:"-"`
	Device *UserDevice `gorm:"foreignKey:DeviceID" json:"-"`
}

func (UserSession) TableName() string { return "user_sessions" }

func (s *UserSession) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = id.MustNew(id.TypeUserSession)
	}
	if s.Status == "" {
		s.Status = UserSessionStatusActive
	}
	return nil
}

// UserDevice represents a user client device with a public encryption key.
//
// Each device owns a private key stored outside Meta, usually in the operating
// system key store. Meta stores the corresponding public key and approval state
// so clients can wrap secret content-encryption keys to approved devices.
//
// Device lifecycle:
//   - the first device for a user can become active after login because no
//     existing device can approve it;
//   - additional devices start pending;
//   - an active device for the same user approves a pending device and uploads
//     recipient wraps for secrets the new device should be able to decrypt;
//   - disabled/deleted devices should no longer receive new wraps.
//
// If a user loses all active devices, Meta cannot recover user-owned secret
// values. Those secrets must be re-entered or rotated.
type UserDevice struct {
	ID                 string         `gorm:"primaryKey;type:text" json:"id"`
	UserID             string         `gorm:"column:user_id;not null;type:text;index" json:"userId"`
	Name               string         `gorm:"not null;type:text" json:"name"`
	PublicKeyAlgorithm string         `gorm:"column:public_key_algorithm;not null;type:text" json:"publicKeyAlgorithm"`
	PublicKey          []byte         `gorm:"column:public_key;not null" json:"publicKey"`
	Status             string         `gorm:"not null;type:text;default:pending;index" json:"status"`
	ApprovedByDeviceID *string        `gorm:"column:approved_by_device_id;type:text;index" json:"approvedByDeviceId,omitempty"`
	ApprovedAt         *time.Time     `gorm:"column:approved_at" json:"approvedAt,omitempty"`
	LastSeenAt         *time.Time     `gorm:"column:last_seen_at" json:"lastSeenAt,omitempty"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt          time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`

	User             *User       `gorm:"foreignKey:UserID" json:"-"`
	ApprovedByDevice *UserDevice `gorm:"foreignKey:ApprovedByDeviceID" json:"-"`
}

func (UserDevice) TableName() string { return "user_devices" }

func (d *UserDevice) BeforeCreate(_ *gorm.DB) error {
	if d.ID == "" {
		d.ID = id.MustNew(id.TypeUserDevice)
	}
	if d.Status == "" {
		d.Status = UserDeviceStatusPending
	}
	return nil
}

// Group represents a named collection of users.
//
// Groups are first-class authorization subjects for project membership and
// secret ownership inside one organization. They model team access that should
// not depend on one individual user account.
//
// Groups can be managed manually or linked to external group identities via
// GroupIdentity. Group-owned secrets are managed by group admins and can be bound
// to projects or agent sessions just like user- or project-owned secrets.
type Group struct {
	ID             string         `gorm:"primaryKey;type:text" json:"id"`
	OrganizationID string         `gorm:"column:organization_id;not null;type:text;uniqueIndex:idx_organization_group_name;index" json:"organizationId"`
	Name           string         `gorm:"not null;type:text;uniqueIndex:idx_organization_group_name" json:"name"`
	Description    *string        `gorm:"type:text" json:"description,omitempty"`
	Status         string         `gorm:"not null;type:text;default:active;index" json:"status"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	Organization *Organization   `gorm:"foreignKey:OrganizationID" json:"-"`
	Members      []GroupMember   `gorm:"foreignKey:GroupID" json:"-"`
	Identities   []GroupIdentity `gorm:"foreignKey:GroupID" json:"-"`
}

func (Group) TableName() string { return "groups" }

func (g *Group) BeforeCreate(_ *gorm.DB) error {
	if g.ID == "" {
		g.ID = id.MustNew(id.TypeGroup)
	}
	if g.Status == "" {
		g.Status = GroupStatusActive
	}
	return nil
}

// GroupMember represents user membership in a group.
//
// Group members inherit project access when the group is a ProjectMember.
// Role is local to the group: admins can manage membership and group-owned
// secrets, while members inherit group-based access.
//
// Source records whether membership is manually managed in Meta or synchronized
// from an external identity provider. Externally-sourced memberships may be
// read-only to Meta service APIs depending on the provider integration.
type GroupMember struct {
	ID        string         `gorm:"primaryKey;type:text" json:"id"`
	GroupID   string         `gorm:"column:group_id;not null;type:text;uniqueIndex:idx_group_user;index" json:"groupId"`
	UserID    string         `gorm:"column:user_id;not null;type:text;uniqueIndex:idx_group_user;index" json:"userId"`
	Role      string         `gorm:"not null;type:text;default:member" json:"role"`
	Source    string         `gorm:"not null;type:text;default:manual" json:"source"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Group *Group `gorm:"foreignKey:GroupID" json:"-"`
	User  *User  `gorm:"foreignKey:UserID" json:"-"`
}

func (GroupMember) TableName() string { return "group_members" }

func (m *GroupMember) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = id.MustNew(id.TypeGroupMember)
	}
	if m.Role == "" {
		m.Role = GroupMemberRoleMember
	}
	if m.Source == "" {
		m.Source = "manual"
	}
	return nil
}

// GroupIdentity links a group to an external group identity.
//
// Examples include Active Directory groups, LDAP groups, OIDC/SAML group claim
// values, or GitHub teams. ExternalID should be used when the upstream provider
// exposes a stable identifier; Name can hold the provider's group name or claim
// value for display and providers that lack stable IDs.
type GroupIdentity struct {
	ID         string         `gorm:"primaryKey;type:text" json:"id"`
	GroupID    string         `gorm:"column:group_id;not null;type:text;index" json:"groupId"`
	Provider   string         `gorm:"not null;type:text;uniqueIndex:idx_group_identity_external" json:"provider"`
	ExternalID *string        `gorm:"column:external_id;type:text;uniqueIndex:idx_group_identity_external" json:"externalId,omitempty"`
	Name       string         `gorm:"not null;type:text;index" json:"name"`
	ClaimsJSON []byte         `gorm:"column:claims_json" json:"-"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Group *Group `gorm:"foreignKey:GroupID" json:"-"`
}

func (GroupIdentity) TableName() string { return "group_identities" }

func (i *GroupIdentity) BeforeCreate(_ *gorm.DB) error {
	if i.ID == "" {
		i.ID = id.MustNew(id.TypeGroupIdentity)
	}
	return nil
}

// Project represents a Discobot collaboration boundary.
//
// Projects are the main collaboration and authorization boundary inside an
// organization. Users and groups become project members through ProjectMember
// rows. Projects own agent sessions and can own secrets. A project-level
// SecretBinding makes a secret eligible for all current and future agent
// sessions in that project, subject to recipient-wrap availability.
type Project struct {
	ID              string         `gorm:"primaryKey;type:text" json:"id"`
	OrganizationID  string         `gorm:"column:organization_id;not null;type:text;uniqueIndex:idx_organization_project_slug;index" json:"organizationId"`
	Name            string         `gorm:"not null;type:text" json:"name"`
	Slug            string         `gorm:"not null;type:text;uniqueIndex:idx_organization_project_slug" json:"slug"`
	Description     *string        `gorm:"type:text" json:"description,omitempty"`
	Status          string         `gorm:"not null;type:text;default:active;index" json:"status"`
	CreatedByUserID string         `gorm:"column:created_by_user_id;not null;type:text;index" json:"createdByUserId"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	Organization  *Organization   `gorm:"foreignKey:OrganizationID" json:"-"`
	CreatedByUser *User           `gorm:"foreignKey:CreatedByUserID" json:"-"`
	Members       []ProjectMember `gorm:"foreignKey:ProjectID" json:"-"`
	AgentSessions []AgentSession  `gorm:"foreignKey:ProjectID" json:"-"`
}

func (Project) TableName() string { return "projects" }

func (p *Project) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = id.MustNew(id.TypeProject)
	}
	if p.Status == "" {
		p.Status = ProjectStatusActive
	}
	return nil
}

// ProjectMember represents direct project membership for a user or group.
//
// MemberType determines whether MemberID points to a User or Group. User access
// to a project is the union of direct user memberships and memberships inherited
// through groups.
//
// Project roles follow owner > admin > member > viewer. Owners have full
// control; admins can manage settings, members, agent sessions, and project
// secrets; members can use the project and participate in agent sessions; viewers
// have read-only access.
type ProjectMember struct {
	ID         string         `gorm:"primaryKey;type:text" json:"id"`
	ProjectID  string         `gorm:"column:project_id;not null;type:text;uniqueIndex:idx_project_member;index" json:"projectId"`
	MemberType string         `gorm:"column:member_type;not null;type:text;uniqueIndex:idx_project_member" json:"memberType"`
	MemberID   string         `gorm:"column:member_id;not null;type:text;uniqueIndex:idx_project_member;index" json:"memberId"`
	Role       string         `gorm:"not null;type:text;default:member" json:"role"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (ProjectMember) TableName() string { return "project_members" }

func (m *ProjectMember) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = id.MustNew(id.TypeProjectMember)
	}
	if m.Role == "" {
		m.Role = ProjectRoleMember
	}
	return nil
}

// AgentSession represents a Discobot coding-agent session.
//
// Agent sessions are not authenticated user sessions. They represent the
// coding-agent runtime/work session that users talk to. Every agent session
// belongs to one project.
//
// Users can receive explicit session membership through AgentSessionMember, and
// project membership can also authorize access. Agent sessions have public keys
// so secret content-encryption keys can be wrapped directly to the runtime. The
// matching private key lives in the agent runtime, not in Meta.
type AgentSession struct {
	ID              string         `gorm:"primaryKey;type:text" json:"id"`
	ProjectID       string         `gorm:"column:project_id;not null;type:text;index" json:"projectId"`
	Name            string         `gorm:"not null;type:text" json:"name"`
	Description     *string        `gorm:"type:text" json:"description,omitempty"`
	Status          string         `gorm:"not null;type:text;index" json:"status"`
	CreatedByUserID string         `gorm:"column:created_by_user_id;not null;type:text;index" json:"createdByUserId"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	Project       *Project                `gorm:"foreignKey:ProjectID" json:"-"`
	CreatedByUser *User                   `gorm:"foreignKey:CreatedByUserID" json:"-"`
	Members       []AgentSessionMember    `gorm:"foreignKey:AgentSessionID" json:"-"`
	PublicKeys    []AgentSessionPublicKey `gorm:"foreignKey:AgentSessionID" json:"-"`
}

func (AgentSession) TableName() string { return "agent_sessions" }

func (s *AgentSession) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = id.MustNew(id.TypeAgentSession)
	}
	return nil
}

// AgentSessionPublicKey stores a public encryption key for an agent session runtime.
//
// The agent runtime owns the matching private key. Meta stores only the public
// key and status. A SecretBinding authorizes an agent session to receive a
// secret, but the agent session cannot decrypt that secret until there is a
// SecretRecipientWrap for an active AgentSessionPublicKey.
type AgentSessionPublicKey struct {
	ID             string         `gorm:"primaryKey;type:text" json:"id"`
	AgentSessionID string         `gorm:"column:agent_session_id;not null;type:text;index" json:"agentSessionId"`
	Algorithm      string         `gorm:"not null;type:text" json:"algorithm"`
	PublicKey      []byte         `gorm:"column:public_key;not null" json:"publicKey"`
	Status         string         `gorm:"not null;type:text;default:active;index" json:"status"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	AgentSession *AgentSession `gorm:"foreignKey:AgentSessionID" json:"-"`
}

func (AgentSessionPublicKey) TableName() string { return "agent_session_public_keys" }

func (k *AgentSessionPublicKey) BeforeCreate(_ *gorm.DB) error {
	if k.ID == "" {
		k.ID = id.MustNew(id.TypeAgentSessionPublicKey)
	}
	if k.Status == "" {
		k.Status = PublicKeyStatusActive
	}
	return nil
}

// AgentSessionMember represents explicit user membership in an agent session.
//
// This grants session-specific access to a user independent of project-wide
// membership. Roles follow owner > editor > viewer. Owners can manage the agent
// session; editors can interact with the agent and modify session state; viewers
// can read session state but not send chat/control actions.
//
// Groups are intentionally not direct agent-session members in the current
// model. Group access flows through project membership unless we later add
// session-level group membership.
type AgentSessionMember struct {
	ID             string         `gorm:"primaryKey;type:text" json:"id"`
	AgentSessionID string         `gorm:"column:agent_session_id;not null;type:text;uniqueIndex:idx_agent_session_user;index" json:"agentSessionId"`
	UserID         string         `gorm:"column:user_id;not null;type:text;uniqueIndex:idx_agent_session_user;index" json:"userId"`
	Role           string         `gorm:"not null;type:text;default:viewer" json:"role"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	AgentSession *AgentSession `gorm:"foreignKey:AgentSessionID" json:"-"`
	User         *User         `gorm:"foreignKey:UserID" json:"-"`
}

func (m AgentSessionMember) TableName() string { return "agent_session_members" }

func (m *AgentSessionMember) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = id.MustNew(id.TypeAgentSessionMember)
	}
	if m.Role == "" {
		m.Role = AgentSessionRoleViewer
	}
	return nil
}

// Secret stores non-sensitive secret metadata.
//
// Secret values never live on this table. Values are encrypted client-side and
// stored as ciphertext in SecretVersion.EncryptedData. For environment secrets,
// variable names are non-sensitive and may be stored in MetadataJSON for display,
// filtering, and conflict detection, while variable values must remain inside the
// encrypted version payload.
//
// Secrets belong to one organization and are owned by users, groups, or projects
// through SecretOwner rows. Ownership controls who can manage metadata and
// rotation. SecretBinding rows separately control where a secret is available for
// use.
type Secret struct {
	ID               string         `gorm:"primaryKey;type:text" json:"id"`
	OrganizationID   string         `gorm:"column:organization_id;not null;type:text;index" json:"organizationId"`
	Name             string         `gorm:"not null;type:text;index" json:"name"`
	Type             string         `gorm:"not null;type:text;index" json:"type"`
	Description      *string        `gorm:"type:text" json:"description,omitempty"`
	MetadataJSON     []byte         `gorm:"column:metadata_json" json:"-"`
	CurrentVersionID *string        `gorm:"column:current_version_id;type:text;index" json:"currentVersionId,omitempty"`
	CreatedByUserID  string         `gorm:"column:created_by_user_id;not null;type:text;index" json:"createdByUserId"`
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	Organization  *Organization   `gorm:"foreignKey:OrganizationID" json:"-"`
	CreatedByUser *User           `gorm:"foreignKey:CreatedByUserID" json:"-"`
	Owners        []SecretOwner   `gorm:"foreignKey:SecretID" json:"-"`
	Versions      []SecretVersion `gorm:"foreignKey:SecretID" json:"-"`
	Bindings      []SecretBinding `gorm:"foreignKey:SecretID" json:"-"`
}

func (Secret) TableName() string { return "secrets" }

func (s *Secret) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = id.MustNew(id.TypeSecret)
	}
	return nil
}

// SecretOwner declares ownership of a secret by a user, group, or project.
//
// OwnerType determines whether OwnerID points to a User, Group, or Project.
// Ownership answers "who controls this secret?" and is distinct from bindings,
// which answer "where can this secret be used?".
//
// A secret usually has one owner, but this join table can support shared
// ownership if that becomes useful.
type SecretOwner struct {
	ID        string         `gorm:"primaryKey;type:text" json:"id"`
	SecretID  string         `gorm:"column:secret_id;not null;type:text;uniqueIndex:idx_secret_owner;index" json:"secretId"`
	OwnerType string         `gorm:"column:owner_type;not null;type:text;uniqueIndex:idx_secret_owner" json:"ownerType"`
	OwnerID   string         `gorm:"column:owner_id;not null;type:text;uniqueIndex:idx_secret_owner;index" json:"ownerId"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Secret *Secret `gorm:"foreignKey:SecretID" json:"-"`
}

func (SecretOwner) TableName() string { return "secret_owners" }

func (o *SecretOwner) BeforeCreate(_ *gorm.DB) error {
	if o.ID == "" {
		o.ID = id.MustNew(id.TypeSecretOwner)
	}
	return nil
}

// SecretVersion stores encrypted payload ciphertext for one secret version.
//
// Meta must never see plaintext secret values or plaintext content-encryption
// keys. EncryptedData is an opaque ciphertext payload produced outside Meta by a
// client, agent runtime, or external KMS/HSM flow. EncryptionAlgorithm and
// EncryptionNonce record the payload-encryption envelope metadata needed by the
// decrypting recipient.
//
// Secret versions are monotonically numbered within a secret. Bindings point to
// the stable Secret, and normal resolution returns the current active version.
// Older versions can be disabled or deleted while preserving audit history.
type SecretVersion struct {
	ID                  string         `gorm:"primaryKey;type:text" json:"id"`
	SecretID            string         `gorm:"column:secret_id;not null;type:text;uniqueIndex:idx_secret_version;index" json:"secretId"`
	Version             int            `gorm:"not null;uniqueIndex:idx_secret_version" json:"version"`
	EncryptedData       []byte         `gorm:"column:encrypted_data;not null" json:"-"`
	EncryptionAlgorithm string         `gorm:"column:encryption_algorithm;not null;type:text" json:"encryptionAlgorithm"`
	EncryptionNonce     []byte         `gorm:"column:encryption_nonce" json:"-"`
	Digest              *string        `gorm:"type:text" json:"digest,omitempty"`
	State               string         `gorm:"not null;type:text;default:active;index" json:"state"`
	CreatedByUserID     string         `gorm:"column:created_by_user_id;not null;type:text;index" json:"createdByUserId"`
	CreatedAt           time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`

	Secret        *Secret               `gorm:"foreignKey:SecretID" json:"-"`
	CreatedByUser *User                 `gorm:"foreignKey:CreatedByUserID" json:"-"`
	RecipientWrap []SecretRecipientWrap `gorm:"foreignKey:SecretVersionID" json:"-"`
}

func (SecretVersion) TableName() string { return "secret_versions" }

func (v *SecretVersion) BeforeCreate(_ *gorm.DB) error {
	if v.ID == "" {
		v.ID = id.MustNew(id.TypeSecretVersion)
	}
	if v.State == "" {
		v.State = SecretVersionStateActive
	}
	return nil
}

// SecretRecipientWrap stores a wrapped content-encryption key for a recipient.
//
// This table makes a SecretVersion decryptable by a specific recipient without
// giving Meta decrypt capability. For RecipientType=user, RecipientID is the user
// ID and RecipientKeyID is the approved UserDevice ID whose private key can
// unwrap the content-encryption key. For RecipientType=agent_session,
// RecipientID is the agent session ID and RecipientKeyID is an
// AgentSessionPublicKey ID.
//
// WrappedKey contains an encrypted content-encryption key. EncapsulatedKey stores
// HPKE encapsulated key material or equivalent recipient metadata. Meta stores
// both as opaque bytes and does not unwrap them.
type SecretRecipientWrap struct {
	ID              string         `gorm:"primaryKey;type:text" json:"id"`
	SecretVersionID string         `gorm:"column:secret_version_id;not null;type:text;uniqueIndex:idx_secret_recipient_wrap;index" json:"secretVersionId"`
	RecipientType   string         `gorm:"column:recipient_type;not null;type:text;uniqueIndex:idx_secret_recipient_wrap" json:"recipientType"`
	RecipientID     string         `gorm:"column:recipient_id;not null;type:text;uniqueIndex:idx_secret_recipient_wrap;index" json:"recipientId"`
	RecipientKeyID  string         `gorm:"column:recipient_key_id;not null;type:text;uniqueIndex:idx_secret_recipient_wrap;index" json:"recipientKeyId"`
	Algorithm       string         `gorm:"not null;type:text" json:"algorithm"`
	EncapsulatedKey []byte         `gorm:"column:encapsulated_key" json:"-"`
	WrappedKey      []byte         `gorm:"column:wrapped_key;not null" json:"-"`
	CreatedByUserID string         `gorm:"column:created_by_user_id;type:text;index" json:"createdByUserId,omitempty"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	SecretVersion *SecretVersion `gorm:"foreignKey:SecretVersionID" json:"-"`
	CreatedByUser *User          `gorm:"foreignKey:CreatedByUserID" json:"-"`
}

func (SecretRecipientWrap) TableName() string { return "secret_recipient_wraps" }

func (w *SecretRecipientWrap) BeforeCreate(_ *gorm.DB) error {
	if w.ID == "" {
		w.ID = id.MustNew(id.TypeSecretRecipientWrap)
	}
	return nil
}

// SecretBinding binds a secret to a project or agent session with usage scopes.
//
// Bindings are separate from ownership. A binding says a secret is available to
// a target, and Scopes says which runtime contexts may use it. Supported targets
// are projects and agent sessions. Supported scopes are llm, hooks, services,
// and tools.
//
// A project binding applies to all current and future agent sessions in that
// project, but every agent session still needs a SecretRecipientWrap for its
// active public key before it can decrypt the secret. An agent-session binding
// applies only to one agent session.
type SecretBinding struct {
	ID              string         `gorm:"primaryKey;type:text" json:"id"`
	SecretID        string         `gorm:"column:secret_id;not null;type:text;uniqueIndex:idx_secret_binding;index" json:"secretId"`
	TargetType      string         `gorm:"column:target_type;not null;type:text;uniqueIndex:idx_secret_binding" json:"targetType"`
	TargetID        string         `gorm:"column:target_id;not null;type:text;uniqueIndex:idx_secret_binding;index" json:"targetId"`
	Scopes          string         `gorm:"not null;type:text" json:"scopes"`
	ExpiresAt       *time.Time     `gorm:"column:expires_at;index" json:"expiresAt,omitempty"`
	CreatedByUserID string         `gorm:"column:created_by_user_id;not null;type:text;index" json:"createdByUserId"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`

	Secret        *Secret `gorm:"foreignKey:SecretID" json:"-"`
	CreatedByUser *User   `gorm:"foreignKey:CreatedByUserID" json:"-"`
}

func (SecretBinding) TableName() string { return "secret_bindings" }

func (b *SecretBinding) BeforeCreate(_ *gorm.DB) error {
	if b.ID == "" {
		b.ID = id.MustNew(id.TypeSecretBinding)
	}
	return nil
}

// JWTSigningKey stores Meta-owned JWT signing key metadata.
//
// For db-local keys, PrivateKeyEncrypted contains an encrypted private JWK/PEM
// envelope and Meta signs tokens in process after decrypting it. For external
// signer backends, BackendKeyID points at the provider key or key version and
// private key material never enters the Meta database.
type JWTSigningKey struct {
	ID                  string         `gorm:"primaryKey;type:text" json:"id"`
	OrganizationID      *string        `gorm:"column:organization_id;type:text;index" json:"organizationId,omitempty"`
	KeyID               string         `gorm:"column:kid;not null;type:text;uniqueIndex" json:"kid"`
	Algorithm           string         `gorm:"not null;type:text" json:"algorithm"`
	Backend             string         `gorm:"not null;type:text;index" json:"backend"`
	BackendKeyID        *string        `gorm:"column:backend_key_id;type:text" json:"backendKeyId,omitempty"`
	PublicJWKJSON       []byte         `gorm:"column:public_jwk_json;not null" json:"-"`
	PrivateKeyEncrypted []byte         `gorm:"column:private_key_encrypted" json:"-"`
	Status              string         `gorm:"not null;type:text;default:next;index" json:"status"`
	NotBefore           *time.Time     `gorm:"column:not_before" json:"notBefore,omitempty"`
	NotAfter            *time.Time     `gorm:"column:not_after" json:"notAfter,omitempty"`
	CreatedAt           time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt           time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`

	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"-"`
}

func (JWTSigningKey) TableName() string { return "jwt_signing_keys" }

func (k *JWTSigningKey) BeforeCreate(_ *gorm.DB) error {
	if k.ID == "" {
		k.ID = id.MustNew(id.TypeJWTSigningKey)
	}
	if k.Status == "" {
		k.Status = JWTSigningKeyStatusNext
	}
	if k.Backend == "" {
		k.Backend = JWTSigningKeyBackendDBLocal
	}
	return nil
}

// AuditEvent records security-sensitive activity in Meta.
//
// Authorization is designed to happen at the URL router layer. HTTPMethod,
// PathTemplate, Path, QueryJSON, and Action preserve the route-level decision
// context that allowed or denied a request before handlers ran.
//
// QueryJSON must contain only non-sensitive query values. If a query parameter
// can contain secret material, it should be omitted or redacted before storage.
// AuditEvent rows should be treated as append-only.
type AuditEvent struct {
	ID                 string         `gorm:"primaryKey;type:text" json:"id"`
	ActorUserID        *string        `gorm:"column:actor_user_id;type:text;index" json:"actorUserId,omitempty"`
	ActorUserSessionID *string        `gorm:"column:actor_user_session_id;type:text;index" json:"actorUserSessionId,omitempty"`
	ActorDeviceID      *string        `gorm:"column:actor_device_id;type:text;index" json:"actorDeviceId,omitempty"`
	HTTPMethod         string         `gorm:"column:http_method;not null;type:text" json:"httpMethod"`
	PathTemplate       string         `gorm:"column:path_template;not null;type:text;index" json:"pathTemplate"`
	Path               string         `gorm:"not null;type:text" json:"path"`
	QueryJSON          []byte         `gorm:"column:query_json" json:"-"`
	Action             string         `gorm:"not null;type:text;index" json:"action"`
	TargetType         string         `gorm:"column:target_type;not null;type:text;index" json:"targetType"`
	TargetID           string         `gorm:"column:target_id;not null;type:text;index" json:"targetId"`
	ProjectID          *string        `gorm:"column:project_id;type:text;index" json:"projectId,omitempty"`
	OrganizationID     *string        `gorm:"column:organization_id;type:text;index" json:"organizationId,omitempty"`
	AgentSessionID     *string        `gorm:"column:agent_session_id;type:text;index" json:"agentSessionId,omitempty"`
	Scopes             string         `gorm:"type:text" json:"scopes,omitempty"`
	Result             string         `gorm:"not null;type:text;index" json:"result"`
	MetadataJSON       []byte         `gorm:"column:metadata_json" json:"-"`
	CreatedAt          time.Time      `gorm:"autoCreateTime;index" json:"createdAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

func (AuditEvent) TableName() string { return "audit_events" }

func (e *AuditEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == "" {
		e.ID = id.MustNew(id.TypeAuditEvent)
	}
	return nil
}

// AllModels returns the Meta service models in migration order.
//
// Keep this list in dependency order so AutoMigrate can create referenced tables
// before join/dependent tables where possible.
func AllModels() []any {
	return []any{
		&Organization{},
		&User{},
		&OrganizationMember{},
		&OrganizationBootstrapToken{},
		&OAuthApplication{},
		&UserIdentity{},
		&UserDevice{},
		&UserSession{},
		&Group{},
		&GroupMember{},
		&GroupIdentity{},
		&Project{},
		&ProjectMember{},
		&AgentSession{},
		&AgentSessionPublicKey{},
		&AgentSessionMember{},
		&Secret{},
		&SecretOwner{},
		&SecretVersion{},
		&SecretRecipientWrap{},
		&SecretBinding{},
		&JWTSigningKey{},
		&AuditEvent{},
	}
}
