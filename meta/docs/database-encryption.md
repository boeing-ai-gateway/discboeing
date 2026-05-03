# Database Field Encryption

This document describes how the `meta` service should encrypt sensitive values
that must be recoverable by the service itself, such as OAuth provider secrets
and OIDC/JWKS signing private keys.

It complements [secrets.md](./secrets.md). User, project, and agent runtime
secrets use client-side envelope encryption where `meta` stores ciphertext and
recipient wraps but must not see plaintext. Database field encryption is
separate: it protects operational secret fields stored in the `meta` database
when the `meta` service must decrypt them to perform a trusted server-side
operation.

## Summary

Use application-level field encryption before values are written through GORM.
The database stores only encrypted envelopes plus non-sensitive metadata. The
`meta` process decrypts fields only at the point of use and only with an
enabled encryption provider.

Initial encrypted data classes:

| Data class | Why `meta` must decrypt it |
| --- | --- |
| OAuth provider client secrets | Exchange authorization codes and refresh provider metadata during login flows |
| OAuth token refresh material owned by Meta | Refresh server-managed upstream access tokens when needed |
| OIDC/JWT signing private keys | Sign ID tokens, access tokens, and refresh-token JWTs if refresh tokens are JWT-backed |
| Cloud-provider integration secrets | Store AWS, Google, and Azure control-plane credentials that bootstrap provider-specific integrations |

Initial key providers:

| Provider | Production use |
| --- | --- |
| `aws-kms` | AWS KMS symmetric keys |
| `gcp-kms` | Google Cloud KMS CryptoKeys |
| `azure-key-vault` | Azure Key Vault keys |
| `local` | Development and single-node test deployments only |

The `local` provider is not a production default. Production deployments should
use a cloud KMS provider or an equivalent external key-management boundary.

## Goals

- Keep plaintext operational secrets out of database rows, backups, query logs,
  and ad-hoc database exports.
- Support AWS, Google Cloud, and Azure KMS integrations out of the box.
- Keep the encrypted envelope format provider-neutral so rows can survive key
  provider changes and staged migrations.
- Support key rotation without rewriting every row immediately.
- Bind ciphertext to its table, column, and row context with authenticated
  additional data.
- Make encrypted fields hard to misuse by giving them a typed storage shape and
  a small service-level API.

## Non-goals

- This is not transparent database engine encryption such as PostgreSQL TDE.
  Disk-level or database-level encryption can still be enabled, but it does not
  replace field encryption.
- This is not a replacement for the client-side secret encryption model in
  [secrets.md](./secrets.md).
- This does not make plaintext unavailable to the `meta` process. If `meta`
  needs to sign a JWT or call an OAuth provider with a client secret, that value
  exists briefly in process memory.
- This does not provide queryable encryption for arbitrary secret values.
  Sensitive fields should not be filtered or sorted by plaintext.

## Threat Model

Field encryption protects against:

- database-only compromise
- leaked database backups or snapshots
- accidental secret exposure through SQL debugging, exports, or support tooling
- read-only database replicas used for analytics or diagnostics

Field encryption does not protect against:

- arbitrary code execution inside the `meta` process
- an attacker with both database access and KMS decrypt permission
- logging plaintext after decryption
- compromised cloud credentials that can call the configured KMS key

The design assumes the `meta` service is trusted to handle operational
plaintext at the point of use, but storage and database operators are not trusted
with plaintext secret values.

## Encryption Model

Use envelope encryption per field value:

1. Generate a random data encryption key (DEK).
2. Encrypt the plaintext field with the DEK using an AEAD cipher.
3. Wrap the DEK with the configured key encryption key (KEK), usually through a
   cloud KMS provider.
4. Store the encrypted field envelope in the database.
5. On read, unwrap the DEK through the provider, decrypt the ciphertext, use the
   plaintext, then drop references as soon as possible.

```text
plaintext field
  -> encrypt with random DEK using AEAD
  -> ciphertext stored in database envelope

DEK
  -> wrapped by AWS KMS / Google Cloud KMS / Azure Key Vault / local KEK
  -> encrypted_dek stored in database envelope
```

### Algorithms

Recommended baseline:

- field encryption: AES-256-GCM
- DEK size: 256 bits generated from `crypto/rand`
- nonce size: 96 bits generated randomly per encryption
- JWT signing: ES256 for the initial implementation

ES256 maps cleanly to Google Cloud KMS `EC_SIGN_P256_SHA256`, which Google marks
as its recommended elliptic-curve signing algorithm. RFC 8725 does not rank
ES256, EdDSA, and RS256 globally; it requires applications to explicitly allow
only cryptographically current algorithms, bind each key to exactly one
algorithm, and verify that the JWT `alg` header matches the operation. EdDSA is a
strong modern choice where supported, but Google Cloud KMS asymmetric signing does
not provide an Ed25519 JWT signing equivalent today, so ES256 is the best fit for
GCP-backed signing. RS256 remains widely interoperable but uses larger keys and
signatures and is not necessary for Meta's first-party ecosystem.

For GCP KMS-backed JWT signing, Meta should only register KMS key versions with
`EC_SIGN_P256_SHA256` for `ES256` rows.
- KMS wrapping: provider-native symmetric encrypt/decrypt or wrap/unwrap
- envelope encoding: JSON for readability during early implementation, stored in
  a `json`/`jsonb` or text column depending on database support

The envelope must include algorithm identifiers so future migrations can support
new ciphers without guessing.

### Authenticated Additional Data

Each encrypted field should include authenticated additional data (AAD) derived
from non-sensitive storage context:

```text
service=meta
table=<table_name>
column=<column_name>
row_id=<stable row id>
purpose=<data class>
```

AAD prevents ciphertext copied from one field or row from decrypting
successfully in another context. For rows created before an ID is assigned,
create the ID in application code before encrypting fields.

## Envelope Shape

Store encrypted values in a common envelope shape:

```json
{
  "version": 1,
  "provider": "aws-kms",
  "key_id": "arn:aws:kms:us-east-1:123456789012:key/abcd...",
  "algorithm": "AES-256-GCM",
  "wrap_algorithm": "AWS-KMS-SYMMETRIC",
  "nonce": "base64url...",
  "ciphertext": "base64url...",
  "encrypted_dek": "base64url...",
  "aad": {
    "service": "meta",
    "table": "oauth_providers",
    "column": "client_secret_encrypted",
    "row_id": "oprv_01...",
    "purpose": "oauth_client_secret"
  },
  "created_at": "2026-05-01T00:00:00Z"
}
```

The envelope may include provider-specific metadata under a `provider_data`
object when needed, but common fields should remain provider-neutral.

## Data Model

Use typed encrypted columns rather than generic string columns whose semantics
are unclear. Suggested column names:

| Table | Column | Purpose |
| --- | --- | --- |
| `oauth_providers` or expanded `oauth_applications` | `client_secret_encrypted` | Upstream OAuth/OIDC provider client secret |
| `oauth_provider_tokens` | `refresh_token_encrypted` | Server-managed upstream refresh token, if stored |
| `jwt_signing_keys` | `private_key_encrypted` | OIDC/JWT signing private key material |
| `cloud_provider_configs` | `credential_encrypted` | Provider-specific bootstrap credential or service principal secret |

Bootstrap tokens are intentionally excluded from this table. They are generated
with 260 bits of cryptographic randomness, printed once per setup-mode startup,
and stored as a SHA-256 lookup hash because Meta never needs to recover the raw
token. A password-based hash is unnecessary here because the token is not a
low-entropy user-chosen password. The printed token uses lowercase Crockford
base32 characters, so it contains no extra dashes or underscores beyond the
`mboot_` prefix separator.

Each encrypted column should be nullable only when the credential type does not
require a secret. For example, a public OAuth client or PKCE-only client may not
have a client secret.

### JWKS Signing Keys

JWT signing keys are operational signing keys owned by Meta. Meta should own the
full lifecycle for these keys: creation, activation, rotation, retirement,
invalidation, and deletion of private material. Operators may trigger lifecycle
operations through Meta APIs or admin commands, but they should not mutate
signing keys directly in the database or rotate backing KMS keys behind Meta's
back.

Public JWKS material is not sensitive and should be served from plaintext
metadata:

