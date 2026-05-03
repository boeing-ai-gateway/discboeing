# OAuth Login

This document describes how OAuth/OIDC login should work in the `meta` service.

OAuth is the first identity feature we want to implement. The design goal is to
let an organization configure one or more friendly OAuth providers, use the
request domain to resolve the organization, and link users by verified email.

## Goals

- OAuth providers are scoped to organizations.
- Organization domain determines which organization is handling the login.
- Provider IDs are friendly URL identifiers, such as `google` or `github`, not
  random database IDs.
- Meta links users by verified email initially.
- Meta records the upstream provider subject for future stability and audit.
- Organizations can decide whether new users may auto-join and what role they
  receive.
- The model should leave room for future non-email identity keys, such as AD
  object IDs or enterprise IdP subjects.

## Bootstrap Token Setup

Before an organization has any real owner/admin, Meta uses an organization
bootstrap token as a narrow setup principal.

On startup, Meta ensures the public organization exists. If that organization
has no real owner/admin, Meta enters setup mode: it revokes any existing active
bootstrap tokens for the organization, creates a fresh token, and prints the raw
token once. This means every startup during setup produces a new valid bootstrap
token and invalidates older ones.

The raw token has 260 bits of cryptographic randomness and is stored only as a
SHA-256 lookup hash in `organization_bootstrap_tokens`. This is intentionally not
an encrypted field: Meta never needs to recover the raw token, only verify a
presented bearer token. A password-based hash such as bcrypt, scrypt, PBKDF2, or
Argon2 is unnecessary for this high-entropy random token; those algorithms are
for low-entropy user-chosen passwords.

The bootstrap token can authenticate only as the bootstrap role for its
organization. Initially, that role should be limited to:

- reading setup status
- configuring the first OAuth provider for the organization
- rotating or revoking bootstrap tokens, if needed

It cannot act as a normal user, create projects, read or bind secrets, or manage
ordinary users.

After the first real verified OAuth login succeeds for an organization with no
owner/admin, Meta should add that real user as an organization admin/owner. On
subsequent startup, Meta detects that the organization is initialized and revokes
any lingering active bootstrap tokens.

## Organization-Domain Routing

Organizations are fundamentally linked to domains. When a request comes in, Meta
can resolve the organization from the request host.

Examples:

```text
https://example.com/oauth/google/login
  -> organization domain: example.com
  -> provider ID: google

https://acme.test/oauth/github/login
  -> organization domain: acme.test
  -> provider ID: github

https://meta.example.test/v1/org/example.com/oauth/google/login
  -> organization domain: example.com
  -> provider ID: google
```

The public organization still exists as a special default. On the default Meta
host, routes that omit an organization domain resolve to `public`.

```text
https://meta.example.test/oauth/google/login
  -> organization domain: public
  -> provider ID: google
```

Explicit organization routes should always win over host-derived organization
context.

## Friendly Provider IDs

OAuth provider URLs should use a friendly organization-local provider ID:

```text
/oauth/{providerId}/login
/oauth/{providerId}/callback

/v1/org/{organizationDomain}/oauth/{providerId}/login
/v1/org/{organizationDomain}/oauth/{providerId}/callback
```

Examples:

```text
/oauth/google/login
/oauth/github/login
/v1/org/example.com/oauth/google/login
```

`providerId` should be:

- lowercase
- URL-safe
- unique within the organization
- human-readable

Good provider IDs:

```text
google
github
microsoft
okta
adfs
```

The database can still have an internal row ID for the provider/application, but
that ID should not be the primary URL identifier for login flows.

## OAuth Provider Configuration

OAuth providers should be configured per organization.

Conceptually, an organization OAuth provider has:

| Field | Meaning |
| --- | --- |
| internal ID | Stable Meta row ID |
| organization ID | Organization that owns the provider |
| provider ID | Friendly URL ID, such as `google` |
| type | Provider implementation type, such as `oidc`, `google`, `github` |
| display name | Human-readable label |
| client ID | OAuth client ID |
| client secret | Stored as a Meta secret or encrypted configuration |
| scopes | Provider scopes to request |
| issuer/discovery URL | OIDC issuer/discovery location, if applicable |
| enabled | Whether the provider can be used for login |

This may be represented as a new table or as an evolution of
`oauth_applications`. The important design point is that the login URL uses the
friendly `providerId`, while the database still has a proper internal ID.

## Identity Assertion Shape

Providers differ in what they return. Meta should normalize successful provider
login into a provider identity shape:

```text
ProviderIdentity
 ├── organizationDomain
 ├── providerId
 ├── providerSubject
 ├── email
 ├── emailVerified
 └── claims
```

Examples by provider:

| Provider | Stable subject | Email verification notes |
| --- | --- | --- |
| Google OIDC | `sub` | `email_verified` claim is available |
| Generic OIDC | `sub` | depends on provider; require verified email claim or configured trust |
| GitHub OAuth | numeric user ID | primary email may require `/user/emails`; use only verified email |
| Microsoft/AD | `oid` / `sub` / tenant claims | may later use object ID as canonical identity |

## Initial User Linking Rule

The initial canonical user key is verified email.

When OAuth succeeds:

1. Normalize provider identity.
2. Require a non-empty email address.
3. Require provider evidence that the email is verified.
4. Lowercase/canonicalize the email for lookup.
5. Find `users.primary_email = verifiedEmail`.
6. If no user exists, create one.
7. Create or update `user_identities` for `providerId + providerSubject`.
8. Continue organization membership checks.

