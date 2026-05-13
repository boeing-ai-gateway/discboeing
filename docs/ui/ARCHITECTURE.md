# UI Architecture

This document describes the architecture of the Discobot frontend, a SvelteKit application built with Vite that provides an IDE-like chat interface for AI coding agents.

## Overview

The UI is a single-page SvelteKit application. It renders an IDE-style interface with resizable panels for workspace navigation, chat/terminal, file diffs, and embedded session tools such as the desktop viewer and VS Code.

```
┌─────────────────────────────────────────────────────────────────┐
│                      Header (logo, controls)                     │
├─────────────────────────────────────────────────────────────────┤
│ Left Sidebar  │              Main Content                        │
│ ┌───────────┐ │  ┌─────────────────────────────────────────┐    │
│ │ Workspace │ │  │           Diff Panel (tabs)             │    │
│ │   Tree    │ │  ├─────────────────────────────────────────┤    │
│ ├───────────┤ │  │        Bottom Panel                     │    │
│ │  Agents   │ │  │     (Chat or Terminal)                  │    │
│ │   Panel   │ │  └─────────────────────────────────────────┘    │
│ └───────────┘ │                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
ui/src/
├── app.html                   # HTML shell (favicon, meta)
├── hooks.client.ts            # Browser startup hooks (Sentry init, build-time release metadata, client error capture)
├── routes/
│   └── +layout.svelte         # AppContext provider, shared project stream subscription
├── lib/
│   ├── context/               # useAppContext, useSessionContext, useThreadContext
│   ├── api-client.ts          # REST API client
│   ├── api-types.ts           # TypeScript interfaces
│   ├── api-config.ts          # API configuration (server-config discovery, HTTPS)
│   ├── components/
│   │   ├── ui/                # Pure primitives (buttons, inputs, dialogs)
│   │   ├── ai/                # Self-contained compound AI components
│   │   └── app/               # App shell context consumers
│   │       └── parts/         # Props-only sub-components
│   ├── markdown/              # Native markdown renderer (remend + unified)
│   └── session/
│       └── runtime/           # Chat stream reducer state
└── static/                    # Favicon and static assets
```

## Component Architecture

### Component Folders

Components live under `ui/src/lib/components/` in three folders:

| Folder | Role                                             | Context                            |
| ------ | ------------------------------------------------ | ---------------------------------- |
| `ui/`  | Pure primitives — buttons, inputs, dialogs, etc. | None                               |
| `ai/`  | Self-contained compound components               | Component-local only               |
| `app/` | App shell — session UI, composer, panels         | Global app/session/thread contexts |

### Global Context System

Three contexts flow top-down, each set by a single provider:

| Context          | Provider                      | Provides                                               |
| ---------------- | ----------------------------- | ------------------------------------------------------ |
| `AppContext`     | `routes/+layout.svelte`       | Sessions, workspaces, models, credentials, preferences |
| `SessionContext` | `app/SessionWorkspace.svelte` | Threads, files, hooks, services, session credentials   |
| `ThreadContext`  | `app/ThreadWorkspace.svelte`  | Conversation, messages, plan entries                   |

Access via `useAppContext()`, `useSessionContext()`, `useThreadContext()` from `$lib/context/`.

## Key Architectural Decisions

### 1. Single-Page Application with SvelteKit

The application uses SvelteKit's adapter-static for a fully client-side SPA. Panel content is driven by Svelte stores and context rather than URL routes.

### 2. Data Fetching and Realtime Streams

All server data is fetched via `ui/src/lib/api-client.ts`. App-level project events, multiplexed chat streams, and service log streams now share one project-scoped WebSocket (`/api/projects/{projectId}/ws`). The app uses targeted reloads for affected sessions and workspaces, with full refreshes on reconnect. The store layer coalesces concurrent reloads into at most one active request plus one queued follow-up per resource key to avoid duplicate fetches during reconnect bursts. Queued thread prompts can carry a `runAfter` time; the UI treats far-future values as paused, shows live relative countdowns for scheduled prompts, and the agent now arms an idle timer so the next eligible queued prompt starts automatically when its scheduled time arrives. The composer mirrors that scheduling UI with a clock button next to submit, and a scheduled composer submit queues the prompt with `runAfter` instead of starting it immediately. Shared SWR-style store primitives now live under `ui/src/lib/resource/` and `ui/src/lib/store/`: `createResource(...)` models one async resource, `createIndexedResource(...)` wraps keyed list/item caching, and `createEntityStore(...)` adds a conditional CRUD-oriented store surface with collection state, optional indexed item state, and cache reconciliation policies for create/update/remove operations. UI testing now uses Vitest for Svelte component tests and rune-backed `.svelte.ts` runtime tests, while plain source-shape and helper tests can stay on `node:test`.

