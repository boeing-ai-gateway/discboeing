# ui-go2

A minimal Datastar + templ scaffold that mirrors the broad `ui-go` project
shape while keeping the application intentionally small.

## Commands

```bash
pnpm dev
pnpm build
pnpm check
```

The server listens on port `3300` by default. Override it with
`UI_GO2_PORT` and override static assets with `UI_GO2_STATIC_DIR`.

## Structure

- `cmd/ui-go2` — executable entry point
- `content` — templ page and component tree
- `content/lib/viewmodel` — backend-authored view/data state
- `content/lib/components/ui` — pure primitives
- `content/lib/components/ai` — self-contained compound UI
- `content/lib/components/app` — app shell components
- `content/lib/components/app/parts` — props-only app sub-components
- `internal/config` — environment configuration
- `internal/server` — HTTP routes and Datastar commands
- `static` — generated CSS and vendored Datastar bundle
- `styles` — Tailwind input
