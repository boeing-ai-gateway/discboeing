# Discobot UI Go

This is the starting point for the Go-based Discobot UI rewrite. The top-level
server uses Chi. `GET /` is explicitly routed to `content/root.templ`, and all
other paths are served from the static file directory with Go's standard
`http.FileServer`.

By default, static files are served from `./ui-go/static`. Set
`UI_GO_STATIC_DIR` to point at a different directory during development. The
Go UI listens on port `3200` by default; set `UI_GO_PORT` to override it. It
uses `DISCOBOT_API_BASE_URL` to reach the backend API, defaulting to
`http://127.0.0.1:3001`.

The first Datastar slice is intentionally small:

- `GET /` renders one full templ page shell.
- `/vendor/datastar.js` serves the pinned Datastar browser bundle.
- `GET /ui/stream` opens the primary Datastar SSE read stream.
- `POST /ui/commands/sidebar-refresh` is a short-lived command that patches the
  sidebar island.

The stream currently patches a proof read model in `#app-sidebar`. Replace that
read model with real sessions/workspaces in the next migration slice.

## Commands

```bash
pnpm ui-go:dev      # Build assets, generate templ files, and run with Air on port 3200
pnpm ui-go:build    # Build assets, generate templ files, and build ./ui-go/build/ui-go
pnpm ui-go:check    # Build assets, generate, test, and lint the Go UI
```

During development, Air watches `.templ`, Go, JavaScript, Tailwind source CSS,
and the shared `server/api` client package. It rebuilds the static assets, runs
`pnpm generate`, rebuilds the Go server, and restarts it. Generated `*_templ.go`
and `static/app.css` files are ignored by Air to avoid rebuild loops.

## Styling

`styles/app.css` is the Tailwind v4 source for the Go UI. It ports the same
Fontsource imports, theme tokens, theme variants, and Tailwind `@theme` mappings
from `../ui/src/app.css`, then adds a small component layer for the current templ
proof shell.

The Tailwind source scans both `ui-go` templ/Go files and `../ui/src` so classes
copied from the Svelte UI are available during the migration. `pnpm assets:build`
copies the Fontsource font files into the generated `static/files` directory
and compiles the served CSS to `static/app.css`.

## Datastar UI guidelines

Keep transient browser-only state local. Editable text, popover visibility,
focus/cursor state, and debounce timers should live in Datastar local signals or
component-scoped TypeScript instead of being echoed through server patches on
every keystroke.

Keep committed and validated state server-authoritative. Workspace selection,
validation results, suggestions, model/reasoning/service-tier choices,
attachments, and security-sensitive/shared state should be stored by the server
and patched back into the UI.

Prefer component-scoped TypeScript modules for imperative behavior and wire them
from templates with `data-on:*` bindings. The top-level JS entrypoint should stay
focused on bootstrapping and enhancement orchestration.
