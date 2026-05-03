# Meta Database Structure

`meta` is Discobot's centralized metadata service. It owns concrete Discobot
metadata that multiple services need to share: organizations, organization-scoped OAuth applications, users, authenticated
user sessions, devices, groups, projects, agent sessions, secrets, and the
bindings that make secrets available to sessions.

This document is intentionally a high-level map. The authoritative field-level
model lives in [`../internal/model/models.go`](../internal/model/models.go),
including GORM tags, defaults, indexes, relationships, and detailed type
comments. Keep detailed schema semantics in code so they stay maintained with the
implementation.

`meta` is not intended to be a generic IAM system. The database should model the
specific resources Discobot cares about and the specific relationships we need
to enforce.

## ID Convention

Meta row IDs are strings with a resource prefix and a lowercase ULID suffix:

```text
org_01hx3z7h5h7m7kny4v4f2r0p5q
usr_01hx3z9m8w7t2k5v3q6yb4n2rc
sec_01hx3zb9tmd3ye4hyf4fgwzk9q
ags_01hx3zcn9j28w8d6dzmtg6h7vg
```

The prefix identifies the resource type in logs, URLs, support tools, and audit
records. The lowercase ULID suffix is sortable and opaque to callers. The prefix
registry and generator live in [`../internal/id`](../internal/id); GORM `BeforeCreate`
hooks in [`../internal/model/models.go`](../internal/model/models.go) use that
helper for all primary keys.

## High-Level Map

```text
Organization
 ├── OrganizationMember ── User
 ├── OAuthApplication
 ├── OrganizationBootstrapToken
 ├── Group
 │    ├── GroupMember ─── User
 │    ├── GroupIdentity
 │    ├── ProjectMember ─ Project
 │    └── SecretOwner ─── Secret
 ├── Project
 │    ├── ProjectMember   (users and groups)
 │    ├── AgentSession
 │    │    ├── AgentSessionMember
 │    │    └── AgentSessionPublicKey
 │    ├── SecretOwner ─── Secret
 │    └── SecretBinding   (all project sessions receive the secret)
 └── Secret
      ├── SecretOwner     (user, group, or project)
      ├── SecretVersion
      │    └── SecretRecipientWrap
      └── SecretBinding   (project/session scoped usage)

User
 ├── UserIdentity
 ├── UserSession          (authenticated login/device sessions)
 └── UserDevice           (approved client/device with public key)

AuditEvent
 └── records router-level authorization and security-sensitive activity
```

## Core Concepts

### Organizations

Organizations are the top-level hierarchy boundary. Every group, project, and
secret belongs to exactly one organization. Users join organizations through
organization membership rows. OAuth/OIDC client applications are scoped to an
organization so each organization controls its own login/client configuration.

The special public organization has domain `public`. Routes for public resources
can omit the organization segment, so `/v1/projects` is treated as the public-org
shortcut for `/v1/org/public/projects`.

Meta can also resolve an organization from the request host. If the host is an
organization domain such as `example.com`, then `/v1/projects` is treated as that
organization's projects rather than the public organization. In other words:

```text
https://meta.example.test/v1/projects      -> /v1/org/public/projects
https://example.com/v1/projects           -> /v1/org/example.com/projects
https://meta.example.test/v1/org/acme.com/projects -> /v1/org/acme.com/projects
```

Path-scoped organization routes are explicit and always win over host-derived
organization context.

### OAuth applications

OAuth applications are organization-scoped clients for login and token exchange.
The public organization owns the default public login surface. Public users can
join the public organization as members, but only public-organization admins and
owners manage OAuth applications.

### Users, sessions, and devices

A user is a human Discobot account. Users authenticate through one or more
external identities, establish user sessions, and register devices.

User sessions are authenticated browser, desktop, API-token, or refresh-token
sessions. They are distinct from agent sessions.

User devices hold private keys outside Meta, typically in the OS key store. Meta
stores public keys and approval state only. Device keys allow clients to unwrap
secret content-encryption keys locally without Meta ever seeing plaintext secret
values or plaintext content-encryption keys.

### Groups

Groups are named collections of users. They can be manually managed or linked to
third-party group identities such as AD/LDAP/OIDC/SAML groups or GitHub teams.

Groups can be project members and can own secrets. Group membership gives users
access through concrete programmatic rules rather than a generic policy engine.

### Projects

Projects are the main Discobot collaboration boundary inside an organization.
Users and groups become project members through project membership rows. Projects
contain agent sessions and can own secrets.

Project membership is the primary way to authorize project-level actions and
agent-session access.

### Agent sessions

An agent session is a Discobot coding-agent runtime/session. It is not an
authenticated login session.

Agent sessions belong to projects, can have explicit user members, and have
public keys used for secret recipient wrapping. The matching private keys live in
the agent runtime, not in Meta.

### Secrets

Secrets are sensitive values in an organization, owned by a user, group, or
project. Secret values are encrypted outside Meta and stored as encrypted secret
versions.

Secret ownership controls who can manage the secret. Secret bindings control
where the secret is available. A project binding makes a secret eligible for all
agent sessions in that project, but each agent session still needs a recipient
wrap for its public key before it can decrypt the secret.

For environment secrets, environment variable names are non-sensitive metadata;
values live only inside encrypted secret-version payloads.

### Audit events

Audit events are append-only records of security-sensitive activity. They include
router-level context such as method, path template, action, actor, target,
project/session context, scopes, and result.

## Tables

The model currently includes these tables. See
[`../internal/model/models.go`](../internal/model/models.go) for the exact fields
and semantics.

### Organization tables

- `organizations`
- `organization_members`
- `organization_bootstrap_tokens`
- `oauth_applications`

### Identity and device tables

- `users`
- `user_identities`
- `user_sessions`
- `user_devices`

### Group tables

- `groups`
- `group_members`
- `group_identities`

### Project and agent-session tables

- `projects`
- `project_members`
- `agent_sessions`
- `agent_session_public_keys`
- `agent_session_members`

### Secret tables

- `secrets`
- `secret_owners`
- `secret_versions`
- `secret_recipient_wraps`
- `secret_bindings`

### Audit tables

- `audit_events`

## Common Access Patterns

### Resolve a user's project access

```text
User
 ├── direct ProjectMember rows where member_type = user in the organization
 └── GroupMember rows for groups in the organization
      └── ProjectMember rows where member_type = group
```

### Resolve agent sessions visible to a user

```text
User
 ├── projects available through direct/group project membership
 │    └── AgentSession rows in those projects
 └── direct AgentSessionMember rows
```

### Resolve who owns a secret

```text
Secret
 └── SecretOwner
      ├── owner_type = user    -> User
      ├── owner_type = group   -> Group
      └── owner_type = project -> Project
```

### Resolve whether an agent session can use a secret

```text
AgentSession
 ├── explicit SecretBinding where target_type = agent_session
 ├── SecretRecipientWrap for the active agent-session public key
 └── Project
      └── SecretBinding where target_type = project
```

### Sync an external group

```text
GroupIdentity(provider, external_id/name)
 -> Group
 -> GroupMember rows updated from the provider
 -> ProjectMember rows grant project access to the group
```

## Related Design Documents

- [Secrets Management and Encryption](./secrets.md)
- [Database Field Encryption](./database-encryption.md)
- [OAuth Login](./oauth.md)
- [Identity and Authentication](./identity-authentication.md)
- [Authorization](./authorization.md)
