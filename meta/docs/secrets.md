# Secrets Management and Encryption

This document describes how the `meta` service stores encrypted secret envelopes,
binds secrets to projects and agent sessions, and serves those envelopes to
authorized callers.

It builds on the database model in [database-model.md](./database-model.md),
especially these tables:

- `secrets`
- `secret_owners`
- `secret_versions`
- `secret_bindings`

This document is intentionally a working draft. The secret model may still drive
changes back into the database model as the design becomes clearer.

## Summary

Secrets are sensitive values in an organization, owned by a user, group, or
project and made available to agent sessions through explicit bindings.

The core rules are:

- A secret belongs to exactly one organization.
- A secret is owned by exactly one owner: a user, group, or project in that organization.
- A secret value is stored as encrypted, versioned material.
- A secret is not automatically available everywhere just because it exists.
- A secret is bound either to one agent session or to a project.
- A project binding makes the secret available to every agent session in that
  project.
- Each binding declares the scopes where the secret may be used.

This keeps ownership simple while still allowing broad project-level defaults
and precise per-session overrides.

## Secret Types

The first supported secret types are:

| Type | Description |
| --- | --- |
| `api_key` | A single API key or static provider key |
| `oauth` | OAuth credentials with refresh token and related token metadata |
| `token` | General token material, including refresh tokens or structured token bundles |
| `environment` | A set of environment variable names with sensitive values |

### `api_key`

Use for provider keys and other static key values.

Examples:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY`
- webhook signing secret

### `oauth`

Use for OAuth-backed credentials where the system may need to refresh access.

Expected encrypted payload fields may include:

- access token
- refresh token
- expiry
- token type
- scopes returned by the provider
- provider account metadata

### `token`

Use for token-shaped credentials that are not necessarily part of a full OAuth
flow but still need structured storage.

Examples:

- refresh token bundles
- service tokens
- short-lived token exchange state that needs to be retained securely

### `environment`

Use for generic environment variable bundles.

An environment secret contains a list of key-value pairs:

```json
{
  "entries": [
    {"name": "FOO_API_URL", "value": "https://example.test"},
    {"name": "FOO_API_TOKEN", "value": "secret-token"}
  ]
}
```

Environment variable names are not considered sensitive. Values are sensitive.
That distinction matters for storage and display:

- names can appear in plaintext metadata, API responses, previews, and audit logs
- values must be encrypted and should not be returned unless a scoped secret
  resolution flow needs them
- names can be used for conflict detection before decrypting values
- values should only live inside encrypted secret-version payloads

This type is for generic bundles that do not fit provider-specific API key or
OAuth flows.

## Ownership

A secret can be owned by:

- a user
- a group
- a project

Ownership answers: **who controls the secret?**

The owner can manage the secret's metadata, rotate versions, and decide which
sessions or projects receive bindings.

Group-owned secrets are scoped to the group's organization and are useful for team-owned credentials that should not belong
to one individual user and are not tied to a single project. Group admins can
manage the secret, and the secret can still be bound to specific projects or
agent sessions.

## Bindings

A binding makes a secret available somewhere.

Supported binding targets:

| Target | Meaning |
| --- | --- |
| `agent_session` | The secret is available only to one agent session |
| `project` | The secret is available to every agent session in the project |

A project binding is the mechanism for project-wide defaults. If a secret is
bound to a project, every current and future agent session in that project should
receive the secret for the binding's allowed scopes.

An agent-session binding is the mechanism for targeted access. It can be used to
attach a user-, group-, or project-owned secret to one specific agent session.

## Scopes

Each binding declares where the secret may be used. A secret can be bound with
one or more scopes.

Initial scopes:

| Scope | Meaning |
| --- | --- |
| `llm` | Available to model provider calls and LLM-facing credential resolution |
| `hooks` | Available to workspace/session hooks |
| `services` | Available to background services configured for the session |
| `tools` | Available to agent tool execution |

Scopes answer: **what part of the system can use this bound secret?**

For example:

```text
Secret: OPENAI_API_KEY
Binding target: project/proj-123
Scopes: llm
```

This means every agent session in `proj-123` can use the key for LLM provider
calls, but hooks, services, and tools should not receive it unless those scopes
are also granted.

Another example:

```text
Secret: DEPLOY_TOKEN
Binding target: agent_session/sess-456
Scopes: tools, hooks
```

This means only session `sess-456` can use the token, and only in tool and hook
contexts.

## Effective Secret Resolution

When an agent session starts or asks for secrets, `meta` should resolve effective
bindings from two sources:

1. project bindings for the session's project
2. direct bindings for the agent session

```text
AgentSession
 ├── Project
 │    └── SecretBinding(target_type = project)
 └── SecretBinding(target_type = agent_session)
