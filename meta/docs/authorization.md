# Authorization

This document describes how authorization should work in the `meta` service and
for Discobot agents.

Authorization has two layers:

1. **Agent authorization**: agents authorize JWT access tokens using scopes and
   token claims issued by `meta`.
2. **Meta authorization**: `meta` authorizes its own API operations
   programmatically using internal rules over organizations, organization members,
   users, groups, projects, project members, agent sessions, secrets, bindings,
   and devices.

`meta` is not intended to be a generic policy engine. Authorization rules should
be concrete, understandable, and tied directly to Discobot resources.

## Goals

- Keep agent authorization simple and scope-based.
- Keep Meta authorization explicit and programmatic.
- Enforce Meta authorization at the URL router layer before handlers run.
- Avoid generic resource/policy abstractions until the model proves it needs
  them.
- Make organization membership the top-level boundary and project membership the
  main collaboration boundary inside an organization.
- Scope OAuth applications to organizations.
- Keep secret ownership separate from secret binding/use.
- Make admin/owner actions auditable.

## Agent Authorization

Agents receive JWT access tokens issued by `meta`. An agent validates:

- token signature using Meta JWKS
- issuer
- per-agent-session audience, such as `agent-session:sess_123`
- expiry and timing claims
- required scopes
- `agent_session_id` matches the agent session being addressed

Agent authorization should primarily be driven by token scopes.

## Agent Scopes

Initial candidate scopes:

| Scope | Allows |
| --- | --- |
| `agent.chat` | Send a user message or continue a turn |
| `agent.read` | Read agent/session state needed by the UI |
| `agent.cancel` | Cancel an active turn |

Future possible scopes:

| Scope | Allows |
| --- | --- |
| `agent.files.read` | Read files or diffs exposed by the agent runtime |
| `agent.terminal` | Interact with terminal-like capabilities |
| `agent.secrets.resolve` | Request scoped encrypted secret envelopes for the agent session |
| `agent.admin` | Perform high-trust administrative actions for the agent session |

The exact scope list should evolve with the agent-side authorization checks. A
scope should only be added when there is a concrete agent operation that checks
it.

## Meta Authorization

Meta API authorization is programmatic. It should use concrete database state and
internal service rules, not externally-authored generic policies.

The main inputs are:

- effective organization, resolved from explicit path segment or request host
- authenticated user ID
- active user session and device state
- organization membership
- project membership
- group membership
- agent session membership
- secret ownership
- secret binding target and scopes

## Router-Level Authorization

Meta authorization should happen at the URL router layer before request handlers
execute.

The router should be able to determine the action being attempted from:

- authenticated user
- HTTP method
- URL path
- path parameters
- query parameters

That should be enough to map a request to an internal action such as
`project.read`, `project.member.add`, `agent_session.chat`, `secret.bind`, or
`device.approve`. The authorization middleware evaluates that action against
Meta's internal rules and either accepts or rejects the request before the handler
runs.

This gives us a few important properties:

- handlers can assume authorization for the route action already happened
- authorization decisions are centralized and testable at the route table level
- request bodies are not needed to determine the primary action
- sensitive operations fail before mutating service logic runs
- audit metadata can consistently include method, path, action, user, and result

Request bodies may still be validated by handlers for correctness, but they
should not be required to decide what broad action is being attempted. If a route
needs body content to determine authorization, prefer splitting it into more
specific routes or moving the decision into explicit path/query parameters.

Example route-action mapping:

| Method | Path | Query | Action |
| --- | --- | --- | --- |
| `GET` | `/v1/org/{organizationDomain}/projects/{projectId}` | | `project.read` |
| `GET` | `/v1/projects/{projectId}` | host/default org | `project.read` |
| `PATCH` | `/v1/projects/{projectId}` | | `project.update` |
| `POST` | `/v1/projects/{projectId}/members` | | `project.member.add` |
| `DELETE` | `/v1/projects/{projectId}/members/{memberId}` | | `project.member.remove` |
| `POST` | `/v1/projects/{projectId}/agent-sessions` | | `agent_session.create` |
| `POST` | `/v1/projects/{projectId}/agent-sessions/{sessionId}/token` | `scope=agent.chat` | `agent_token.mint` |
| `POST` | `/v1/secrets/{secretId}/bindings` | | `secret.bind` |
| `DELETE` | `/v1/secrets/{secretId}/bindings/{bindingId}` | | `secret.binding.remove` |
| `POST` | `/v1/users/me/devices/{deviceId}/approve` | | `device.approve` |

## Roles


### Organization roles

Organization roles apply to users through `organization_members`.

| Role | Meaning |
| --- | --- |
| `owner` | Full control of the organization and its resources |
| `admin` | Manage organization-level resources, members, groups, and projects |
| `member` | Participate in the organization where project/group membership permits |

Baseline role hierarchy:

```text
owner > admin > member
```

### Project roles

Project roles apply to users and groups through `project_members`.

