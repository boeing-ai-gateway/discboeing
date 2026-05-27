# Discobot

A minimal Datastar + templ application that keeps rendering code in `content`
and server/application state in `internal`.

## Commands

```bash
pnpm dev
pnpm build
pnpm check
```

The server listens on port `3300` by default. Override it with
`DISCOBOT_PORT` and override static assets with `DISCOBOT_STATIC_DIR`.

## Structure

- `cmd/discobot` — executable entry point
- `content/root.templ` — document shell and top-level page render
- `content/components/app` — application-specific templ components
- `content/components/ui` — reusable low-level templ primitives
- `internal/state` — `Shell`, `Data`, and `View` render/application state
- `internal/config` — environment configuration
- `internal/server` — HTTP routes and Datastar command handlers
- `static` — editable reset/theme/app CSS and vendored/generated assets
- `styles` — source CSS using native cascade layers

## Design system

See [`DESIGN_SYSTEM.md`](./DESIGN_SYSTEM.md) for the VS Code-inspired layout,
spacing, and token guidelines. `static/themes.css` mirrors the semantic theme
token names from `../ui/src/app.css` where practical, while the CSS uses native
cascade layers instead of Tailwind.
