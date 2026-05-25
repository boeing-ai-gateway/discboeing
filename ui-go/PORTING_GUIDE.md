# ui-go Single-File Porting Guide

Use this guide when porting one Svelte UI file to `ui-go`. It is written for
agents that may run in parallel, so each task should keep its changes scoped to
one component and clearly report any shared model, JavaScript, or server changes
it had to make.

## Goal

Port one Svelte component to templ/Datastar with functional parity, not just
visual markup parity. A complete port accounts for:

- HTML structure and Tailwind classes
- props and derived state
- read-model fields in Go
- Datastar signals and server command endpoints
- JavaScript islands needed for browser-only behavior
- streaming and patching behavior
- accessibility and keyboard behavior
- tests or validation steps

Do not treat the templ file as isolated if the Svelte file depends on stores,
contexts, effects, browser APIs, or command handlers.

## Before You Start

1. Identify exactly one source file to port.
   - Svelte source: usually under `ui/src/lib/components/app/`,
     `ui/src/lib/components/app/parts/`, or `ui/src/routes/`.
   - Go target: usually under `ui-go/content/lib/components/app/`,
     `ui-go/content/lib/components/app/parts/`, or `ui-go/content/`.
2. Read the Svelte file and all direct dependencies it imports.
3. Read the existing Go target file, if present.
4. Read these shared Go files if the component needs state:
   - `ui-go/content/lib/viewmodel/view.go`
   - `ui-go/cmd/ui-go/main.go`
   - nearby helpers such as `view_helpers.go` or `parts/branding.go`
5. Do not edit generated `*_templ.go` files directly. Change `.templ` files and
   run generation only when validating.

## Component Inventory, Missing Files, and Shared Status

This guide applies to both:

1. Existing templ files that are partial ports.
2. Svelte components that do not yet have any `ui-go` counterpart.

All agents must use the shared status file:

```text
ui-go/PORTING_STATUS.md
```

Before changing a component:

1. Find its section in `ui-go/PORTING_STATUS.md`.
2. Change its checkbox from `[ ]` to `[~]`.
3. Set `Owner/task` to your task or agent identifier.
4. Add a note describing the scope you are taking.

After changing a component:

1. Set the checkbox to `[x]` if parity is complete for the current available
   infrastructure, or `[!]` if blocked by missing shared infrastructure.
2. Update `Status`, `Shared files touched`, `Validation`, and `Notes`.
3. Leave other component sections untouched.

Parallel agents may conflict in this file. That is expected. Keep edits limited
to your assigned component section so conflicts are easy to resolve.

### Classify the Svelte Component

For every Svelte component discovered under `ui/src/lib/components`, classify it
before creating or editing a target:

| Svelte location | `ui-go` target |
| --- | --- |
| `app/*.svelte` | `ui-go/content/lib/components/app/*.templ` |
| `app/parts/*.svelte` | `ui-go/content/lib/components/app/parts/*.templ` |
| `ai/<group>/*.svelte` | `ui-go/content/lib/components/ai/<group>/*.templ` |
| `ui/<group>/*.svelte` | `ui-go/content/lib/components/ui/<group>/*.templ` |
| route/layout files | `ui-go/content/` or a route-specific package |

Use snake_case file names for new templ files, matching the existing `ui-go`
style. For example:

```text
AppHeader.svelte                -> app_header.templ
ThreadWorkspaceHeader.svelte    -> thread_workspace_header.templ
ConversationComposerTextarea    -> conversation_composer_textarea.templ
```

### Creating a New Templ Target

When no templ file exists yet:

1. Create the `.templ` file in the matching directory.
2. Use the Go package for that directory.
   - `ui-go/content/lib/components/app` uses `package app`.
   - `ui-go/content/lib/components/app/parts` uses `package parts`.
   - New `ai` or `ui` directories should use package names that match their
     folder when possible.
3. Add small helper functions in a colocated `.go` file only when needed.
4. Add shared read-model fields to `viewmodel` only when the component consumes
   app/session/thread state.
5. Update sample data in `cmd/ui-go/main.go` only if the component needs example
   states to render correctly.
6. Never create or edit generated `*_templ.go` files by hand.
7. Run `pnpm --dir ./ui-go generate` only during validation.

### When Not to Port a Component Yet

Some Svelte files are low-level UI primitives or wrappers around browser-heavy
libraries. It is acceptable to mark a component as blocked in
`PORTING_STATUS.md` when parity requires infrastructure that does not exist yet.

Examples:

- `ui/resizable/*` may be blocked until the Go UI has a shared resizable-pane
  JavaScript island.
- terminal, desktop, VSCode, Monaco, and noVNC panels may be blocked until their
  browser-side clients and server proxy routes exist.
- dialog/menu/popover primitives may be blocked until a shared accessibility
  strategy is chosen.

When blocked, add a precise note listing the missing shared system and any small
skeleton that was still implemented.

## Required Parity Checklist

For the file you are porting, document and implement each category below.

### 1. Markup and Styling

Compare the rendered DOM, not just the component name.