| Field | Sensitivity | Storage |
| --- | --- | --- |
| `kid` | Non-sensitive | Plaintext, indexed |
| algorithm | Non-sensitive | Plaintext |
| signer backend | Non-sensitive | Plaintext |
| backend key ID | Usually non-sensitive but operational | Plaintext, admin-only API exposure |
| public JWK | Non-sensitive | Plaintext JSON for JWKS endpoint |
| private JWK/PEM | Sensitive | `private_key_encrypted` for local DB-backed keys only |
| status | Non-sensitive | Plaintext, indexed |
| activation/retirement timestamps | Non-sensitive | Plaintext |

The JWKS endpoint must never need private key decryption or external KMS signing
permissions. It should serve public keys from plaintext `public_jwk_json` rows.
Only active signing operations should use private key material or external
signing APIs.

Recommended signing key table:

```text
jwt_signing_keys
  id
  organization_id nullable       // null for global Meta issuer keys
  kid unique
  algorithm
  backend                        // db-local, aws-kms, gcp-kms, azure-key-vault
  backend_key_id nullable         // KMS key/version/resource ID for external backends
  public_jwk_json
  private_key_encrypted nullable  // required only for db-local
  status                         // next, active, retired, disabled
  not_before
  not_after
  created_at
  updated_at
```

Backend rules:

- `db-local` means Meta generated the private key, encrypted it with database
  field encryption, and stored it in `private_key_encrypted`.
- `aws-kms`, `gcp-kms`, and `azure-key-vault` mean Meta created and controls an
  asymmetric signing key or key version in the provider and stores only the
  provider resource ID plus public JWK metadata locally.
- External signer backends should not be manually rotated outside Meta. Meta owns
  the rotation workflow and calls the provider APIs needed to create, activate,
  retire, or disable keys.
- If provider policy prevents Meta from creating keys directly, the deployment is
  not using the fully managed rotation mode and must still register changes
  through Meta before they are used. Direct out-of-band replacement of signing
  keys is unsupported because it can break `kid`, JWKS cache, and token lifetime
  guarantees.

For a single global issuer, `organization_id` can remain null. If Meta later
supports organization-specific issuers, the same model can scope signing keys by
organization.

#### Rotation and JWKS overlap

Meta should rotate JWT signing keys automatically. The initial default policy is:

- rotation interval: 72 hours
- prepublish window: 24 hours
- verification overlap: 7 days

A three-day signing cryptoperiod is intentionally conservative but not crazy for
an automated service-owned key system. Daily rotation is also reasonable once the
provider and store automation is reliable. Weekly rotation reduces operational
churn and is still a common practical default. The important requirement is not
the exact interval; it is that rotation is automatic, observable, and preserves a
verification overlap longer than the maximum token lifetime plus expected JWKS
cache TTL.

Lifecycle behavior:

1. A `next` key is created before it is used for signing.
2. `next` keys are published in JWKS during the prepublish window so verifiers can
   cache them before tokens reference their `kid`.
3. At rotation time, the `next` key becomes `active` and the old `active` key
   becomes `retired`.
4. New tokens are signed only with the `active` key.
5. JWKS publishes `next`, `active`, and non-expired `retired` keys.
6. `retired` keys are removed from JWKS only after `not_after`, which should be
   at least the maximum token lifetime plus JWKS cache TTL.
7. `disabled` keys are never used for signing and are not published in JWKS.

The code-level rotation defaults are configurable with:

```text
META_JWT_SIGNING_BACKEND=gcp-kms
META_JWT_SIGNING_KEY_ID=projects/<project>/locations/<location>/keyRings/<ring>/cryptoKeys/<signing-key>
META_JWT_SIGNING_ROTATION_INTERVAL=72h
META_JWT_SIGNING_PREPUBLISH_WINDOW=24h
META_JWT_SIGNING_VERIFICATION_OVERLAP=168h
```

#### Signing key store abstraction

Token issuing code should depend on a signing-key store and signer interface, not
on direct access to decrypted private key material. The first implementation can
be `db-local`, but the interface should leave room for cloud KMS-backed signing
where the private key never enters Meta process memory.

Conceptual interfaces:

```go
type SigningKeyStore interface {
    ActiveSigningKey(ctx context.Context, issuer string) (SigningKey, error)
    PublicJWKS(ctx context.Context, issuer string) (JWKS, error)
    CreateNextKey(ctx context.Context, issuer string, alg string) (SigningKey, error)
    PromoteKey(ctx context.Context, keyID string) error
    RetireKey(ctx context.Context, keyID string) error
    DisableKey(ctx context.Context, keyID string) error
}

type JWTSigner interface {
    Sign(ctx context.Context, key SigningKey, signingInput []byte) ([]byte, error)
}

type SigningKey struct {
    ID         string
    Issuer     string
    KID        string
    Algorithm  string
    Backend    string
    BackendKey string
    Status     string
    PublicJWK  json.RawMessage
}
```

`SigningKeyStore` owns lifecycle and metadata. `JWTSigner` owns the act of
signing for one backend:

- `db-local` signer decrypts `private_key_encrypted` with the database encryption
  provider and signs in process.
- `aws-kms` signer calls AWS KMS `Sign` using the Meta-owned key ARN/version.
- `gcp-kms` signer calls Cloud KMS asymmetric signing using the Meta-owned key
  version. For ES256, the KMS key version must use `EC_SIGN_P256_SHA256`; Meta
  sends the SHA-256 digest of the JWT signing input to `AsymmetricSign`, converts
  the returned DER ECDSA signature to JWT's raw `R || S` encoding, and stores the
  KMS key-version resource name in `backend_key_id`. The public key is read from
  Cloud KMS with `GetPublicKey` and published through the normal JWKS metadata.
  Deployments configure the parent asymmetric signing CryptoKey with
  `META_JWT_SIGNING_KEY_ID`; Meta creates new CryptoKeyVersions under that key
  during bootstrap and rotation.
- `azure-key-vault` signer calls Azure Key Vault signing using the Meta-owned key
  version.

All backends return the same JWT shape and publish public verification keys
through the same JWKS endpoint. The difference is only where private signing
happens.

The implementation should start with `db-local` because it is simple and works in
all deployments. The API should still be backend-aware from the beginning so
external signing can be added without rewriting token issuing code.

For fully managed GCP signing, an administrator creates the parent Cloud KMS
CryptoKey once with purpose `ASYMMETRIC_SIGN` and version template algorithm
`EC_SIGN_P256_SHA256`. Meta then owns CryptoKeyVersion creation and records each
created version as a `jwt_signing_keys` row. The runtime identity needs Cloud KMS
permissions to create key versions, get public keys, and sign with active
versions.

### OAuth Provider Secrets

OAuth login provider configuration needs a clear split between public metadata
and encrypted secret material:

```text
oauth_providers
  id
  organization_id
  provider_id                    // google, github, microsoft, okta, ...
  provider_type                  // oidc, google, microsoft, github, ...
  display_name
  issuer_url
  client_id                      // not secret
  client_secret_encrypted        // secret
  scopes
  status
  created_at
  updated_at
```

For Microsoft Entra ID / Azure, tenant IDs, issuer URLs, authorization URLs, and
client IDs are not secret. Client secrets and certificate private keys are
secret. If a provider uses private-key JWT client authentication, store the
private key as encrypted field material and expose only its public certificate or
thumbprint as plaintext metadata.

## Provider Configuration

Configure exactly one active database encryption provider per `meta` deployment
for new writes. Old rows keep their provider and key metadata and remain
readable as long as the provider remains configured for decrypt.

Suggested environment configuration:

```text
META_DB_ENCRYPTION_PROVIDER=aws-kms
META_DB_ENCRYPTION_KEY_ID=arn:aws:kms:us-east-1:123456789012:key/abcd...
META_DB_ENCRYPTION_REQUIRED=true
```

Provider-specific configuration:

### AWS KMS

```text
META_DB_ENCRYPTION_PROVIDER=aws-kms
META_DB_ENCRYPTION_KEY_ID=arn:aws:kms:us-east-1:123456789012:key/abcd...
META_AWS_REGION=us-east-1
```

Use the default AWS credential chain for production: IAM role, web identity, or
workload identity. Avoid static AWS access keys when running in AWS.

The KMS key policy should allow:

- `kms:Encrypt`
- `kms:Decrypt`
- `kms:DescribeKey`