```

The returned secret set should preserve scope information so downstream services
can filter by usage context.

## Encryption Model

The `meta` service must never see plaintext secret values. Plaintext secret
material should not be present in the `meta` service process memory during
create, read, rotate, bind, or rewrap operations.

`meta` also does not need to perform cryptography itself. Its role is to store
and authorize cryptographic envelopes created by clients, agent runtimes, or an
external KMS/HSM. In the normal flow, `meta` does not encrypt, decrypt, wrap,
unwrap, or rewrap keys. It validates metadata, stores ciphertext, stores public
keys, stores recipient wraps, and serves the right envelopes to authorized
callers.

That constraint means encryption happens before data reaches `meta`, and
decryption happens only in a recipient that owns an authorized private key, such
as a user's client or an agent session runtime.

### Cryptographic shape

Use standard envelope encryption:

1. The client generates a random content encryption key (CEK) locally.
2. The client encrypts the secret payload locally with an AEAD cipher.
3. The client wraps the CEK to one or more recipient public keys.
4. The client sends only ciphertext and wrapped CEKs to `meta`.
5. `meta` stores the encrypted payload and recipient wraps.
6. A recipient downloads the ciphertext and its wrap, unwraps the CEK with its
   private key, then decrypts locally.

```text
plaintext secret
  -> client-side AEAD encryption with random CEK
  -> encrypted payload stored in secret_versions.encrypted_data

CEK
  -> wrapped to user public key and/or agent-session public key
  -> stored in secret_recipient_wraps
```

### Content encryption keys (CEKs)

CEK stands for **content encryption key**. It is the random symmetric key used to
encrypt one secret version's payload.

The CEK is not the user's private key and it is not the agent session's private
key. It is a per-secret-version data key:

```text
secret payload --encrypted with CEK--> encrypted_data
CEK            --wrapped to recipient public key--> secret_recipient_wrap
```

This is what makes efficient sharing possible. To grant a secret version to a new
recipient, the client only needs to create another wrapped CEK for that
recipient. The encrypted payload does not need to be decrypted and encrypted
again.

`meta` stores encrypted payloads and wrapped CEKs, but it must never see a
plaintext CEK.

### Recommended standards

Use a well-known public-key recipient encryption standard rather than designing a
custom scheme. These standards and libraries are for clients, agent runtimes, and
external KMS/HSM integrations. They are not requirements for `meta` to perform
cryptographic operations in-process.

Recommended baseline:

- **HPKE (RFC 9180)** for wrapping the CEK to recipient public keys
- **X25519** recipient keys
- **HKDF-SHA-256** key schedule
- **ChaCha20-Poly1305** or **AES-256-GCM** for AEAD encryption

Acceptable implementation options in Go:

- Go standard-library HPKE support, if available in the target Go version
- a widely used HPKE implementation such as Cloudflare CIRCL
- `filippo.io/age` if we decide the age file/recipient format is a better fit
  than storing our own HPKE envelope metadata

The important requirement is not the exact library choice yet. The requirement is
that clients and agent runtimes use a reviewed, standard recipient-encryption
construction and that `meta` store algorithm identifiers with each encrypted
version/wrap.

### Recipient keys

Recipients have public/private keypairs:

| Recipient | Private key location | Public key location |
| --- | --- | --- |
| User device | OS key store on the approved user device | Device public key stored in `meta` |
| Agent session | Agent session runtime/container | Stored in `meta` |

`meta` stores device public keys and key metadata. It does not store user-device
private keys or agent-session private keys.

A user may have multiple approved devices, and each approved device has its own
public key. An agent session has its own public key so secrets can be wrapped
directly to that session.

### User devices and approval

A user device is a client installation that owns a private key and has a public
key registered in `meta`.

The normal device lifecycle is:

1. User logs in on a new device.
2. The client generates a new public/private keypair locally.
3. The client stores the private key in the OS key store.
4. The client registers the device public key with `meta` as `pending`, unless
   this is the user's first device.
5. If this is the user's first device, `meta` marks it `active` after login
   because there is no existing device to approve from.
6. Otherwise, an existing active device for that user receives an approval
   request.
7. The existing device asks the user to approve the new device.
8. If approved, the existing device creates any needed recipient wraps for the new
   device and uploads them to `meta`.
9. `meta` marks the new device as `active`.

```text
New device                         meta                         Existing device
   | login + generate keypair        |                                  |
   | register pending device pubkey  |                                  |
   |-------------------------------->|                                  |
   |                                 | notify approval required         |
   |                                 |--------------------------------->|
   |                                 |                                  | user approves
   |                                 |<---------------------------------|
   |                                 | wraps for new device + approval  |
   |<--------------------------------| device active                    |