Important rule:

> Do not create or link a user by email unless the provider asserts the email is
> verified, or the organization has an explicit provider-specific trust rule that
> makes the email authoritative.

If email is missing or unverified, login should fail for now.

## User Identity Rows

Even though email is the initial user-linking key, Meta should still store the
provider subject.

`user_identities` should record:

- user ID
- organization/provider context, if needed
- provider ID
- provider subject
- email returned by provider
- email verification state
- claims snapshot

This gives us:

- auditability
- future ability to move from email-based linking to subject/object-ID linking
- visibility into which external identities are attached to a user

## Organization Membership and Auto-Join

After Meta resolves or creates the user, it must decide whether the user can join
or access the organization.

Organizations should have login policy fields like:

| Field | Meaning |
| --- | --- |
| `allow_auto_join` | Whether new verified users can be added automatically |
| `default_member_role` | Role assigned on auto-join, usually `member` |

Recommended defaults:

| Organization | `allow_auto_join` | `default_member_role` |
| --- | --- | --- |
| `public` | `true` | `member` |
| private/customer org | `false` by default | `member` if enabled |

Login flow after user lookup:

```text
User + organization
 ├── if organization member exists -> continue
 ├── else if allow_auto_join -> create organization_member(role=default_member_role)
 └── else -> deny login / require invitation or admin approval
```

Public organization behavior:

- Public users can self-join as `member`.
- Public users do not become admins.
- Public users can create projects.
- When a public user creates a project, they become owner of that project.
- Public users only see projects they belong to, created, were invited to, or can
  access through group/project membership.

## OAuth Login Flow

```text
Browser
 -> GET /oauth/{providerId}/login on organization domain
 -> Meta resolves organization from host
 -> Meta loads organization provider by friendly provider ID
 -> Meta redirects to upstream OAuth provider
 -> Provider authenticates user
 -> Provider redirects to Meta callback
 -> Meta exchanges code for provider identity
 -> Meta verifies email
 -> Meta finds or creates user
 -> Meta links provider subject
 -> Meta checks organization membership / auto-join policy
 -> Meta creates user session
 -> Meta redirects to client/app
```

## Metadata Endpoints

Meta serves standard metadata documents for both OIDC and OAuth clients:

```text
GET /.well-known/openid-configuration
GET /.well-known/oauth-authorization-server
GET /.well-known/jwks.json
```

The OIDC discovery document includes the issuer, authorization endpoint, token
endpoint, userinfo endpoint, JWKS URI, supported scopes, supported claims, PKCE
methods, and ID-token signing algorithms. The OAuth authorization server metadata
document exposes the OAuth subset of the same server capabilities, including
authorization-code, refresh-token, and token-exchange grants.

The JWKS endpoint is backed by Meta's persistent signing-key store. It publishes
active keys, pre-published next keys, and retired keys that remain within the
verification overlap window. It does not require token issuance to be implemented
and never accesses private key material.

## Callback State

Meta must protect the OAuth flow with state and PKCE where applicable.

State should bind together:

- organization domain
- provider ID
- OAuth client/application
- redirect target
- nonce
- PKCE verifier reference or challenge state
- expiration

State can be stored server-side or encoded/signed, but it should not trust only
query parameters on callback to determine organization/provider.

## Suggested Routes

Host-derived organization routes:

```text
GET /whoami
GET /oauth/{providerId}/login
GET /oauth/{providerId}/callback
```

Explicit organization routes:

```text
GET /v1/org/{organizationDomain}/oauth/{providerId}/login
GET /v1/org/{organizationDomain}/oauth/{providerId}/callback
```

Provider management routes:

```text
GET    /v1/org/{organizationDomain}/oauth-providers
POST   /v1/org/{organizationDomain}/oauth-providers
GET    /v1/org/{organizationDomain}/oauth-providers/{providerId}
PATCH  /v1/org/{organizationDomain}/oauth-providers/{providerId}
DELETE /v1/org/{organizationDomain}/oauth-providers/{providerId}
```

During setup, the organization bootstrap token can authenticate these routes for
the limited purpose of creating/configuring the first provider for that
organization. After real admins exist, provider management should be authorized
for organization owners and admins only.

## Database Model Implications

The current database model has organization-scoped `oauth_applications`. OAuth
login likely needs either:

1. a dedicated `oauth_providers` table, or
2. an expanded `oauth_applications` table that includes provider login
   configuration.

Likely fields needed:

- organization ID
- friendly provider ID
- provider type
- display name
- issuer/discovery URL
- client ID
- client secret reference
- requested scopes
- enabled/status

Organizations also need login policy fields:

- `allow_auto_join`
- `default_member_role`

`user_identities` may need organization/provider context if provider IDs are only
unique inside an organization. At minimum, the uniqueness rule should not assume
that `providerId` is globally unique across all organizations.

## Open Questions

- Should provider IDs be unique per organization only, or globally reserved for
  built-in providers like `google` and `github`?
- Do we need both OAuth applications and OAuth providers, or should one model
  represent both login providers and OAuth clients?
- Should private organizations support domain allowlists in addition to
  `allow_auto_join`?
- How should invitation-only organizations handle first login before membership
  exists?
- When do we move from verified-email linking to provider-subject or AD object-ID
  linking for enterprise organizations?