Only the `meta` runtime identity and tightly controlled break-glass operators
should have decrypt permission.

### Google Cloud KMS

```text
META_DB_ENCRYPTION_PROVIDER=gcp-kms
META_DB_ENCRYPTION_KEY_ID=projects/<project>/locations/<location>/keyRings/<ring>/cryptoKeys/<key>
```

The implementation uses the Google Cloud KMS Go SDK and the default Google
credential chain used by Application Default Credentials or Workload Identity.
`META_DB_ENCRYPTION_KEY_ID` must be the full symmetric CryptoKey resource name.
The runtime identity needs Cloud KMS CryptoKey Encrypter/Decrypter on the
configured key.

Meta encrypts field plaintext with a random local AES-256-GCM DEK, then wraps
that DEK with Google Cloud KMS. The same canonical field AAD is sent to Cloud KMS
as `additional_authenticated_data`, so both the local payload ciphertext and the
wrapped DEK are bound to the table, column, row ID, and purpose.

### Azure Key Vault

```text
META_DB_ENCRYPTION_PROVIDER=azure-key-vault
META_DB_ENCRYPTION_KEY_ID=https://<vault>.vault.azure.net/keys/<key-name>/<version>
```

Use Managed Identity or workload identity. The runtime identity needs key
encrypt/decrypt or wrap/unwrap permission for the configured key, depending on
the Azure SDK operation we choose.

### Local Provider

```text
META_DB_ENCRYPTION_PROVIDER=local
META_DB_ENCRYPTION_KEY_FILE=/var/lib/meta/db-encryption.key
```

The local provider loads a 256-bit KEK from a file or environment secret. It is
acceptable for tests, local development, and ephemeral demos. It should fail in
production unless explicitly allowed by a development-mode flag.

## Startup Behavior

At startup, `meta` should validate encryption configuration before serving
requests that need encrypted fields.

Recommended behavior:

- If `META_DB_ENCRYPTION_REQUIRED=true`, fail startup when no provider is
  configured or the provider cannot decrypt a health-check test envelope.
- If encrypted rows exist but no matching provider is configured, fail startup.
- If only plaintext legacy rows exist and encryption is required, allow startup
  only in migration mode or fail with a clear remediation message.
- Expose health status that reports provider availability without exposing key
  IDs to unauthenticated callers.

## Read and Write API

Keep encryption behind a small package, for example
`meta/internal/crypto/dbcrypt`. The package should make AAD mandatory and should
not expose lower-level encrypt/decrypt functions that accept arbitrary byte
slices without field context.

The internal API should force callers to identify the exact storage location and
purpose of the encrypted value:

```go
type Encryptor interface {
    Encrypt(ctx context.Context, ref FieldRef, plaintext []byte) (Envelope, error)
    Decrypt(ctx context.Context, ref FieldRef, envelope Envelope) ([]byte, error)
}

type FieldRef struct {
    Table   string
    Column  string
    RowID   string
    Purpose string
}
```

`FieldRef` is the source of authenticated additional data. `Encrypt` derives AAD
from the provided `FieldRef`, stores that AAD context in the envelope metadata,
and passes the canonical AAD bytes into the AEAD operation. `Decrypt` derives the
same canonical AAD bytes from its `FieldRef` argument and uses them for AEAD
verification. If the caller provides a different table, column, row ID, or
purpose than the original encryption context, decryption must fail with an
authentication error and return no plaintext.

The implementation should treat AAD mismatch exactly like corrupted ciphertext:

- return a typed error such as `ErrAuthenticationFailed`
- do not fall back to decrypting without AAD
- do not try alternate row IDs, columns, or purposes
- do not return partial plaintext
- audit the failed decrypt attempt without logging the envelope or plaintext

Canonical AAD should be deterministic and versioned. A simple initial shape is a
stable JSON object with sorted keys or a length-prefixed binary encoding:

```json
{
  "version": 1,
  "service": "meta",
  "table": "oauth_providers",
  "column": "client_secret_encrypted",
  "row_id": "oprv_01...",
  "purpose": "oauth_client_secret"
}
```

Do not rely on the envelope's stored AAD to authorize decryption. The stored AAD
is useful for diagnostics and migration planning, but the decrypt path must use
the caller-provided `FieldRef` as the expected context. The stored AAD should be
compared to the caller-derived context before decrypting when possible, and the
AEAD check remains the final authority.

