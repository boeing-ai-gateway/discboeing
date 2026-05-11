# Server Architecture

This document describes the architecture of the Discobot Go server, which provides REST APIs and manages workspace/session/sandbox lifecycle.

## Overview

The server follows a layered architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Layer                                │
│  Middleware → Router → Handlers                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Service Layer                               │
│  Business logic, validation, orchestration                      │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌─────────────┐       ┌─────────────┐       ┌─────────────┐
│    Store    │       │   Sandbox   │       │     Git     │
│   (GORM)    │       │   Provider  │       │   Provider  │
└─────────────┘       └─────────────┘       └─────────────┘
        │                     │                     │
        ▼                     ▼                     ▼
   Database              Docker API            File System
```

## Module Documentation

- [Handler Module](./design/handler.md) - HTTP request handlers
- [Service Module](./design/service.md) - Business logic layer
- [Store Module](./design/store.md) - Data access layer
- [Sandbox Module](./design/sandbox.md) - Docker integration
- [Cache System](./design/cache.md) - Project-scoped cache volumes
- [Events Module](./design/events.md) - SSE and event system
- [Jobs Module](./design/jobs.md) - Background job processing

## Directory Structure

```
server/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── config/config.go        # Environment configuration
│   ├── database/database.go    # DB connection
│   ├── model/model.go          # GORM models
│   ├── store/store.go          # Data access layer
│   ├── handler/                # HTTP handlers
│   │   ├── handler.go          # Base handler
│   │   ├── auth.go
│   │   ├── projects.go
│   │   ├── workspaces.go
│   │   ├── sessions.go
│   │   ├── agents.go
│   │   ├── chat.go
│   │   ├── credentials.go
│   │   ├── preferences.go      # User preferences API
│   │   ├── files.go
│   │   ├── terminal.go
│   │   ├── git.go
│   │   ├── events.go
│   │   └── status.go
│   ├── service/                # Business logic
│   │   ├── auth.go
│   │   ├── project.go
│   │   ├── workspace.go
│   │   ├── session.go
│   │   ├── agent.go
│   │   ├── chat.go
│   │   ├── sandbox.go
│   │   ├── sandbox_client.go
│   │   ├── credential.go
│   │   ├── preference.go       # User preferences (key/value store)
│   │   └── git.go
│   ├── sandbox/                # Sandbox abstraction
│   │   ├── runtime.go          # Interface
│   │   ├── docker/provider.go  # Docker impl
│   │   └── mock/provider.go    # Mock impl
│   ├── git/                    # Git provider
│   │   ├── git.go              # Interface
│   │   └── local.go            # Local impl
│   ├── dispatcher/             # Job dispatcher
│   ├── jobs/                   # Background jobs
│   ├── events/                 # Event system
│   ├── middleware/             # HTTP middleware
│   ├── encryption/             # AES-256-GCM
│   └── integration/            # Integration tests
```

## Initialization Flow

The `main()` function initializes all components:

```go
func main() {
    // 1. Load configuration
    cfg := config.Load()

    // 2. Connect database
    db := database.Connect(cfg.DatabaseDSN)
    database.Migrate(db)
    database.Seed(db)

    // 3. Create providers
    gitProvider := git.NewLocalProvider(cfg)
    sandboxProvider := sandbox.NewDockerProvider(cfg)

    // 4. Create store (separate read/write pools for SQLite)
    store := store.New(db.DB, db.ReadDB)

    // 5. Create services
    services := service.NewServices(store, gitProvider, sandboxProvider)

    // 6. Create event system
    eventBroker := events.NewBroker(store)
    eventPoller := events.NewPoller(store, eventBroker)
    go eventPoller.Start()

    // 7. Create job dispatcher
    jobQueue := jobs.NewQueue(store)
    dispatcher := dispatcher.New(cfg, store, jobQueue)
    dispatcher.RegisterExecutor("workspace_init", ...)
    dispatcher.RegisterExecutor("session_init", ...)
    go dispatcher.Start()

    // 8. Create router
    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    r.Use(middleware.Logger)
    r.Use(middleware.CORS)

    // 9. Register handlers
    h := handler.New(cfg, store, services, eventBroker)
    h.RegisterRoutes(r)

    // 10. Start server
    http.ListenAndServe(":"+cfg.Port, r)
}
```

## Request Flow

### Standard API Request

```
Client Request
      │
      ▼
