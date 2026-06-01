# Discobot templ/Datastar guidelines for agents

Use these rules when changing files under `discobot/`. This document is scoped to
that folder only.

## Project boundary

- Treat `discobot/` as a standalone Go + templ + Datastar app.
- Edit source files only: `.templ`, `.go`, `.js`, CSS, docs, and scripts.
- Generated `*_templ.go` files are build/watch outputs. They may appear in diffs
  after editing `.templ` files, but agents and reviewers should not review or
  comment on their contents; review the corresponding `.templ` source instead.
- Do not edit generated `*_templ.go` files directly by hand.
- Validate with `pnpm --dir discobot check` when practical.

## Architecture

- `content/` renders HTML.
- `content/components/app/` contains app-specific components and may read
  `internal/state` render models.
- `content/components/ui/` contains reusable primitives. Keep these
  app-agnostic; pass data and command URLs in through explicit structs.
- `internal/state` owns server-side app and view state.
- `internal/sync` mirrors the running Discobot API server into `state.Data`.
  It subscribes before loading REST snapshots, queues project events while the
  replacement cache is built, swaps the consistent cache through `SaveData`, and
  notifies on every later cache mutation so `/ui/stream` can patch the shell.
- `internal/command` owns command handlers.
- `internal/server` owns routes, `/ui/stream`, and static serving.

## State ownership

- Keep authoritative state on the server.
- Put domain/app data in `state.Data`.
- Put persistent UI view state in `state.View`.
- Use browser state only for transient behavior: focus, menu position, drag
  hover, keyboard navigation, local theme selection, and short timers.
- Do not create client-side state machines for sessions, files, sidebar state,
  or other shared application state.

## The Datastar way

- Think hypermedia, not SPA. The backend decides the next valid UI and sends
  HTML/signal patches.
- Start with Datastar defaults. Do not add custom request/morphing/history logic
  unless the default path is insufficient.
- Prefer SSE responses and server-rendered templ fragments over JSON APIs plus
  client-side rendering.
- Trust morphing. Send a stable component root with a stable ID or `data-*` hook
  instead of hand-updating many tiny children.
- Use signals sparingly for local UI values and request payload state. Do not put
  broad app state, authorization state, or secrets in signals.
- Do not use optimistic UI by default. Show pending/loading state and patch the
  confirmed server result.
- Use normal links/navigation unless an interaction truly needs a Datastar
  command.
- Keep Datastar expressions small. Move complex browser behavior to a small JS
  island or server command.
- Remember attribute order matters. Put `data-indicator` before `data-init` when
  an init request should use that indicator.
- Prefix private/local request-excluded signals with `_` when they must not be
  sent back to the server.

## Datastar flow

- The normal interaction flow is:
  1. templ element triggers a command.
  2. command mutates `state.Data`, `state.View`, or both.
  3. command returns `204 No Content` when the request is complete.
  4. `/ui/stream` patches `AppShell`.
- Prefer server-rendered templ patches over DOM-building JavaScript.
- Keep Datastar signals narrow. The current root signal is `streamOpen`; do not
  add broad app state to signals.
- Use stable IDs and `data-*` hooks for patch targets and JS islands.
- When toggling panels, prefer keeping the panel element mounted and hiding it
  with CSS instead of removing it from the DOM. This preserves local browser
  state such as scroll position, focus, editor/terminal state, and initialized JS
  islands while the panel is not visible.

## Commands

- Add command routes under `/ui/commands` in `internal/command/handler.go`.
- Name routes by domain and action, for example:
  - `/sidebar/toggle`
  - `/sessions/{id}/select`
  - `/files/{id}/toggle-expanded`
- Use `SaveView`, `SaveData`, or `SaveShell` according to what changes.
- Validate all path params and payloads server-side.
- For new templ command actions, prefer `@discobotCommand(...)` so pending
  state and command events stay consistent.
- Use plain Datastar `@post(...)` only for simple cases where command pending
  state and custom payload handling are not needed.

## JavaScript islands

- Use JS only when templ + Datastar attributes are not enough.
- Initialize islands by scanning explicit `data-*` hooks.
- Make initialization idempotent with a `data-*-ready` guard.
- Avoid duplicate document-level listeners after Datastar patches.
- Keep persistent data out of JS. JS may manage transient behavior only.

## UI and CSS

- Follow the compact VS Code-like IDE design system.
- Use semantic CSS tokens from `static/themes.css`, such as `--foreground`,
  `--muted-foreground`, `--border`, `--surface-canvas`, `--surface-card`,
  `--surface-control`, and `--focus-accent`.
- Tailwind v4 is the styling entrypoint. Edit `styles/app.css`; do not edit the
  generated `static/app.css` by hand.
- `pnpm --dir discobot styles:build` builds `styles/app.css` to
  `static/app.css`, and `pnpm --dir discobot check` runs that build as part of
  `assets:build`.
- Prefer Tailwind utility classes in `.templ` markup for straightforward layout,
  spacing, typography, color, and borders.
- Keep templates free of inline `<style>` blocks. Shared or complex selector
  CSS belongs in `styles/app.css` under the appropriate layer, usually
  `@layer components`.
- Use `data-class:*`, `data-style:*`, and `data-attr:*` for dynamic classes,
  styles, and attributes when a Datastar attribute can express the change
  clearly. Keep the backend authoritative for the values rendered into those
  attributes.
- For custom CSS logic in templates, prefer inline `data-class:*` and
  `data-style:*` attributes at the element that needs the behavior. Do not call
  an external helper method unless the logic is truly involved or reused in a
  way that makes the template clearer.
- Avoid server-side helper functions whose only job is to concatenate dynamic
  class strings or full style declarations. Prefer static base classes plus
  `data-class:*` toggles, and prefer `data-style:*` for CSS custom properties or
  measured values.
- Avoid hard-coded colors in templates.
- Preserve the dense shell: small controls, subtle borders, low shadows, and
  tool-like buttons.

## Icons

- Use existing typed icon helpers in `content/components/app/icon.templ` for app
  components.
- Use the reusable Lucide helper in UI primitives.
- If adding an app icon, add a typed helper in
  `content/components/app/icon.templ` when appropriate and regenerate
  templates.
- Do not add ad hoc inline SVGs unless there is no existing icon path.

## Accessibility

- Preserve semantic HTML and ARIA attributes.
- Menus should use `role="menu"`, `role="menuitem"`, `aria-haspopup`, and
  `aria-expanded` as appropriate.
- Trees should use `role="tree"`, `role="treeitem"`, `aria-selected`, and
  `aria-expanded` as appropriate.
- Icon-only buttons need an `aria-label` or screen-reader text.
- Buttons should use `type="button"` unless they intentionally submit a form.
- Preserve keyboard behavior when editing interactive components.

## Security

- Never trust Datastar signals, DOM attributes, or command payloads.
- Do not expose secrets in signals, `data-*` attributes, HTML, JS, or logs.
- Escape user-controlled text through templ. Avoid unsafe HTML.
- Keep command handlers responsible for authorization, validation, and state
  invariants.
