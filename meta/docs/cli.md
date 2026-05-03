# Meta CLI

`metactl` provides convenience commands for the Meta API in addition to the
`raw` request command.

## Output formats

Typed commands use table output by default and accept `-o table`, `-o json`, or
`-o yaml`:

```bash
metactl oauth --org example.com list
metactl oauth --org example.com -o json list
metactl oauth --org example.com -o yaml get oauth_app_123
```

Table output is registered per resource type, so new typed commands can add a
printer by mapping an API object to one table row.

## OAuth applications

OAuth applications can be inspected imperatively:

```bash
metactl oauth --org example.com list
metactl oauth --org example.com get oauth_app_123
metactl oauth --org example.com delete oauth_app_123
```

They can also be managed declaratively with an apply file:

```bash
metactl apply -f oauth.yaml
```

Example resource:

```yaml
type: OAuthApplication
name: github
organization: example.com
provider: github
clientId: github-client-id
clientSecret: github-client-secret
redirectUris:
  - https://meta.example.com/oauth/github/callback
grantTypes:
  - authorization_code
responseTypes:
  - code
scopes:
  - read:user
  - user:email
tokenEndpointAuthMethod: client_secret_basic
status: active
```

`type`, `name`, and `organization` are common top-level fields. Resource-specific
fields also live at the top level; avoid adding resource fields that conflict
with the common names.

`metactl apply` creates the OAuth application when no match exists. It updates
an existing application when it finds a single application with the same `name`
and `provider`. If matching by name/provider is ambiguous, set `id` to the OAuth
application ID.

`clientSecret` is only sent when present in the file, so applying a file without
`clientSecret` preserves the existing encrypted secret on updates.