┌─────────────────┐
│   Middleware    │ → Request ID, Logging, Auth
└─────────────────┘
      │
      ▼
┌─────────────────┐
│    Handler      │ → Parse request, validate
└─────────────────┘
      │
      ▼
┌─────────────────┐
│    Service      │ → Business logic
└─────────────────┘
      │
      ▼
┌─────────────────┐
│     Store       │ → Database query
└─────────────────┘
      │
      ▼
     JSON Response
```

### Chat Request (SSE)

```
Client POST /sessions/{sessionId}/threads/{threadId}/chat
      │
      ▼
┌─────────────────┐
│  Chat Handler   │ → Validate session
└─────────────────┘
      │
      ▼
┌─────────────────┐
│  Chat Service   │ → Create/get session
└─────────────────┘
      │
      ▼
┌─────────────────────┐
│  Sandbox Client     │ → POST to sandbox:3002/chat
└─────────────────────┘
      │
      ▼
   SSE Stream ──────────▶ Client
```

## Data Model

### Entity Relationships

```
User
 │
 └──▶ Project
       │
       ├──▶ Workspace ──▶ Session ──▶ Messages
       │
       └──▶ Agent ──▶ MCPServer
```

### Key Models

```go
type Workspace struct {
    ID          string
    ProjectID   string
    Name        string
    Path        string     // Local path or git URL (actual location)
    DisplayName *string    // Optional: custom display name for UI (nil = use path)
    Status      string     // initializing, ready, error
    Sessions    []Session
}

type Session struct {
    ID          string
    WorkspaceID string
    AgentID     string
    Name        string
    Status      string  // initializing, ready, stopped, error, removing, removed
    ThreadStatus string // last known idle, queued, running, needs_attention, unknown
    SandboxID   string
}

type Agent struct {
    ID        string
    ProjectID string
    Name      string
    Type      string  // claude-code, gemini-cli, etc.
    Mode      string
    Model     string
    IsDefault bool
}