### 3. API Configuration

`ui/src/lib/api-config.ts` discovers `/api/server-config` to prefer the server's HTTPS listener for browser traffic when trusted HTTPS is available, while Tauri continues using its direct local HTTP connection.

### 4. Chat Streaming

The thread chat stream reducer (`ui/src/lib/thread/conversation-stream.ts`) processes AI SDK-style data frames:

- Buffers `history-start`/`history-message`/`history-end` replay events
- Applies `chunk`/`done` events to materialize live AI messages
- Surfaces mode/model/reasoning metadata

Mounted thread workspaces subscribe through a single app-scoped project WebSocket, which multiplexes chat streams, project events, and service logs across multiple sessions and avoids holding separate SSE connections per mounted workspace or service panel. The shell only preloads the active session plus recent sessions the user has already opened, so sidebar recents can remain visible without eagerly creating session or thread contexts for untouched sessions.

The dock reserves `discobot-desktop` and `discobot-vscode` as first-class panes. Both are backed by the existing per-session service proxy, but the VS Code pane is rendered separately from generic service previews so it can use a looser iframe policy and its own toolbar. The editor entry point is disabled by default and can be exposed from Settings, which lets the UI hide the editor button entirely without changing service discovery.

The sandbox image seeds code-server profile defaults from `container-assets/code-server/` into `/home/discobot/.local/share/discobot-code-server/` on first launch. The cache volume now preserves `/home/discobot/.local/share/discobot-code-server/User` and `/home/discobot/.local/share/discobot-code-server/extensions` by default so editor settings and installed extensions carry across sessions, while workspace `.vscode/` settings still apply as normal workspace-level overrides. When `code-server` is present in the sandbox PATH, the agent exposes `discobot-vscode` as a built-in passive service, and the `VSCodePanel` writes the resolved light/dark mode to `~/.discobot/editor/.vscode-theme.json` so the bundled theme extension can watch that home-scoped file and keep the embedded editor aligned with the surrounding UI. The same home-scoped editor directory also carries one-shot control commands such as `~/.discobot/editor/.vscode-control.json` for opening workspace files in the embedded editor without adding repository-visible files.

Reasoning is level-based and sourced from each model's `reasoningLevels`/`defaultReasoning` metadata.

### 5. Markdown Rendering

A native markdown engine (`ui/src/lib/markdown/`) preprocesses streaming content with `remend`, splits into incremental blocks, and renders via unified/remark/rehype. Markdown link safety uses a shell-aware link safety modal for URL opening behavior.

### 6. Desktop Updates

The desktop update settings live in the `AppUpdates` domain and now route through the shared desktop runtime facade. Tauri still uses Rust-side updater commands for `latest.json` and prerelease resolution, while Electron uses its main-process updater bridge.

Desktop-only renderer capabilities now flow through `ui/src/lib/desktop/`, with `ui/src/lib/shell.ts` acting as the runtime-neutral public facade for feature code. This keeps application components off direct `@tauri-apps/*` imports while preserving a browser fallback path plus dedicated Tauri and Electron adapters. The Electron shell now owns its own main-process bridge for window state persistence, tray behavior, sidecar boot, direct-to-Downloads file saves, and updater IPC, while the shared UI layer stays single-source. Shared window chrome now uses desktop-neutral drag-region names (`desktop-drag-region`, `desktop-no-drag`), while the main app header restores Tauri’s native drag-region overlay and the session toolbar also carries `data-tauri-drag-region` so macOS overlay titlebars keep native drag and double-click behavior.

A current audit of the Tauri-specific desktop surface area, plus a dual-runtime Electron port plan, lives in `docs/ELECTRON_PORT_PLAN.md`.

## Key Dependencies

| Package                    | Purpose                                                |
| -------------------------- | ------------------------------------------------------ |
| `svelte` / `@sveltejs/kit` | UI framework and SPA scaffold                          |
| `@sveltejs/adapter-static` | Static SPA output                                      |
| `@sentry/sveltekit`        | Browser error reporting and SvelteKit hook integration |
| `monaco-editor`            | Code editor                                            |
| `ghostty-web`              | Terminal emulator                                      |
| `@novnc/novnc`             | VNC display                                            |
| `@pierre/diffs`            | Diff rendering                                         |
| `bits-ui`                  | Headless UI primitives                                 |
| `@lucide/svelte`           | Icons                                                  |
| `tailwindcss`              | Styling                                                |
| `remend` / `unified`       | Markdown processing                                    |

## Module Documentation

- `docs/ui/design/` — UI module design docs

The app settings dialog also hosts project-scoped sandbox controls, including
resource management and inspection-shell access for the current project.