```

This model intentionally has no recovery secret. If the user loses every active
device, `meta` cannot decrypt or rewrap the user's existing secrets. The user can
still log in, register a new device, and continue using the account. If there are
no active devices left, the new device can become active after login, but secret
values that were only recoverable by lost device keys must be re-entered or
rotated.

For project- or group-owned secrets, a separate active admin device may be able
to recreate wraps if that device already has a recipient wrap for the relevant
secret version. If no authorized active device exists for a secret, the same rule
applies: the value must be re-entered or rotated.

### Creating a secret version

When a user creates or rotates a secret:

1. The user enters the secret value in a client.
2. The client builds the secret-type payload locally.
3. The client generates a CEK locally.
4. The client encrypts the payload locally.
5. The client wraps the CEK to at least one management recipient, typically the
   user's current approved device public key or an owner/group/project
   management key if we add one.
6. The client sends encrypted payload and wraps to `meta`.
7. `meta` stores the ciphertext and wraps without decrypting anything.

For `environment` secrets, variable names are not sensitive and may be stored in
plaintext metadata for display, filtering, and conflict detection. Variable
values are sensitive and must be stored only inside the encrypted version
payload. This allows the UI to show which variables a secret will provide without
exposing their values.

### Binding a secret to an agent session

Binding a secret to an agent session must result in the CEK being wrapped to that
agent session's public key.

There are two valid ways to do that without exposing plaintext to `meta`:

1. **Client-assisted rewrap**
   - An authorized user client downloads an existing wrap it can decrypt.
   - The client unwraps the CEK locally.
   - The client wraps the CEK to the agent session public key.
   - The client uploads the new recipient wrap to `meta`.

2. **External KMS/HSM-assisted rewrap**
   - An authorized client or control-plane worker requests rewrap from an
     external key system.
   - The external key system holds an owner private key or wrapping key.
   - The external key system returns only the new wrap for the agent session
     public key.
   - The plaintext CEK is never returned to `meta`.
   - This is acceptable only if the KMS/HSM boundary, not the `meta` process, is
     trusted to handle the unwrapped key.

`meta` does not independently rewrap secrets. It can accept a recipient wrap
produced by an authorized client, agent runtime, or external KMS/HSM, but it must
not unwrap the CEK itself. If `meta` can unwrap the CEK, the zero-plaintext
requirement is violated.

### Creating an agent-session secret bundle

When a user creates an agent session, `meta` and the client can cooperate to
produce the session's encrypted secret bundle without `meta` seeing plaintext.

Example scenario:

- the user owns three secrets
- the project owns three secrets
- the user creates a new agent session in that project
- all six secrets should be available to the new session for their allowed scopes

The efficient flow is batch-oriented:

```text
Client                         meta                         Agent session
  | create session request       |                                  |
  |----------------------------->|                                  |
  |                              | create agent session             |
  |                              | generate/register session pubkey |
  |<-----------------------------| session id + session public key  |
  |                              |                                  |
  | ask for bundle plan          |                                  |
  |----------------------------->|                                  |
  |                              | resolve effective secret bindings|
  |                              | find versions and device wraps   |
  |<-----------------------------| encrypted versions + device wraps|
  |                              |                                  |
  | unwrap CEKs locally          |                                  |
  | wrap CEKs to session pubkey  |                                  |
  | upload session wraps         |                                  |
  |----------------------------->| store recipient wraps            |
  |                              |                                  |
  | start/continue session       |                                  |
  |----------------------------->| provide ciphertext + wraps       |
  |                              |--------------------------------->|
  |                              |                                  | decrypt locally