type UserPreference struct {
    ID        string
    UserID    string    // Scoped to user, not project
    Key       string    // e.g., "theme", "preferredIDE"
    Value     string    // Stored as text (can be JSON)
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Session Thread Activity Summary

Per-thread activity is authoritative in the sandbox agent and is fetched from the
thread APIs when a session is open. The server persists only one session-level
summary on `sessions.thread_status` so project/session lists can show
`idle`, `queued`, `running`, or `needs_attention` without waking every sandbox or
maintaining a separate activity table.

Active flows update the summary from observations they already make:

- chat start and queued prompt dispatch promote the summary to `running` or
  `queued`;
- stream completion-status chunks trigger a one-shot refresh from an already
  running sandbox when a completion stops;
- thread actions such as cancel, queue edits, answer, and delete refresh the
  summary from the running sandbox;
- thread lists opened by the user can persist their aggregate snapshot, guarded
  so older snapshots do not overwrite newer prompt-start observations.

A background session thread-status syncer closes the remaining observation gap:
it periodically queries only ready sessions whose stored summary is non-terminal
(`queued`, `running`, or `unknown`) and refreshes them from an already-running
sandbox. Once the refreshed summary reaches a terminal state (`idle` or
`needs_attention`), that session falls out of the poll set. The syncer does not
start stopped sandboxes.

## Authentication

### Anonymous Mode (Default)

When `AUTH_ENABLED=false`:
- Uses hardcoded anonymous user
- No session validation
- Suitable for local development

### Authenticated Mode

When `AUTH_ENABLED=true`:
- OAuth2 with PKCE
- Session cookies
- Project membership validation

## Event System

### Event Publishing

```go
// Service emits event
eventBroker.Publish(events.Event{
    Type:      "session_updated",
    ProjectID: projectID,
    Payload: map[string]string{
        "sessionId": sessionID,
        "status":    "ready",
    },
})
```

### Event Subscription (SSE)

```go
// Handler subscribes client
subscriber := eventBroker.Subscribe(projectID)
defer eventBroker.Unsubscribe(subscriber)

for event := range subscriber.Events {
    fmt.Fprintf(w, "data: %s\n\n", event.JSON())
    flusher.Flush()
}
```

## Job System

### Job Types

- `workspace_init` - Clone git repo, setup workspace
- `session_init` - Create sandbox, start agent

### Job Flow

```
1. Handler enqueues job
   │
2. Job saved to database
   │
3. Dispatcher polls for jobs
   │
4. Executor runs job
   │
5. Job status updated
   │
6. Event published (optional)
```

## Sandbox Integration

### Lifecycle

```
Create Workspace → Enqueue workspace_init job
                        │
                        ▼
                   Clone/setup workspace
                        │
                        ▼
Start Chat → Enqueue session_init job
                        │
                        ▼
               Create Docker sandbox
               Mount workspace
               Start agent process
                        │
                        ▼
Chat Message → Update session status to "running"
            → POST sandbox:3002/chat
                        │
                        ▼
               Stream SSE response
                        │
                        ▼
            → Update session status to "ready"
```

On startup, sandbox reconciliation waits for VM-backed providers to finish
initializing, boots any persisted project VMs that still have data disks, and
then compares each sandbox's image ID against the configured runtime image.
Stopped sandboxes with an outdated image are removed so the next session open
recreates them instead of restarting stale containers.

### Sandbox Configuration

```go
type SandboxOptions struct {
    Image       string            // e.g., "discobot-agent-api:latest"
    Binds       []string          // Volume mounts
    Env         []string          // Environment variables
    NetworkMode string            // Docker network
    Labels      map[string]string // Sandbox labels
}
```

## Error Handling

### HTTP Errors

```go
// handlers return appropriate status codes
func (h *Handler) Error(w http.ResponseWriter, err error, status int) {
    h.JSON(w, map[string]string{"error": err.Error()}, status)
}
```

### Service Errors

```go
// services return typed errors
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
```

## Configuration

Key environment variables:

| Variable | Description |
|----------|-------------|
| `PORT` | Server port (default: 3001) |
| `HTTPS_PORT` | Optional HTTPS listener port |
| `HTTPS_TLS_MODE` | HTTPS certificate mode: `ephemeral`, `static`, or `acme` |
| `HTTPS_TLS_HOSTS` | HTTPS SANs / ACME host allowlist |
| `HTTPS_TLS_CERT_FILE` | Static TLS certificate path |
| `HTTPS_TLS_KEY_FILE` | Static TLS private key path |
| `HTTPS_ACME_EMAIL` | Optional ACME contact email |
| `CORS_ORIGINS` | Allowed browser origins; supports `{HTTP_PORT}` and `{HTTPS_PORT}` placeholders |
| `DATABASE_DSN` | Database connection string |
| `WORKSPACE_DIR` | Base directory for workspaces |
| `SANDBOX_IMAGE` | Default sandbox image for local runtimes |
| `SANDBOX_IMAGE_REMOTE` | Default remotely pullable sandbox image for non-local runtimes |
| `AUTH_ENABLED` | Enable authentication |
| `ENCRYPTION_KEY` | AES-256 key for credentials |
| `GITHUB_OAUTH_CLIENT_ID` | GitHub OAuth client ID for GitHub git credentials |
| `GITHUB_OAUTH_CLIENT_SECRET` | GitHub OAuth client secret for GitHub git credentials |

If `HTTPS_PORT` is configured, the server runs a second TLS listener alongside the existing HTTP listener. TLS can be backed by an ephemeral self-signed certificate, a configured static cert/key pair, or ACME/autocert. ACME cache entries are persisted in the database and encrypted with the server encryption key. For trusted HTTPS modes (`static` and `acme`), the HTTP listener redirects regular traffic to HTTPS while still allowing ACME HTTP challenge handling.

CORS defaults are derived from the configured API listener ports instead of hardcoding `:3001`. Custom `CORS_ORIGINS` values can include `{HTTP_PORT}` and `{HTTPS_PORT}` placeholders so callers do not need to duplicate the actual bound ports in multiple settings.

During process startup, the server probes each configured listener port before doing the rest of initialization. It retries the bind check every 10 seconds for up to 2 minutes, closes the temporary listener as soon as the port becomes available, and then proceeds with normal startup.

OAuth credential flows that use the authorization-code redirect path share a
localhost callback server on `127.0.0.1:1455`. The server tries to capture the
browser redirect automatically and exposes callback-status polling endpoints so
the UI can fall back to manual code or redirect-URL paste when the loopback
listener is unavailable.

## Testing

The server includes:
- Unit tests for each package
- Integration tests with real database
- Mock sandbox provider for testing