To avoid accidental context drift, define row-aware helpers near the model or
store layer for each encrypted field. Callers should pass the row object and a
field identifier, and the helper should derive the table, encrypted column, row
ID, and purpose in one place:

```go
type EncryptedField string

const (
    OAuthProviderClientSecret EncryptedField = "oauth_provider.client_secret"
    JWTSigningPrivateKey      EncryptedField = "jwt_signing_key.private_key"
)

func FieldRefFor(row any, field EncryptedField) (dbcrypt.FieldRef, error) {
    switch r := row.(type) {
    case *model.OAuthProvider:
        if field != OAuthProviderClientSecret {
            return dbcrypt.FieldRef{}, ErrInvalidEncryptedField
        }
        return dbcrypt.FieldRef{
            Table:   r.TableName(),
            Column:  "client_secret_encrypted",
            RowID:   r.ID,
            Purpose: "oauth_client_secret",
        }, nil
    case *model.JWTSigningKey:
        if field != JWTSigningPrivateKey {
            return dbcrypt.FieldRef{}, ErrInvalidEncryptedField
        }
        return dbcrypt.FieldRef{
            Table:   r.TableName(),
            Column:  "private_key_encrypted",
            RowID:   r.ID,
            Purpose: "jwt_signing_private_key",
        }, nil
    default:
        return dbcrypt.FieldRef{}, ErrUnsupportedEncryptedRow
    }
}
```

That gives service and store code a simple, consistent call shape:

```go
ref, err := encryptedfields.FieldRefFor(provider, encryptedfields.OAuthProviderClientSecret)
if err != nil {
    return err
}

envelope, err := encryptor.Encrypt(ctx, ref, []byte(clientSecret))
```

This helper should be the preferred path for normal application code. It keeps
all field-ref naming decisions in one registry and prevents each feature from
inventing its own table, column, row ID, or purpose strings. Direct `FieldRef`
literals should be limited to tests for the encryption package itself and rare
migration code where no model row exists yet.

The helper must validate that:

- the row type supports encrypted fields
- the requested encrypted field belongs to that row type
- the row ID is non-empty before encryption or decryption
- the encrypted column and purpose come from the central registry, not caller
  input

For create flows, generate the row ID before encrypting fields. That lets the
same row-aware helper produce stable AAD for both initial insert and later read,
update, rotation, and migration paths.

Store and service code should pass plaintext only through explicit create,
update, or use paths. API response models must omit encrypted envelopes unless
an admin/debug route explicitly needs envelope metadata.

Avoid GORM hooks for encryption unless the hook can reliably receive the field
context and KMS dependencies. Prefer service/store code that is explicit about
which fields are encrypted and which `FieldRef` helper is used.

## Blind Indexes

Do not index encrypted plaintext. If the service needs uniqueness or lookup by a
sensitive value, store a blind index:

```text
blind_index = HMAC-SHA-256(index_key, canonical_plaintext)
```

Initial OAuth and JWKS use cases should avoid blind indexes:

- OAuth provider lookup uses organization ID plus provider ID, not client secret.
- JWKS lookup uses `kid`, not private key material.
- Refresh token lookup should use a token ID, session ID, or opaque handle, not
  the refresh token value.

If blind indexes are added later, the HMAC index key should be managed like a KMS
protected secret and rotated separately from field-encryption KEKs.

## Key Rotation

Support two rotation layers:

1. **KEK rotation:** change the cloud KMS key or key version used to wrap new
   DEKs.
2. **Secret rotation:** change the actual OAuth client secret, provider token, or
   JWT signing key.

KEK rotation should be lazy by default:

- new writes use the active configured key
- reads can decrypt old envelopes using their stored provider/key metadata
- when an old envelope is updated, re-encrypt it with the active key
- an optional background job can rewrap old envelopes without changing plaintext
  values

JWT signing key rotation is separate and must follow token validation rules.
Meta owns this workflow for both `db-local` and external signer backends:

1. Create a new signing key in `next` state.
   - For `db-local`, Meta generates the keypair, encrypts the private key with
     database field encryption, and stores the public JWK.
   - For external signer backends, Meta calls the provider API to create the
     asymmetric key or key version, then stores the provider key ID and public
     JWK metadata.