| Role | Meaning |
| --- | --- |
| `owner` | Full control of the project, including destructive/admin operations |
| `admin` | Manage project settings, members, agent sessions, and project secrets |
| `member` | Use the project and participate in agent sessions |
| `viewer` | Read-only access to project metadata and visible sessions |

Baseline role hierarchy:

```text
owner > admin > member > viewer
```

### Agent session roles

Agent session roles apply directly to users through `agent_session_members`.

| Role | Meaning |
| --- | --- |
| `owner` | Full control of the agent session |
| `editor` | Interact with the agent and modify session state |
| `viewer` | Read session state, but not send chat/control actions |

Baseline role hierarchy:

```text
owner > editor > viewer
```

### Group roles

Group roles apply to users through `group_members`.

| Role | Meaning |
| --- | --- |
| `admin` | Manage group membership and group-owned secrets |
| `member` | Inherit group project membership and use group-owned secrets when bound |

Baseline role hierarchy:

```text
admin > member
```

### Secret owner capabilities

Secrets are owned by a user, group, or project. Ownership is not the same as a
binding.

Owners can manage the secret. Bindings decide where the secret is available.

| Owner type | Who can manage |
| --- | --- |
| `user` | The owning user |
| `group` | Group admins |
| `project` | Project owners and admins |


## Organization Authorization Rules

### Read organization

Allowed if any of these are true:

- user is an organization member
- user has access to a project, group, agent session, or secret in the organization

### Update organization metadata

Allowed for:

- organization `owner`
- organization `admin`

### Add/update/remove organization members

Allowed for:

- organization `owner`
- organization `admin`

Restrictions:

- only an `owner` can add, remove, or demote another `owner`
- an `admin` cannot grant `owner`
- a user should not remove the last organization owner

### Manage OAuth applications

Allowed for:

- organization `owner`
- organization `admin`

OAuth applications are scoped to one organization. Public organization owners and
admins manage OAuth applications for the default public login surface. Public
users that self-join the public organization are ordinary `member`s and cannot
manage OAuth applications.

### Public organization shortcuts

The organization with domain `public` can be omitted from public-org routes. For
example, `/v1/projects` on the default Meta host is authorized as if it were
`/v1/org/public/projects`. If the request host itself is an organization domain,
such as `example.com`, then `/v1/projects` is authorized as if it were
`/v1/org/example.com/projects`. Explicit `/v1/org/{organizationDomain}/...`
routes always win over host-derived organization context. Router-level
authorization should resolve the effective organization before evaluating the
action.

## Project Authorization Rules

### List projects in organization

Allowed for organization members, but the result set is filtered:

- organization `owner` and `admin` can see all projects in the organization
- organization `member` can see only projects where they are a direct project
  member, belong to a group project member, or are an explicit member of an agent
  session in the project

This is especially important for the public organization: public users can join
and create projects, but they should only see projects they created, were added
to, or can access through group membership.

### Create project in organization

Allowed for:

- organization `owner`
- organization `admin`
- organization `member`

When a member creates a project, Meta should also create a direct project
membership granting the creator `owner` on that project.

### Read project

Allowed if any of these are true:

- user is organization `owner` or `admin`
- user is a direct project member
- user belongs to a group that is a project member
- user is an explicit member of an agent session in the project

### Update project metadata

Allowed for:

- organization `owner`
- organization `admin`
- project `owner`
- project `admin`

### Delete/archive project

Allowed for:

- organization `owner`
- organization `admin`
- project `owner`

### List project members

Allowed for:

- organization `owner`
- organization `admin`
- project `owner`
- project `admin`
- project `member`

May be allowed for `viewer` if the UI needs member visibility, but default to no
until needed.

### Add/update/remove project members

Allowed for:

- organization `owner`
- organization `admin`
- project `owner`
- project `admin`

Restrictions:

- only an `owner` can add, remove, or demote another project `owner`
- a project `admin` cannot grant project `owner`
- a user should not remove the last project owner

## Group Authorization Rules

### Read group metadata

Allowed for:

- group `admin`
- group `member`
- organization `owner` or `admin`
- project admins/owners of projects where the group is a member

### Manage group membership

Allowed for:

- group `admin`
- organization `owner` or `admin`

External group-backed membership may be read-only in Meta if membership is owned
by the upstream identity provider.

### Link or unlink external group identity

Allowed for:

- group `admin`
- organization `owner` or `admin`

## Agent Session Authorization Rules

### Create agent session in project

Allowed for:

- project `owner`
- project `admin`
- project `member`

Not allowed for:

- project `viewer`

### Read agent session

Allowed if any of these are true:

- user is an explicit agent session member with `viewer` or higher
- user has project `viewer` or higher through direct or group membership

### Send chat / continue turn

Allowed if any of these are true:

- user is an explicit agent session member with `editor` or higher
- user has project `member` or higher through direct or group membership

When minting an agent token, this maps to `agent.chat`.

### Cancel turn

Allowed if any of these are true:

- user is an explicit agent session member with `editor` or higher
- user has project `member` or higher through direct or group membership

