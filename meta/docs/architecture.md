# Meta Service Architecture

Meta follows a layered request architecture for operations with application
logic:

```text
router -> handler -> service -> DAL/store
```

- **Router** matches generated OpenAPI routes, attaches route metadata, and runs
  cross-cutting middleware such as authentication and authorization.
- **Handler** owns HTTP concerns: path parameters, request decoding, response
  encoding, and HTTP status/error mapping. Handlers should not accumulate
  business rules, encryption workflows, or multi-step persistence orchestration.
- **Service** owns application behavior: validation, resource scoping,
  cross-record workflows, encryption/decryption, policy-neutral business rules,
  and calls into the DAL. Add a service when an operation does more than a simple
  read/write or needs reusable domain behavior.
- **DAL/store** owns database access through GORM. Store methods should express
  persistence operations and hide query details, but avoid embedding application
  workflows that belong in services.

Layers can be dropped when an operation is appropriately simple. For example,
`whoami` only echoes request identity from context, so it can remain a handler
without a service. OAuth application CRUD uses `OAuthApplicationService` because
it validates provider-specific input, scopes operations to organizations,
encrypts client secrets, and coordinates store calls.
