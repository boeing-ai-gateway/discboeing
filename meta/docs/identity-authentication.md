# Identity and Authentication

This document describes how users authenticate to the `meta` service and how
Discobot agents authenticate requests from those users.

The goal is for `meta` to act as Discobot's identity provider. A user signs in to
`meta`, receives standard OIDC/JWT tokens, and uses those tokens when talking to
an agent. The agent validates those tokens before accepting user requests.

This document is a working draft and may drive changes back into
[database-model.md](./database-model.md).

## Goals

- `meta` is the authority for user identity.
- `meta` issues standard OIDC-compatible JWTs.
- Agents validate JWTs locally using `meta`'s JWKS endpoint.
- Agents do not need direct database access to validate ordinary requests.
- Tokens identify the user and enough context to authorize user-to-agent access.
- User login sessions remain distinct from agent sessions.

## Terminology

| Term | Meaning |
| --- | --- |
| User | Human Discobot account |
| User session | Authenticated login/device session with `meta` |
| Agent session | Discobot coding-agent runtime/session |
| Agent | The service/runtime the user sends chat/tool-control requests to |
| ID token | OIDC token describing the authenticated user |
| Access token | JWT used to authorize API calls to Discobot services and agents |
| JWKS | Public signing keys exposed by `meta` for JWT verification |

## High-Level Flow

```text
User client
  -> authenticates to Meta
  -> receives OIDC/JWT tokens
  -> sends request to Agent with access token
  -> Agent validates token against Meta JWKS
  -> Agent authorizes user for requested agent session
  -> Agent handles request
```

More concretely:

```text
Client                         Meta                         Agent
  | login / OIDC flow           |                             |
  |---------------------------->|                             |
  |                             | authenticate user           |
  |<----------------------------| ID token + access token     |
  |                             |                             |
  | request agent action        |                             |
  | Authorization: Bearer JWT   |                             |
  |---------------------------------------------------------->|
  |                             |                             | fetch/cache JWKS
  |                             |<----------------------------|
  |                             | JWKS                        |
  |                             |---------------------------->|
  |                             |                             | validate JWT
  |                             |                             | check claims/session access
  |<----------------------------------------------------------|
  | response                    |                             |
```

## Meta as an OIDC Provider

`meta` should expose standard OIDC provider endpoints:

| Endpoint | Purpose |
| --- | --- |
| `GET /.well-known/openid-configuration` | OIDC discovery metadata |
| `GET /.well-known/jwks.json` | Public signing keys |
| `GET /authorize` | Browser-based authorization flow |
| `POST /token` | Authorization-code/token exchange |
| `GET /userinfo` | User profile claims |
| `POST /logout` | Optional logout/session revocation |

OAuth applications are scoped to organizations. `meta` may authenticate users
through upstream identity providers, but agents should only need to trust `meta`
as the issuer.

## Token Types

### ID Token

The ID token is for the client. It describes the authenticated user and login
event.

Typical claims:

| Claim | Meaning |
| --- | --- |
| `iss` | Meta issuer URL |
| `sub` | Stable user ID |
| `aud` | OAuth/OIDC client ID |
| `exp`, `iat`, `nbf` | Token timing |
| `email` | User email |
| `email_verified` | Email verification state |
| `name` | Display name |
| `picture` | Optional avatar URL |

Agents generally should not use ID tokens for API authorization. They should use
access tokens.

### Access Token

The access token authorizes calls to services and agents.

Typical claims:

| Claim | Meaning |
| --- | --- |
| `iss` | Meta issuer URL |
| `sub` | Stable user ID |
| `aud` | Per-agent-session audience, such as `agent-session:sess_456` |
| `exp`, `iat`, `nbf` | Token timing |
| `jti` | Token ID for audit/revocation correlation |
| `scope` | Space-delimited OAuth scopes |
| `sid` | User session ID |
| `device_id` | User device ID, when applicable |
| `organization_id` | Optional organization context |
| `organization_domain` | Optional route-facing organization domain |
| `project_id` | Optional project context |
| `agent_session_id` | Optional agent session context |
| `roles` | Optional compact role/context data |

The access token should be short-lived. A user session or refresh flow can mint
new access tokens when needed.

## User-to-Agent Authorization

An agent should validate two things:

1. **Authentication:** Is this token valid and issued by `meta`?
2. **Authorization:** Is this user allowed to talk to this agent session?

### Authentication checks

The agent validates the JWT by checking:

- signature against `meta` JWKS
- `iss` matches configured Meta issuer
- `aud` is expected by the agent
- `exp`, `nbf`, and `iat` are valid
- token algorithm is allowed
- required claims are present

### Authorization checks

For authorization, the token can carry enough context for common checks, or the
agent can call back to `meta` for an authorization decision.

Initial recommendation:

- Use JWT claims for the common path.
- Include `project_id` and/or `agent_session_id` when minting a token for a
  specific agent session.
- Keep tokens short-lived so membership changes converge quickly.
- Use a Meta introspection/authorization endpoint for high-risk or ambiguous
  cases.

A valid token for an agent request should prove one of:

- the user is an explicit member of the agent session
- the user has project membership that allows access to that agent session
- the user belongs to a group with project membership that allows access

## Token Audience

Agents should accept tokens scoped to their specific agent session. The audience
is not shared across all agents.

Use per-agent-session audiences:

```text
aud = agent-session:sess_123
agent_session_id = sess_123
project_id = proj_123
```

Each agent should accept only tokens whose `aud` matches its own agent session
ID. The `agent_session_id` claim is still useful for logging, audit, and defense
in depth, but the audience itself should constrain the token to one agent
session.

## Agent Scopes

Agent scopes should stay explicit and evolve as the agent-side authorization
model becomes clearer.

Initial candidate scopes:

| Scope | Meaning |
| --- | --- |
| `agent.chat` | Send user messages or continue a turn |
| `agent.read` | Read agent/session state needed by the UI |
| `agent.cancel` | Cancel an active turn |

Additional scopes should be added when the agent side has concrete authorization
checks for them. The token exchange flow can request a subset of scopes for a
specific agent session.

## Possible API Endpoints

These endpoints are illustrative.

### OIDC Discovery

```http
GET /.well-known/openid-configuration
```

Response includes issuer, authorization endpoint, token endpoint, JWKS URI,
supported signing algorithms, scopes, and claims.

### JWKS

```http
GET /.well-known/jwks.json
```

Agents cache these keys and refresh on unknown `kid` or cache expiry.

### Login / Authorization

```http
GET /authorize?client_id=...&redirect_uri=...&response_type=code&scope=openid%20profile%20agent&state=...&nonce=...
```

User authenticates to `meta` or an upstream provider. `meta` returns an
authorization code to the client redirect URI.

### Token Exchange

```http
POST /token
```

Request:

```text
grant_type=authorization_code
code=...
redirect_uri=...
code_verifier=...
```

Response:

```json
{
  "token_type": "Bearer",
  "expires_in": 900,
  "id_token": "eyJ...",
  "access_token": "eyJ...",
  "refresh_token": "opaque-or-jwt-refresh-token"
}
```

### OAuth token exchange for an agent access token

A client may already have a user session/refresh token and need a short-lived
access token for one agent session. Prefer doing this through OAuth token
exchange instead of a Discobot-specific mint endpoint.

```http
POST /token
```

Request:

```text
grant_type=urn:ietf:params:oauth:grant-type:token-exchange
subject_token=eyJ...
subject_token_type=urn:ietf:params:oauth:token-type:access_token
audience=agent-session:sess_456
scope=agent.chat agent.read agent.cancel
```

Response:

```json
{
  "token_type": "Bearer",
  "issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
  "expires_in": 900,
  "access_token": "eyJ..."
}
```

Token claims include:

```json
{
  "iss": "https://meta.example.com",
  "sub": "usr_alice",
  "aud": "agent-session:sess_456",
  "scope": "agent.chat agent.read agent.cancel",
  "organization_id": "org_123",
  "organization_domain": "example.com",
  "project_id": "proj_123",
  "agent_session_id": "sess_456",
  "sid": "usess_789",
  "device_id": "dev_123"
}
```

A Discobot-specific endpoint can still be added as a convenience wrapper later,
but the canonical flow should be OAuth-compatible.

### Agent token validation path

The client calls the agent:

```http
POST /agent/chat
Authorization: Bearer eyJ...
```

The agent validates:

- token signature using Meta JWKS
- issuer
- audience equals `agent-session:{thisSessionID}`
- expiry
- `agent_session_id` equals this agent session
- scope includes `agent.chat`

## Revocation and Expiry

Because JWT validation is local, revocation is not instant unless the agent calls
back to `meta`.

Recommended baseline:

- access tokens are short-lived, around 5 to 15 minutes
- user sessions can be revoked in `meta`
- refresh tokens or browser sessions stop minting new access tokens after
  revocation
- agents can call introspection for high-risk operations or when a token is near
  expiry

Possible introspection endpoint:

```http
POST /v1/tokens/introspect
```

Request:

```json
{"token": "eyJ..."}
```

Response:

```json
{
  "active": true,
  "sub": "usr_alice",
  "organizationId": "org_123",
  "organizationDomain": "example.com",
  "projectId": "proj_123",
  "agentSessionId": "sess_456",
  "scopes": ["agent.chat"]
}
```

## Relationship to Secret Encryption

Identity/authentication proves who the user is and whether they can talk to an
agent. It does not give `meta` the ability to decrypt secrets.

The user device key model from [secrets.md](./secrets.md) remains separate:

- login tokens authenticate API requests
- device private keys unwrap secret CEKs locally
- agent-session private keys unwrap session secret bundles locally
- `meta` stores tokens, public keys, ciphertext, and wraps, but not plaintext
  secret values

## Open Questions

- Should access tokens contain project/session authorization claims directly, or
  should agents call `meta` for authorization every time? The current bias is to
  use short-lived scoped JWTs for common checks and reserve Meta callbacks for
  high-risk or ambiguous cases.
- What additional scopes do agents need beyond the initial candidate list?
- How should token revocation be handled for long-running streaming requests?
