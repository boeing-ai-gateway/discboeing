# UI Architecture

This document describes the architecture of the Discobot frontend, a SvelteKit application built with Vite that provides an IDE-like chat interface for AI coding agents.

## Overview

The UI is a single-page SvelteKit application. It renders an IDE-style interface with resizable panels for workspace navigation, chat/terminal, and file diffs.

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

All server data is fetched via `ui/src/lib/api-client.ts`. App-level project events, multiplexed chat streams, and service log streams now share one project-scoped WebSocket (`/api/projects/{projectId}/ws`). The app uses targeted reloads for affected sessions and workspaces, with full refreshes on reconnect. The store layer coalesces concurrent reloads into at most one active request plus one queued follow-up per resource key to avoid duplicate fetches during reconnect bursts.

### 3. API Configuration

`ui/src/lib/api-config.ts` discovers `/api/server-config` to prefer the server's HTTPS listener for browser traffic when trusted HTTPS is available, while Tauri continues using its direct local HTTP connection.

### 4. Chat Streaming

The thread chat stream reducer (`ui/src/lib/thread/conversation-stream.ts`) processes AI SDK-style data frames:

- Buffers `history-start`/`history-message`/`history-end` replay events
- Applies `chunk`/`done` events to materialize live AI messages
- Surfaces mode/model/reasoning metadata

Mounted thread workspaces subscribe through a single app-scoped project WebSocket, which multiplexes chat streams, project events, and service logs across multiple sessions and avoids holding separate SSE connections per mounted workspace or service panel. The shell only preloads the active session plus recent sessions the user has already opened, so sidebar recents can remain visible without eagerly creating session or thread contexts for untouched sessions.

Reasoning is level-based and sourced from each model's `reasoningLevels`/`defaultReasoning` metadata.

### 5. Markdown Rendering

A native markdown engine (`ui/src/lib/markdown/`) preprocesses streaming content with `remend`, splits into incremental blocks, and renders via unified/remark/rehype. Markdown link safety uses a Tauri-aware link safety modal for URL opening behavior.

### 6. Desktop Updates

The desktop update settings live in the `AppUpdates` domain and bridge to Rust Tauri commands instead of relying on the JavaScript updater binding alone. That Rust layer keeps the stable channel on the bundled `latest.json` endpoint and can switch to a GitHub pre-release channel by resolving the newest non-draft pre-release release asset named `latest.json` before building the updater request.

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
| `lucide-svelte`            | Icons                                                  |
| `tailwindcss`              | Styling                                                |
| `remend` / `unified`       | Markdown processing                                    |

## Module Documentation

- `docs/ui/design/` — UI module design docs