```

The key optimization is that the client does not decrypt and re-encrypt every
secret payload. It only unwraps each secret's content-encryption key (CEK) and
wraps that CEK to the agent session's public key. The encrypted secret payload
stays unchanged.

For six secrets, the client performs six local unwrap operations and six local
wrap operations, then uploads six `secret_recipient_wraps` in one batch.

#### Concrete example

Assume Alice creates a new agent session in project `proj-123`.

Effective secrets:

| Secret | Owner | Binding source | Scopes |
| --- | --- | --- | --- |
| `ANTHROPIC_API_KEY` | Alice | direct session binding | `llm` |
| `GITHUB_TOKEN` | Alice | direct session binding | `tools` |
| `LOCAL_ENV` | Alice | direct session binding | `hooks`, `tools` |
| `PROJECT_OPENAI_KEY` | Project | project binding | `llm` |
| `DEPLOY_ENV` | Project | project binding | `services`, `hooks` |
| `OBSERVABILITY_TOKEN` | Project | project binding | `services` |

`meta` returns a bundle plan with six encrypted secret versions and six existing
wraps addressed to Alice's current approved device public key. Alice's client
unwraps the six CEKs locally and creates six new wraps addressed to the agent
session public key.

After upload, the agent session has a decryptable bundle:

```text
AgentSession sess-456
 ├── ANTHROPIC_API_KEY      scopes: llm
 ├── GITHUB_TOKEN           scopes: tools
 ├── LOCAL_ENV              scopes: hooks, tools
 ├── PROJECT_OPENAI_KEY     scopes: llm
 ├── DEPLOY_ENV             scopes: services, hooks
 └── OBSERVABILITY_TOKEN    scopes: services
