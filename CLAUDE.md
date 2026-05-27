# CLAUDE.md

This file provides guidance to coding agents working in this repository.

## Project Overview

Discobot is a coding agent manager. It runs, monitors, and manages its own built-in coding agent across isolated sandboxed sessions. Each session gets its own container with a copy-on-write filesystem, MITM proxy, and agent API. Discobot currently supports Anthropic and OpenAI models, with more model providers to come.

## Commands

### Development

```bash
pnpm dev                # Start all services (backend + Electron app, plus VZ/WSL watcher on macOS/Windows)
pnpm dev:backend        # Backend only (Svelte UI + Go server + agent watcher, plus VZ/WSL watcher on macOS/Windows)
pnpm dev:app            # Electron desktop app only
pnpm dev:frontend       # Active frontend only (Svelte UI on port 3100)
pnpm dev:server         # Go backend with hot-reload via air (port 3001)
```

### Build

```bash
pnpm build              # Build the Electron app (server + frontend + Electron shell)
pnpm build:server       # Build the Go server binary
```

### Lint & Format

```bash
pnpm check              # Run frontend, package.json, workflow, backend, shell, and Windows PowerShell checks
pnpm check:fix          # Run autofixes/formatters, including Windows PowerShell when on Windows
pnpm lint               # Run frontend, backend, shell, and Windows PowerShell linters
pnpm check:frontend     # Delegate to the Svelte UI's Prettier + ESLint + typecheck flow
pnpm check:backend      # golangci-lint (server + proxy + agent-go + authservice)
pnpm format             # Run the Svelte UI Prettier formatter, plus PowerShell formatting on Windows
```

### Tests

```bash
pnpm test               # All unit + integration tests
pnpm test:unit          # All unit tests (server, proxy, agent-go, watcher, ui)
pnpm test:integration   # All integration tests (server + proxy)
pnpm test:ui            # Svelte UI tests only
pnpm test:watcher       # Agent watcher tests only

# Go tests
pnpm test:server        # All server tests
pnpm test:server:unit   # Server unit tests (excludes integration/)
pnpm test:server:integration  # Server integration tests
pnpm test:proxy         # All proxy tests
pnpm test:proxy:unit    # Proxy unit tests
pnpm test:proxy:integration  # Proxy integration tests
pnpm test:agent-go      # Agent Go tests
pnpm test:agent-go:unit # Agent Go unit tests

# Single Go test
cd server && go test -v -run TestName ./internal/path/...

# Single Svelte UI source-level or helper test
node --import tsx --test ui/src/lib/components/test/<test-file>.test.ts

# Single Svelte UI Vitest runtime test
cd ui && pnpm vitest run src/lib/<path>/<test-file>.vitest.ts
```

### CI

```bash
pnpm ci                 # Full CI pipeline: check → test:unit → build
```

## Architecture

### Components

| Component | Language | Port | Purpose |
|-----------|----------|------|---------|
| Frontend | TypeScript (Svelte + Vite) | 3100 | Active web UI |
| Server | Go (Chi + GORM) | 3001 | REST API, session orchestration, container management |
| Agent | Go | — | Container PID 1 init process (workspace setup, AgentFS mount) |
| Agent API | Go | 3002 | Per-container API that drives the AI CLI, SSE streaming |
| Proxy | Go | 17080/17081 | Per-container MITM proxy (auth header injection, Docker registry caching) |

### Data Flow

```
Frontend → REST API (/api/projects/{projectId}/...) → Go Server
                                                        ↓
                                              Docker/VM Container
                                              ├── Agent (PID 1 init)
                                              ├── Agent API (chat/SSE)
                                              └── Proxy (MITM + cache)
```

### Backend Layers

```
Handler (HTTP) → Service (Business Logic) → Store (Data Access) → GORM (SQLite/PostgreSQL)
```

### Resource Hierarchy

```
Project → Workspace (git repo or local folder) → Session (chat thread + container) → Messages + Files
       → Agent (AI config: type, prompt, MCP servers, mode, model)
       → Credential (encrypted API keys / OAuth tokens)
```

### Frontend Patterns

- The UI is being migrated from the SvelteKit frontend in `./ui` to the
  Go + templ + Datastar implementation in `./discobot`. New migration UI work
  should happen in `./discobot` unless a task explicitly targets the legacy
  SvelteKit frontend.
- **Styling**: Tailwind CSS v4 with CSS custom properties. Use design tokens (`bg-background`, `text-foreground`, `border-border`) and IDE tokens (`bg-tree-hover`, `bg-diff-add`)
- **Icons**: Theme-aware via `IconRenderer` component. SVGs with `currentColor` must be inlined, not `<img>`

### Svelte UI (`./ui`)

The Svelte UI is the active frontend. Build and develop it from the `./ui` directory:

```bash
cd ui && pnpm build       # Production build
cd ui && pnpm dev         # Dev server (port 3100)
cd ui && pnpm typecheck   # Type-check (svelte-check)
```

#### Component folders

Components live under `ui/src/lib/components/` in three folders, each with a distinct role:

| Folder | Role | Context |
|--------|------|---------|
| `ui/` | Pure primitives — buttons, inputs, dialogs, etc. | None |
| `ai/` | Self-contained compound components | Component-local only |
| `app/` | App shell — session UI, composer, panels | Global app/session/thread contexts |

**`ui/` is always pure.** Never add context consumers here.

**`ai/` is self-contained.** Each subdirectory is a compound component group with its own `context.ts`. The root component (e.g. `AudioPlayer.svelte`) sets local context; children (e.g. `AudioPlayerPlayButton.svelte`) consume it. These components never use the global app/session/thread contexts.

