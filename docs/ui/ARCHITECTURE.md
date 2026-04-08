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
├── routes/
│   └── +layout.svelte         # AppContext provider, SSE subscription
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
│       └── runtime/           # Chat stream reducer, SSE state
└── static/                    # Favicon and static assets
```

## Component Architecture

### Component Folders

Components live under `ui/src/lib/components/` in three folders:

| Folder | Role | Context |
|--------|------|---------|
| `ui/` | Pure primitives — buttons, inputs, dialogs, etc. | None |
| `ai/` | Self-contained compound components | Component-local only |
| `app/` | App shell — session UI, composer, panels | Global app/session/thread contexts |

### Global Context System

Three contexts flow top-down, each set by a single provider:

| Context | Provider | Provides |
|---------|----------|---------|
| `AppContext` | `routes/+layout.svelte` | Sessions, workspaces, models, credentials, preferences |
| `SessionContext` | `app/SessionWorkspace.svelte` | Threads, files, hooks, services, session credentials |
| `ThreadContext` | `app/ThreadWorkspace.svelte` | Conversation, messages, plan entries |

Access via `useAppContext()`, `useSessionContext()`, `useThreadContext()` from `$lib/context/`.

## Key Architectural Decisions

### 1. Single-Page Application with SvelteKit

The application uses SvelteKit's adapter-static for a fully client-side SPA. Panel content is driven by Svelte stores and context rather than URL routes.

### 2. Data Fetching and SSE

All server data is fetched via `ui/src/lib/api-client.ts`. The app-level SSE subscription (`/api/projects/local/events`) triggers targeted reloads for affected sessions and workspaces, with full refreshes on reconnect. The store layer coalesces concurrent reloads into at most one active request plus one queued follow-up per resource key to avoid duplicate fetches during reconnect bursts.

### 3. API Configuration

`ui/src/lib/api-config.ts` discovers `/api/server-config` to prefer the server's HTTPS listener for browser traffic when trusted HTTPS is available, while Tauri continues using its direct local HTTP connection.

### 4. Chat Streaming

The thread chat stream reducer (`ui/src/lib/session/runtime/chat-stream-state.ts`) processes SSE events:
- Buffers `history-start`/`history-message`/`history-end` replay events
- Applies `chunk`/`done` events to materialize live AI messages
- Surfaces mode/model/reasoning metadata

Reasoning is level-based and sourced from each model's `reasoningLevels`/`defaultReasoning` metadata.

### 5. Markdown Rendering

A native markdown engine (`ui/src/lib/markdown/`) preprocesses streaming content with `remend`, splits into incremental blocks, and renders via unified/remark/rehype. Markdown link safety uses a Tauri-aware link safety modal for URL opening behavior.

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `svelte` / `@sveltejs/kit` | UI framework and SPA scaffold |
| `@sveltejs/adapter-static` | Static SPA output |
| `monaco-editor` | Code editor |
| `ghostty-web` | Terminal emulator |
| `@novnc/novnc` | VNC display |
| `@pierre/diffs` | Diff rendering |
| `bits-ui` | Headless UI primitives |
| `lucide-svelte` | Icons |
| `tailwindcss` | Styling |
| `remend` / `unified` | Markdown processing |

## Module Documentation

- `docs/ui/design/` — UI module design docs