2. Publish the new public JWK before using the key to sign tokens.
3. Promote the new key to `active` for new tokens.
4. Keep the old public key in JWKS until all tokens it signed have expired.
5. Mark the old key `retired`.
6. After the retention window, disable or delete private signing material.
   - For `db-local`, clear or disable the encrypted private key.
   - For external signer backends, call the provider API to disable or schedule
     deletion according to provider safety rules.

Meta may expose admin operations to force rotation, retire a key early, disable a
compromised key, or rebuild JWKS metadata. Those operations should still execute
through Meta's signing-key store so audit, status transitions, `kid` selection,
and JWKS cache behavior stay consistent. Direct database edits or out-of-band KMS
rotation are unsupported.

## Audit and Logging

Audit security-sensitive encrypted-field operations without logging plaintext or
ciphertext payloads.

Audit events should include:

- actor user/service ID
- organization ID when applicable
- target resource ID
- field purpose, not raw column value
- action: create, update, decrypt/use, sign, rotate, force-rotate, retire,
  disable
- result: success, denied, error
- KMS provider and key ID only in admin-only audit views if needed

Application logs must not include:

- plaintext secrets
- decrypted signing keys
- raw encrypted envelopes
- OAuth authorization codes or refresh tokens
- KMS plaintext data keys

## Migration Plan

1. Add the encryption package and provider configuration.
2. Add encrypted columns alongside any plaintext legacy columns.
3. For new writes, require encrypted columns.
4. Add a one-shot migration job for existing plaintext operational secrets:
   - read plaintext from legacy column
   - write encrypted envelope to the new column
   - verify decrypt succeeds
   - clear the plaintext column
5. Once all deployments have migrated, drop legacy plaintext columns.
6. Enable `META_DB_ENCRYPTION_REQUIRED=true` in production.

For early `meta` development, avoid creating plaintext columns for new
operational secrets. Start with encrypted columns so no data migration is needed
later.

## Relationship to Cloud Providers

The out-of-box provider goal is about key management, not vendor-specific schema
forks. AWS, Google Cloud, and Azure should use the same Meta service APIs and
metadata model.

There are two provider integration layers:

1. **Database field encryption provider:** wraps and unwraps DEKs for encrypted
   database fields.
2. **JWT signer backend:** optionally creates, rotates, and uses external
   asymmetric signing keys without storing private key material in the Meta
   database.

The first implementation should support `db-local` JWT signing and cloud-backed
field encryption. Cloud KMS-backed JWT signing can be added behind the
signing-key store abstraction later, with Meta still owning the rotation workflow
through provider APIs.

Provider integration examples:

| Cloud | Field encryption provider | Optional JWT signer backend | Typical encrypted Meta fields |
| --- | --- | --- | --- |
| AWS | AWS KMS symmetric key | AWS KMS asymmetric signing key | IAM Identity Center/OIDC client secrets, AWS integration bootstrap secrets, local Meta JWT signing keys |
| Google | Cloud KMS CryptoKey | Cloud KMS asymmetric signing key version | Google OAuth client secrets, Google Workspace/Admin integration secrets, local Meta JWT signing keys |
| Azure | Azure Key Vault key | Azure Key Vault signing key version | Microsoft Entra ID client secrets or private-key auth material, Azure integration bootstrap secrets, local Meta JWT signing keys |

This keeps Discobot deployments portable: encrypted database fields can be read
by any deployment that has the matching field-encryption provider configuration
and decrypt permission for the envelope's stored key ID. JWT signing remains
portable at the API level because token issuing code depends on Meta's signer
abstraction rather than directly loading private keys.

## Open Questions

- Should Meta use a single global signing-key set initially, or should each
  organization eventually have organization-scoped issuer keys?
- Should private-key JWT OAuth client authentication be part of the initial OAuth
  provider schema, or added after client-secret auth works?
- Do we need a production-supported non-cloud provider such as HashiCorp Vault?
- Should the first field-encryption implementation use provider-native KMS
  encrypt/decrypt for DEK wrapping, or provider-native asymmetric wrap/unwrap
  where available?