```

`meta` still has only ciphertext and recipient wraps. Alice's client saw CEKs
briefly, but it did not need to decrypt the secret payload values. The agent
session decrypts the values locally when it needs to materialize a scoped runtime
environment.

#### Bundle plan

`meta` can prepare a bundle plan containing only ciphertext and wrapping metadata:

- agent session ID
- agent session public key ID and public key
- secret IDs and version IDs
- encrypted payload metadata
- the existing recipient wrap for one of the requesting user's approved devices,
  when available
- target scopes for each secret
- whether the binding came from the project or directly from the session

The bundle plan includes only secrets that are both authorized for the session
and decryptable by the requesting device. For user-owned secrets, that usually
means the requesting device already has a wrap. For project- or group-owned
secrets, it means the requesting device has a wrap because the user can manage or
use that owner context. If a secret is authorized but the requesting device has
no usable wrap, `meta` should return `needs_rewrap` and identify which active
user/device or external KMS flow can satisfy it, if known.

The bundle plan must not contain plaintext secret values or plaintext CEKs.

#### Client-side batch rewrap

The client processes the plan locally:

1. For each secret version, select a wrap addressed to one of the user's active
   devices.
2. Use the current device's private key from the OS key store to unwrap the CEK locally.
3. Use the agent session public key to wrap that CEK.
4. Create a `secret_recipient_wraps` upload record.
5. Zero local plaintext CEK memory as soon as the wrap is produced, where the
   platform allows it.
6. Upload all new wraps in one batch.

The client never needs to decrypt the encrypted payload unless it is displaying
or editing the secret value. For session provisioning, rewrapping the CEK is
sufficient.

#### Server-side validation

When accepting the uploaded wraps, `meta` validates authorization and consistency
without decrypting anything:

- the requesting user may create the agent session
- the requesting user may bind or use each selected secret
- each secret is effectively bound to the agent session or project with the
  requested scopes
- each wrap targets the correct agent-session public key
- each wrap references the current or requested secret version
- duplicate wraps are idempotent

`meta` still cannot verify that the uploaded wrap decrypts to the correct CEK
unless the cryptographic envelope provides a public/verifiable binding. The
agent session will fail to decrypt if a bad wrap is uploaded. We can reduce user
visible failures by having the client verify its own wrap locally before upload
when the chosen library supports that efficiently.

#### Agent-session startup

At startup, the agent session receives or fetches:

- encrypted secret payloads
- recipient wraps addressed to its active public key
- scopes for each secret binding
- non-sensitive metadata, such as environment variable names

The agent session uses its private key locally to unwrap each CEK and decrypt the
payloads it is allowed to use. Plaintext values exist only inside the agent
session runtime and only for the scopes that requested them.

#### Missing wraps

A project binding may authorize a secret for a new session before a recipient
wrap exists for that session.

In that case, `meta` should report the secret as `needs_rewrap` rather than
silently omitting it. The UI can then ask an authorized client to complete the
batch rewrap flow.

Possible states for an effective session secret:

| State | Meaning |
| --- | --- |
| `ready` | Binding exists and a session recipient wrap exists |
| `needs_rewrap` | Binding exists but no wrap exists for the session key |
| `unauthorized` | Caller cannot create the needed wrap or binding |
| `stale` | Wrap exists for an old secret version or inactive session key |

#### Efficiency notes

The expensive work scales with the number of secrets, not the size of each
secret payload, because only CEKs are rewrapped.

For common session creation:

- resolve all effective bindings in one `meta` request
- return all device-addressed wraps in one bundle plan
- upload all agent-session wraps in one batch
- avoid decrypting secret payloads during rewrap
- avoid one network round trip per secret

This makes the three personal secrets plus three project secrets case a single
planning request and a single wrap upload request.

#### Possible API endpoints

These endpoints are illustrative. The exact route shape can change, but the
important part is that APIs exchange public keys, ciphertext, wraps, scopes, and
metadata only.

##### Register pending user device

```http
POST /v1/users/me/devices
```

Request:

```json
{
  "name": "Alice's MacBook",
  "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
  "publicKey": "base64..."
}
```

If this is the user's first device, `meta` can return `active`; otherwise it
returns `pending` until an existing active device approves it.

Response:

```json
{
  "id": "dev_123",
  "status": "pending"
}
```

A pending device can authenticate the user, but it cannot decrypt existing
secrets until an active device approves it and creates wraps for it.

##### List pending device approvals

```http
GET /v1/users/me/devices/pending
```

Response shown on an existing active device:

```json
{
  "items": [
    {
      "id": "dev_123",
      "name": "Alice's MacBook",
      "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
      "publicKey": "base64...",
      "createdAt": "2026-04-30T18:00:00Z"
    }
  ]
}
```

##### Plan new-device approval

```http
POST /v1/users/me/devices/{deviceId}/approval-plan
```

Response contains existing wraps for secrets the approving device can rewrap:

```json
{
  "pendingDevice": {
    "id": "dev_123",
    "publicKey": "base64...",
    "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305"
  },
  "items": [
    {
      "secretId": "sec_1",
      "secretVersionId": "ver_1",
      "userWrap": {
        "recipientKeyId": "dev_existing",
        "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
        "encapsulatedKey": "base64...",
        "wrappedKey": "base64..."
      }
    }
  ]
}
```

The existing device unwraps each CEK locally and wraps it to the pending device
public key.

##### Approve new device

```http
POST /v1/users/me/devices/{deviceId}/approve
```

Request:

```json
{
  "approvedByDeviceId": "dev_existing",
  "recipientWraps": [
    {
      "secretVersionId": "ver_1",
      "recipientType": "user",
      "recipientId": "usr_alice",
      "recipientKeyId": "dev_123",
      "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
      "encapsulatedKey": "base64...",
      "wrappedKey": "base64..."
    }
  ]
}
```

Response:

```json
{
  "id": "dev_123",
  "status": "active",
  "wrapsCreated": 12
}
```

`meta` marks the device active only after validating the approving device is
active for the same user and the uploaded wraps target the pending device.

##### Create encrypted secret

```http
POST /v1/org/{organizationDomain}/secrets
```

For the public organization on the default Meta host, the shortcut route is:

```http
POST /v1/secrets
```

Request:

```json
{
  "name": "ANTHROPIC_API_KEY",
  "type": "api_key",
  "owner": {"type": "user", "id": "usr_alice"},
  "metadata": {"envNames": ["ANTHROPIC_API_KEY"]},
  "version": {
    "encryptionAlgorithm": "chacha20poly1305",
    "encryptionNonce": "base64...",
    "encryptedData": "base64...",
    "recipientWraps": [
      {
        "recipientType": "user",
        "recipientId": "usr_alice",
        "recipientKeyId": "dev_123",
        "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
        "encapsulatedKey": "base64...",
        "wrappedKey": "base64..."
      }
    ]
  }
}
```

`encryptedData` is already encrypted by the client. `meta` stores it as-is.

##### Create agent session

```http
POST /v1/org/{organizationDomain}/projects/{projectId}/agent-sessions
```

For the public organization on the default Meta host, the shortcut route is:

```http
POST /v1/projects/{projectId}/agent-sessions
```

Response includes or references the session public key:

```json
{
  "id": "sess_456",
  "projectId": "proj_123",
  "publicKey": {
    "id": "asp_789",
    "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
    "publicKey": "base64..."
  }
}
```

The agent runtime owns the matching private key. `meta` stores only the public
key.

##### Request a session bundle plan

```http
POST /v1/org/{organizationDomain}/projects/{projectId}/agent-sessions/{sessionId}/secrets/bundle-plan
```

For the public organization on the default Meta host, the shortcut route is:

```http
POST /v1/projects/{projectId}/agent-sessions/{sessionId}/secrets/bundle-plan
```

Request:

```json
{
  "scopes": ["llm", "hooks", "services", "tools"],
  "userDeviceIds": ["dev_123"]
}
```

Response:

```json
{
  "agentSessionId": "sess_456",
  "agentSessionPublicKey": {
    "id": "asp_789",
    "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
    "publicKey": "base64..."
  },
  "items": [
    {
      "secretId": "sec_1",
      "secretVersionId": "ver_1",
      "name": "ANTHROPIC_API_KEY",
      "type": "api_key",
      "source": "session",
      "scopes": ["llm"],
      "state": "needs_rewrap",
      "encryptedData": {
        "algorithm": "chacha20poly1305",
        "nonce": "base64...",
        "ciphertext": "base64..."
      },
      "userWrap": {
        "recipientKeyId": "dev_123",
        "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
        "encapsulatedKey": "base64...",
        "wrappedKey": "base64..."
      }
    }
  ]
}
```

The client uses each `userWrap` and the current device private key to recover
the CEK locally, then wraps that same CEK to `agentSessionPublicKey`.

##### Upload agent-session wraps

```http
POST /v1/org/{organizationDomain}/projects/{projectId}/agent-sessions/{sessionId}/secrets/recipient-wraps
```

For the public organization on the default Meta host, the shortcut route is:

```http
POST /v1/projects/{projectId}/agent-sessions/{sessionId}/secrets/recipient-wraps
```

Request:

```json
{
  "wraps": [
    {
      "secretVersionId": "ver_1",
      "recipientType": "agent_session",
      "recipientId": "sess_456",
      "recipientKeyId": "asp_789",
      "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
      "encapsulatedKey": "base64...",
      "wrappedKey": "base64..."
    }
  ]
}
```

Response:

```json
{
  "created": 6,
  "skipped": 0,
  "items": [
    {"secretVersionId": "ver_1", "state": "ready"}
  ]
}
```

The endpoint is idempotent. Re-uploading the same wrap should not create
duplicate effective access.

##### Fetch session secrets inside the agent runtime

```http
GET /v1/org/{organizationDomain}/projects/{projectId}/agent-sessions/{sessionId}/secrets?scope=tools
```

For the public organization on the default Meta host, the shortcut route is:

```http
GET /v1/projects/{projectId}/agent-sessions/{sessionId}/secrets?scope=tools
```

Response:

```json
{
  "items": [
    {
      "secretId": "sec_2",
      "secretVersionId": "ver_2",
      "name": "GITHUB_TOKEN",
      "type": "token",
      "scopes": ["tools"],
      "encryptedData": {
        "algorithm": "chacha20poly1305",
        "nonce": "base64...",
        "ciphertext": "base64..."
      },
      "recipientWrap": {
        "recipientKeyId": "asp_789",
        "algorithm": "hpke-x25519-hkdf-sha256-chacha20poly1305",
        "encapsulatedKey": "base64...",
        "wrappedKey": "base64..."
      }
    }
  ]
}
```

The agent runtime unwraps the CEK with its private key and decrypts locally.

### Project bindings

A project binding means every agent session in the project should receive the
secret for the binding's scopes.

To satisfy that without `meta` seeing plaintext, each effective agent session
needs its own recipient wrap. There are two implementation options:

1. **Eager wrapping**
   - When the project binding is created, create wraps for all existing agent
     sessions in the project.
   - When a new agent session is created, an authorized client or external KMS
     creates a wrap for the new session.

2. **On-demand wrapping**
   - The binding exists at the project level.
   - When a session first needs the secret, `meta` detects that no session wrap
     exists and requests client/KMS-assisted rewrap.
   - The secret is unavailable to that session until the wrap exists.

The database can store the project binding separately from per-session recipient
wraps. The binding says the session is allowed to receive the secret; the wrap is
what makes decryption cryptographically possible.

### What `meta` may store

`meta` may store:

- encrypted payload bytes
- nonce/algorithm/version metadata
- public keys
- recipient key IDs
- wrapped CEKs
- non-sensitive secret metadata, such as environment variable names
- audit records

`meta` must not store:

- plaintext secret values
- plaintext CEKs
- user private keys
- agent-session private keys
- decrypted OAuth refresh tokens
- decrypted environment variable values

### Meta responsibilities

`meta` is responsible for:

- authenticating and authorizing callers
- storing public keys and key status
- storing encrypted secret payloads exactly as submitted
- storing recipient wraps exactly as submitted
- resolving effective secret bindings and scopes
- returning bundle plans and encrypted envelopes to authorized callers
- recording audit events

`meta` is not responsible for:

- generating CEKs
- encrypting secret payloads
- decrypting secret payloads
- wrapping CEKs to recipients
- unwrapping CEKs
- rewrapping CEKs from one recipient to another
- validating secret payload plaintext

This keeps cryptographic operations in clients, agent runtimes, or external
KMS/HSM systems that are explicitly designed to handle key material.

### KMS use

A KMS can be part of the design, but only if it preserves the zero-plaintext
property for the `meta` service runner. KMS integration should be initiated by a
client, agent runtime, or narrowly scoped control-plane worker that returns only
ciphertext/wrap artifacts to `meta`.

Acceptable KMS patterns:

- client-side encryption using public keys exported or managed by KMS
- KMS/HSM rewrap where plaintext key material never returns to `meta`
- KMS-backed user/group/project management keys where the KMS enforces policy

Unacceptable KMS pattern:

- `meta` calls KMS decrypt, receives plaintext key material, and then re-encrypts
  it in process
- `meta` calls a library to unwrap a CEK or decrypt a secret payload in process

That pattern would put secret material or CEKs in `meta` memory, which violates
the primary security goal.

## Versioning and Rotation

Secrets are versioned so callers can rotate values without changing the stable
secret ID.

Rules:

- each secret has monotonically increasing versions
- only one version is current at a time
- older versions can be disabled or deleted
- bindings point to the stable secret, not a specific version by default
- resolving a binding returns the current active version

Future work may allow a binding to pin a specific version, but that is not a
first-pass requirement.

## Read vs Use

The system should distinguish between using a secret and reading raw secret
material.

For now, bindings primarily grant **use** in one or more scopes. A service that
resolves secrets for a scope may receive enough material to perform that scoped
operation, but the user-facing API should avoid exposing raw values unless we add
an explicit read workflow.

Examples:

- `llm` scope may inject a provider key into model-provider configuration.
- `hooks` scope may inject an environment variable into hook execution.
- `services` scope may inject variables into managed background services.
- `tools` scope may expose values to agent tool subprocesses.

Whether any scope can receive raw secret material should be decided per scope and
enforced by the service that requests it.

## Audit Requirements

Audit at least these events:

- secret created
- secret metadata updated
- secret rotated
- secret disabled/deleted
- binding added
- binding removed
- recipient wrap added
- device registered
- device approved
- device disabled/deleted
- secret resolved for an agent session
- secret read directly, if direct read is ever supported

Audit events should include:

- acting user
- secret ID
- owner type and owner ID
- binding target type and target ID, when relevant
- scopes, when relevant
- project/session context, when relevant
- success/denied/error result

## Database Model Alignment

This design is reflected in the database model as follows:

- `secrets.organization_id` records the organization that owns the secret namespace.
- route-level APIs identify organizations by domain (for example `example.com`)
  or by the special public organization value `public`.

- `secret_owners.owner_type` allows `user`, `group`, and `project`.
- `secret_bindings.target_type` allows `project` and `agent_session`.
- `secret_bindings.scopes` records usage contexts such as `llm`, `hooks`,
  `services`, and `tools`.
- `user_devices` stores user device public keys and approval state.
- `agent_session_public_keys` stores agent-session public keys.
- `secret_recipient_wraps` stores wrapped CEKs for user devices and agent
  sessions.
- `meta` stores ciphertext, public keys, and wraps only; it does not perform
  cryptographic operations itself.

See [database-model.md](./database-model.md) for the current table sketch.