- Root element tag and attributes
- Tailwind classes, including responsive classes
- Conditional branches
- Slots/children equivalents
- `aria-*`, `role`, `title`, `alt`, and labels
- `data-*` attributes used by Electron, Datastar, tests, or scripts
- Icon choice and sizing
- Empty, loading, selected, active, disabled, and error states

Use existing design tokens from `ui-go/styles/app.css`; do not introduce ad hoc
CSS unless the Svelte component also requires custom behavior that cannot be
expressed in utilities.

### 2. Props to Go Read Model

Svelte props, contexts, and derived values must become explicit Go read-model
fields or server-side derived fields.

Shared read models live in:

```text
ui-go/content/lib/viewmodel/view.go
```

Rules:

- Add only fields needed by the component being ported.
- Prefer Svelte names when they are clear (`ReserveSidebar`, `ThreadState`,
  `DisplayName`, `Selected`, etc.).
- Keep raw backend values and display values separate when Svelte does.
  For example, keep `Name` and `DisplayName` separate if the UI has fallback
  behavior.
- Put display fallback helpers in a nearby Go helper file when multiple
  components need the same behavior.
- Update sample data in `ui-go/cmd/ui-go/main.go` so every new field has at
  least one representative state.

Example pattern:

```go
type SidebarThreadItem struct {
    ID          string
    Name        string
    DisplayName string
    Selected    bool
    Status      string
    State       string
    Primary     bool
    Children    []SidebarThreadItem
}
```

If the component needs real backend data that does not exist yet, add the
smallest read-model placeholder and report the missing backend query in your
final summary.

### 3. Datastar Signals

Use Datastar signals for client-observable UI state that must update without a
full page reload.

Signals are initialized in templ with attributes such as:

```html
<body data-signals="{streamOpen: false}" data-init="@get('/ui/stream')">
```

When adding signals:

1. Define the initial signal near the component root or page root.
2. Name it narrowly, using the component or session prefix when appropriate.
3. Patch it from Go with Datastar when server state changes.
4. Avoid global signal names for component-local state if multiple instances may
   render on the page.

Go patch example:

```go
sse := datastar.NewSSE(w, r)
if err := sse.MarshalAndPatchSignals(map[string]any{
    "streamOpen": true,
}); err != nil {
    slog.Warn("failed to patch signal", "error", err)
    return
}
```

Use signals for simple state such as:

- sidebar open/closed
- selected tab or active panel
- dialog open/closed
- pending command state
- counters or booleans displayed in the component

Do not force complex browser objects such as terminals, Monaco editors, noVNC,
or WebSocket clients into Datastar signals. Use JavaScript islands for those.

### 4. Server Commands and CQRS-Style Flow

Most Svelte actions become command endpoints in `ui-go/cmd/ui-go/main.go` or a
small handler file if the command set grows.

Use this convention unless a route already exists:

```text
POST /ui/commands/<domain>/<action>
POST /ui/commands/sessions/{sessionID}/<action>
POST /ui/commands/sessions/{sessionID}/threads/{threadID}/<action>
```

Examples:

```text
POST /ui/commands/sidebar-refresh
POST /ui/commands/sessions/{sessionID}/select
POST /ui/commands/sessions/{sessionID}/threads/{threadID}/select
POST /ui/commands/sessions/{sessionID}/threads/{threadID}/rename
```

Command handler requirements:

1. Parse and validate path params and form/body values.
2. Mutate server-side UI/application state or call the backend service.
3. Rebuild the affected read model.
4. Return a Datastar SSE response that patches only the affected fragment or
   signal.
5. Log warnings with enough context, but do not expose secrets in the DOM.

Patch example:

```go
func (s *uiState) handleSidebarRefresh(w http.ResponseWriter, r *http.Request) {
    s.commands.Add(1)

    sse := datastar.NewSSE(w, r)
    if err := sse.PatchElementTempl(app.AppSidebar(s.sidebarSnapshot())); err != nil {
        slog.Warn("failed to patch sidebar command response", "error", err)
    }
}
```

Templ command trigger example:

```html
<button
    type="button"
    data-on-click="@post('/ui/commands/sidebar-refresh')"
>
    Refresh
</button>
```

If the user says "CQRS commands", use this model: query/read model functions
produce snapshots; command handlers mutate state and patch snapshots. Keep read
model construction separate from command mutation.

### 5. CORS Requirements

Most `ui-go` component ports should not need CORS changes because the UI and its
`/ui/commands/*` endpoints are served from the same origin.

Only add CORS handling if the component must call a different origin directly
from browser JavaScript, such as a service iframe, dev server, or external
preview endpoint. Before adding CORS:

1. Prefer same-origin proxying through the Go server.
2. If direct cross-origin browser access is required, add the narrowest allowed
   origin, methods, and headers.
3. Never allow credentials with `Access-Control-Allow-Origin: *`.
4. Document why same-origin proxying was insufficient.

Suggested helper shape if CORS becomes necessary:

```go
func withCORS(next http.Handler, allowedOrigin string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        origin := r.Header.Get("Origin")
        if origin == allowedOrigin {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Vary", "Origin")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        }
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Do not add broad CORS middleware as part of a normal component port.

### 6. JavaScript Islands

Use JavaScript when Svelte behavior depends on browser APIs, persistent client
objects, or event lifecycles that Datastar attributes cannot express cleanly.

Examples that require JavaScript:

- Monaco editor
- Ghostty terminal
- noVNC desktop
- service iframes with resize or auth behavior
- drag-to-resize panes
- keyboard shortcuts
- clipboard and file download APIs
- scroll position tracking and auto-scroll
- focus traps that are not handled by native elements

Preferred file layout:

```text
ui-go/static/js/<feature>.js
```

Load scripts from `root.templ` or the smallest containing component:

```html
<script type="module" src="/js/session-workspace.js"></script>
```

JavaScript island rules:

1. Export an initializer, or register behavior by scanning for `data-ui-*`
   attributes.
2. Make initialization idempotent. Datastar may patch a fragment multiple times.
3. Clean up event listeners, timers, observers, WebSockets, and workers when the
   owning DOM node is removed.
4. Store per-element state in a `WeakMap`.
5. Do not embed secrets in the DOM or JavaScript.
6. Keep component-specific JavaScript separate from unrelated components.

Recommended pattern:

```js
const controllers = new WeakMap();

function initWidget(root) {
    if (controllers.has(root)) return;

    const abort = new AbortController();
    root.addEventListener("click", onClick, { signal: abort.signal });

    controllers.set(root, { abort });
}

function destroyWidget(root) {
    const controller = controllers.get(root);
    if (!controller) return;
    controller.abort.abort();
    controllers.delete(root);
}

function scan() {
    document.querySelectorAll("[data-ui-widget]").forEach(initWidget);
}

scan();
document.addEventListener("datastar:after-patch", scan);
```

If Datastar does not emit the exact event needed, use a `MutationObserver` to
scan newly added DOM and clean up removed nodes.

### 7. Updating Rendering After Commands

Every command must decide what it patches:

- Patch a full component when the read model changed substantially.
- Patch a small target element when only a child changed.
- Patch signals when DOM can remain the same.
- Use append/prepend behavior only for logs or message streams.

Component roots that may be patched should have stable IDs or stable data
attributes. Example:

```html
<aside id="app-sidebar">
    ...
</aside>
```

Then command response:

```go
if err := sse.PatchElementTempl(app.AppSidebar(snapshot)); err != nil {
    slog.Warn("failed to patch app sidebar", "error", err)
}
```

For one component port, add only the patch targets it owns. Do not rename shared
IDs without coordinating with other ports.

### 8. Context Translation

Svelte contexts map to Go read models and server state:

| Svelte context | Go equivalent |
| --- | --- |
| `AppContext` | `viewmodel.ShellSnapshot`, app-level `uiState` fields |
| `SessionContext` | `SessionWorkspaceSnapshot`, per-session state map |
| `ThreadContext` | `ThreadWorkspaceSnapshot` or thread fields nested under session |

When a Svelte component calls `useAppContext`, `useSessionContext`, or
`useThreadContext`, list every consumed field before editing. Add only the fields
needed by the file being ported.

### 9. Parallel-Agent Conflict Rules

Agents will run in parallel and may conflict. That is acceptable, but minimize
unnecessary conflicts:

- Edit only the component you were assigned and its direct helpers.
- Update only your component section in `ui-go/PORTING_STATUS.md`.
- If you must edit `viewmodel/view.go`, add the smallest fields possible.
- If you must edit `cmd/ui-go/main.go`, keep handler names and routes narrowly
  scoped to your component.
- Do not reformat unrelated files.
- Do not edit generated `*_templ.go` files by hand.
- Do not rename shared structs, routes, IDs, or helpers unless your task is
  explicitly responsible for that shared concept.
- In your final response, list every shared file touched so conflicts are easy
  to resolve.

Generated files will change after `pnpm generate`; that is expected. Source of
truth remains `.templ`, `.go`, `.js`, CSS, and docs.

### 10. Validation

For component-only changes:

```bash
pnpm --dir ./ui-go generate
pnpm --dir ./ui-go check
```

For JavaScript or CSS changes, also ensure assets build through the normal check:

```bash
pnpm --dir ./ui-go assets:build
```

Do not commit or push from an agent task unless explicitly asked.

## Single-File Porting Template

Agents should use this template in their final summary.

```text
Ported file:
- Svelte source:
- Go target:

Implemented parity:
- Markup/classes:
- Read model fields:
- Datastar signals:
- Command endpoints:
- JavaScript islands:
- Accessibility:

Shared files touched:
-

Validation run:
-

PORTING_STATUS.md updated:
- yes/no

Remaining blockers:
-
```

## When to Stop and Report Instead of Building

Stop and report if completing parity would require any of these larger systems
and they do not already exist:

- full session/thread backend query integration
- terminal implementation
- Monaco editor implementation
- noVNC desktop implementation
- multi-panel dock/resizable pane system
- authentication or credential plumbing
- broad CORS middleware
- cross-component state architecture changes

In those cases, implement the smallest safe skeleton or read-model addition for
the assigned file, then describe the missing system precisely.