**`app/` root** contains context consumers and providers — every component here reads from at least one global context. **`app/parts/`** contains pure sub-components that are props-only and used as implementation details by the context consumers at the root. When adding to `app/`, the rule is simple: if it calls `useAppContext`, `useSessionContext`, or `useThreadContext`, it belongs at the `app/` root; if it only takes props, it belongs in `app/parts/`.

#### Global context system

Three contexts flow top-down, each set by a single provider:

| Context | Provider | Provides |
|---------|----------|---------|
| `AppContext` | `routes/+layout.svelte` | Sessions, workspaces, models, credentials, preferences |
| `SessionContext` | `app/SessionWorkspace.svelte` | Threads, files, hooks, services, session credentials |
| `ThreadContext` | `app/ThreadWorkspace.svelte` | Conversation, messages, plan entries |

Access via `useAppContext()`, `useSessionContext()`, `useThreadContext()` from `$lib/context/`.

#### Pure vs context consumer

Make a component **pure** when it can be described and tested without knowing its parent, and all the data it displays is passed directly via props.

Make a component a **context consumer** when it would require threading the same data through multiple intermediate components that don't use it, or when it needs to coordinate with siblings that share the same ambient state.

The practical test: if removing `useXxxContext()` would mean adding three or more props that just relay data the context already holds, it belongs in context. If the component makes sense anywhere in the tree without a specific ancestor, it should be pure.

### Adding Features

1. Add Go handler/service/store in `server/internal/`
2. Build current Svelte UI changes in `ui/src/`, or migration UI changes in
   `discobot/content` and `discobot/static`

## Testing

**Svelte UI tests use both runners, depending on the test type:**
- Use **Vitest** for Svelte component tests and runtime tests that import rune-backed `.svelte.ts` modules.
- Use **Node's built-in `node:test`** for plain TypeScript helper tests and source-level assertion tests that do not rely on Svelte/Vite transforms.

**Go tests** use standard `go test`. Integration tests are under `*/internal/integration/`.

## Formatting & Style

- **Package manager**: pnpm only (never npm or yarn)
- **TypeScript / Svelte UI**: Prettier + ESLint — tabs, double quotes, organized imports where applicable
- **Go**: gofmt + goimports with local prefix `github.com/obot-platform/discobot`
- **Go version**: 1.26 — use `new(value)` to create a pointer to a value (e.g. `new(true)` for `*bool`); avoid `boolPtr`/`intPtr` helper functions
- **Go linters**: golangci-lint (errcheck, govet, staticcheck, revive, unused, etc.)
- **PowerShell**: PSScriptAnalyzer for linting and Invoke-Formatter formatting on Windows
- **Git commit messages**: use Conventional Commits for every commit, with a type-based subject like `feat(scope): short description` (for example, `feat(ui): add session filter`). Keep the subject line to 50 characters or fewer when possible. Always include a commit body after a blank line, even for small changes, and use it to explain the nature of the change, the key decisions, and any important context. Wrap body lines to 72 characters or fewer
- **Code style**:
  - Optimize for code that a maintainer can read once and understand. Prefer
    boring, direct control flow over clever abstractions.
  - Keep simple code simple. Do not add helpers, layers, interfaces, or
    temporary variables unless they make the code easier to scan or remove a
    real repetition problem.
  - Give complex setup code a visible shape. For long constructors,
    lifecycle/orchestration flows, or HTTP server wiring, prefer an explicit
    state struct and named phases such as `initCredentials`, `initRuntime`,
    `routes`, and `close`.
  - Split by meaningful lifecycle boundaries, not by arbitrary line count. Good
    boundaries include configuration loading, dependency construction,
    background process startup, route registration, and cleanup.
  - Avoid dense "blob" functions: a function with many unrelated local
    variables, callbacks, and cleanup steps should usually become a small
    object with clear methods or a short top-level outline.
  - Keep related logic together. Do not scatter a straightforward operation
    across many tiny helpers just to make individual functions short.
  - Separate concerns when the lifecycle is non-trivial: runtime object
    construction, HTTP route assembly, background process startup, and cleanup
    should be easy to identify independently.
  - Prefer descriptive names in broad scopes (`runtime`, `router`,
    `credentialManager`, `providerRegistry`) over terse names (`r`, `h`,
    `mgr`, `reg`). Short names are fine only in very small, obvious scopes.
  - Do not compress multi-field structs, long function calls, or callbacks into
    hard-to-scan one-liners just to save vertical space. Use vertical space to
    make important structure visible.
  - Accept small, local duplication when it avoids premature abstraction or
    keeps a call site easier to understand.
  - Add comments when they help the reader understand intent: function comments
    for non-trivial code, short section comments for setup phases, and inline
    comments for dependency ordering, tricky logic, edge cases, or behavior that
    is surprising from the code alone. Avoid comments that simply restate the
    next line.

## Documentation

When making changes, update the relevant docs:

- `docs/ARCHITECTURE.md` — System-wide architecture
- `docs/ui/ARCHITECTURE.md` — Frontend patterns
- `docs/design/` — Cross-cutting design docs
- `server/docs/` — Server architecture and design docs
- `sandbox-init/docs/` — Sandbox init process docs
- `agent-go/docs/` — Agent API docs
- `server/README.md`, `sandbox-init/README.md`, `proxy/README.md` — Component READMEs

## Known Quirks

1. **Terminal resize**: Uses debounced `requestAnimationFrame` to avoid loops
2. **Icon rendering**: SVGs with `currentColor` must be inlined, not used as `<img>`