When minting an agent token, this maps to `agent.cancel`.

### Manage agent session members

Allowed for:

- explicit agent session `owner`
- project `owner`
- project `admin`

### Delete/archive agent session

Allowed for:

- explicit agent session `owner`
- project `owner`
- project `admin`

## Secret Authorization Rules

Secret authorization has two separate questions:

1. Can the user manage the secret?
2. Can the user bind or provision the secret to a project or agent session?

### Manage user-owned secret

Allowed for:

- owning user

### Manage group-owned secret

Allowed for:

- group `admin`

### Manage project-owned secret

Allowed for:

- project `owner`
- project `admin`

### Create user-owned secret

Allowed for:

- authenticated organization member creating a secret owned by themselves in that organization

### Create group-owned secret

Allowed for:

- group `admin`
- organization `owner` or `admin`

### Create project-owned secret

Allowed for:

- organization `owner`
- organization `admin`
- project `owner`
- project `admin`

### Rotate secret

Allowed for the same users who can manage the secret owner.

Rotation still happens client-side from a crypto perspective. Meta only stores
the new encrypted version and submitted recipient wraps.

### Delete/disable secret

Allowed for the same users who can manage the secret owner.

Destructive actions should be audited.

## Secret Binding Rules

Bindings make a secret available to a target with scopes.

Supported targets:

- `project`
- `agent_session`

Supported scopes:

- `llm`
- `hooks`
- `services`
- `tools`

### Bind secret to project

Allowed if the user can manage the secret and can administer the target project.

Target project administration means:

- project `owner`
- project `admin`

This rule applies regardless of whether the secret owner is a user, group, or
project.

Examples:

- Alice can bind her user-owned secret to Project A if she is admin/owner of
  Project A.
- A group admin can bind a group-owned secret to Project A if they are also
  admin/owner of Project A.
- A project admin can bind a project-owned secret to that project.

### Bind secret to agent session

Allowed if the user can manage the secret and can administer the target agent
session.

Target agent-session administration means:

- explicit agent session `owner`
- project `owner`
- project `admin`

### Remove binding

Allowed if either:

- user can manage the secret
- user can administer the binding target

This lets project admins remove a problematic binding from their project even if
they do not own the underlying secret.

### Resolve secret for agent session

Allowed when:

- the secret has an effective project or agent-session binding
- the requested scope is included in the binding
- the agent session is the target or belongs to the bound project
- a recipient wrap exists for the active agent-session public key

Meta still returns only encrypted payloads and recipient wraps.

## User Device Authorization Rules

User devices are used for secret encryption and rewrap flows.

### Register first device

Allowed when:

- user is authenticated
- user has no active devices

The device can become `active` immediately because there is no existing device to
approve from.

### Register additional device

Allowed when:

- user is authenticated

The device starts as `pending` and cannot unwrap existing secrets until approved.

### Approve pending device

Allowed when:

- approving device belongs to the same user
- approving device is `active`
- pending device belongs to the same user

The approving device creates recipient wraps for the pending device. Meta stores
the wraps and marks the device `active`.

### Disable/delete device

Allowed for:

- owning user

If this is the last active device, warn the user that existing secrets may become
unrecoverable and must be re-entered or rotated.

## Token Minting Rules

Meta should mint agent-scoped access tokens only after checking internal rules.

### Mint `agent.read`

Allowed when the user can read the agent session.

### Mint `agent.chat`

Allowed when the user can send chat / continue turns for the agent session.

### Mint `agent.cancel`

Allowed when the user can cancel turns for the agent session.

Tokens should use:

```text
aud = agent-session:{agentSessionID}
agent_session_id = {agentSessionID}
project_id = {projectID}
scope = requested allowed scopes
```

If the user requests multiple scopes, Meta should grant only the subset the user
is allowed to use.

## Audit Requirements

Audit at least:

- organization member added/updated/removed
- OAuth application created/updated/deleted
- project member added/updated/removed
- group member added/updated/removed
- external group identity linked/unlinked
- agent session created/updated/deleted
- agent session member added/updated/removed
- secret created/rotated/disabled/deleted
- secret binding added/removed
- secret recipient wrap added
- user device registered/approved/disabled/deleted
- agent-scoped token minted
- authorization denied for sensitive operations

Audit records should include:

- acting user
- acting user session/device, when available
- target type and ID
- project ID, when relevant
- agent session ID, when relevant
- scopes, when relevant
- result: `success`, `denied`, or `error`

## Open Questions

- Should public organization membership be created automatically for every
  authenticated public user at first login?
- Do we need a project `viewer` role immediately, or should we start with only
  `owner`, `admin`, and `member`?
- Should explicit agent session membership override project membership or only
  add access for users outside the project?
- Should project admins be able to remove any project binding, even if the secret
  is externally owned? This document says yes, but that should be confirmed.
- Do group-owned secrets require group admin plus project admin to bind, or should
  group admin alone be enough for some targets?
- Which additional agent scopes should exist once agent-side authorization is
  implemented?
