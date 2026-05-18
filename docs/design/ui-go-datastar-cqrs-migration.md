# Svelte to templ/Datastar CQRS Migration Plan

This document records the migration plan for replacing the current Svelte UI with
server-rendered templ components enhanced by Datastar. The goal is not to port the
Svelte component tree one component at a time. The goal is to move Discobot to a
server-driven hypermedia UI where the Go backend owns authoritative state,
commands mutate that state, and Datastar applies server-rendered HTML patches.

## Core Datastar model

The migration should be designed around Datastar's rendering model:

1. The browser makes one normal request to render the page entry point, usually
   `GET /`.
2. The server returns a complete templ-rendered HTML document for the current
   read-model snapshot.
3. Datastar does not hydrate a client-side component tree and should not fetch
   JSON just so browser code can rebuild UI state.
4. After the initial render, the page is kept current with server-sent Datastar
   events that patch rendered HTML fragments and update small client signals.
5. Commands are short-lived requests that mutate state. They can return immediate
   Datastar patches for validation/optimistic UI, but authoritative domain
   updates should flow from the read model and its event stream.

In practical terms, the preferred shape is one full-page render and one primary
app read stream for the active page. Additional streams should be used only for
expensive or specialized live regions, such as a service log, terminal, desktop
viewer, or editor island. Avoid recreating an SPA made of many JSON reads and
client-side stores.

## CQRS framing

### Query/read side

Read models produce templ components or full pages. They should be shaped around
what the UI needs to render, not around the existing REST response shapes.

Primary read models:

- App shell: current user, auth state, startup tasks, models, preferences,
  selected workspace/session/thread, and global view state.
- Sidebar: workspaces, recent sessions/threads, session status summaries, and
  startup banners.
- Session workspace: session status, selected thread, dock state, file summary,
  hooks status, services status, and embedded service metadata.
- Thread conversation: thread metadata, messages, queued prompts, pending
  questions, approval state, active model/reasoning, and running state.
- Files/diffs: file tree, selected file content, diff file list, and selected
  diff content.
- Services/hooks: service list, service output, hook status, hook output, and
  rerun/start/stop affordances.
- Settings: credentials, credential types, OAuth/device-flow state, providers,
  sandbox resources, preferences, support info, and system status.

These read models should be rendered server-side with templ. Datastar patches
replace stable DOM islands instead of mutating browser-side stores.

### Command side

Commands mutate domain state. They should generally be `POST`, `PUT`, `PATCH`, or
`DELETE` handlers invoked by Datastar forms/actions.

Command groups:

- Workspaces: create, update, delete, validate.
- Sessions: create, select, update display name/settings, stop, delete.
- Threads: create, select, rename, delete, update/delete queued prompts.
- Chat: submit prompt, cancel run, answer approval/question.
- Files: write, delete, rename.
- Services/hooks: start service, stop service, rerun hook.
- Credentials/settings: create, delete, refresh, OAuth/device-flow steps, update
  preferences, update project resources, clear cache.

Command handlers may return Datastar patches for local form errors, disabled
states, or immediate UI response. The durable UI state should still come from the
read model stream after the backend commits the mutation.

### Stream side

Use a primary Datastar SSE stream for page-level read-model updates. The stream
should listen to the existing project event sources and render patches for the DOM
islands affected by each event.

Recommended default:

```text
GET /                         full page render
GET /ui/stream                primary Datastar read stream for the page
POST /ui/commands/...         mutations
GET /assets/...               static assets
```

The primary stream can patch:

- `#app-sidebar`
- `#startup-banner`
- `#session-{sessionID}`
- `#thread-list-{sessionID}`
- `#conversation-{threadID}`
- `#composer-{threadID}`
- `#dock-{sessionID}`
- `#file-tree-{sessionID}`
- `#services-{sessionID}`
- `#hooks-{sessionID}`

Additional streams are acceptable for specialized high-volume regions:

- `GET /ui/sessions/{sessionID}/services/{serviceID}/output`
- terminal output streams
- desktop/viewer/editor bridge streams
- very large diff or file-tail streams

These should be exceptions, not the default application data model.

## `ui-go` target structure

Suggested package layout:

```text
ui-go/
  cmd/ui-go/
    main.go

  internal/backend/
    client.go          # talks to existing REST/WS backend during migration

  internal/query/
    app.go
    sessions.go
    threads.go
    files.go
    services.go
    settings.go

  internal/command/
    sessions.go
    threads.go
    chat.go
    files.go
    services.go
    settings.go

  internal/stream/
    app.go
    render.go

  content/
    layout/
      root.templ
      shell.templ
    app/
      sidebar.templ
      header.templ
      startup_banner.templ
    session/
      workspace.templ
      thread_workspace.templ
      dock.templ
    thread/
      conversation.templ
      message.templ
      composer.templ
    files/
      tree.templ
      diff.templ
      viewer.templ
    services/
      list.templ
      output.templ
    settings/
      dialog.templ

  static/
    app.css
    datastar.js
```

During the transition, `ui-go/internal/backend` can call the existing REST and
WebSocket APIs. Once the Go UI is embedded into the main server, those adapters
can be replaced with direct service/query calls.

## View state rules

Server-authoritative domain state:

- sessions
- workspaces
- threads
- messages
- queued prompts
- files
- diffs
- hooks
- services
- credentials
- providers
- preferences that should persist across reloads

Datastar/client signal state:

- selected session ID for the current browser view
- selected thread ID for the current browser view
- sidebar open/collapsed
- active dock panel
- selected file path
- modal/dialog open state
- composer draft
- form-local validation/display state

Persist client-view state only when there is a product reason to restore it after
reload. Otherwise keep it as Datastar signals.

## Migration phases

### Phase 0: Foundation

- Add Datastar to `ui-go` static assets or vendored asset pipeline.
- Add `static/app.css`, initially porting shared design tokens from the Svelte UI.
- Keep `GET /` as the single full-page templ render.
- Add a primary `GET /ui/stream` Datastar SSE endpoint.
- Add `/ui/commands` route groups for mutations.
- Add a small backend adapter around current Discobot REST/WS APIs.
- Keep the Svelte UI as the production UI while this matures.

Exit criteria: `ui-go` renders a full-page shell and opens a Datastar stream.

### Phase 1: Read-only shell/sidebar

- Render the app shell, header, sidebar, session list, workspace list, and startup
  banner from server-side read models.
- Patch sidebar/session/workspace/startup regions from the primary stream.
- Do not introduce browser-side entity stores.

Exit criteria: `ui-go` can show live workspace/session state without Svelte.

### Phase 2: Navigation and simple commands

- Add Datastar commands for selecting sessions/threads and creating/stopping basic
  sessions.
- Patch the main workspace placeholder and sidebar from command responses or the
  primary stream.
- Keep navigation state in Datastar signals unless it must survive reload.

Exit criteria: users can navigate the shell and perform basic session/workspace
actions.

### Phase 3: Thread workspace read model

- Render thread list, conversation transcript, queued prompts, and pending
  question/approval UI with templ.
- Convert current chat stream state into a server-side read model that can render
  `#conversation-{threadID}` patches.
- Prefer server-side materialized conversation state over browser-side stream
  reducers.

Exit criteria: users can open a session/thread and see a live conversation.

### Phase 4: Chat commands and streaming

- Add commands for prompt submit, cancel, and approval/question answers.
- Have chat execution update the authoritative read model.
- Use the primary stream to patch conversation, composer state, pending questions,
  and thread/session titles.

Exit criteria: the core Discobot loop works in `ui-go`: submit, stream, answer,
and cancel.

### Phase 5: Files, diffs, hooks, and services

- Port file tree, file viewer, diff viewer, hook status/output, service list, and
  service output panels.
- Use the primary stream for normal status/list patches.
- Use dedicated streams only for high-volume service logs or terminal-like output.

Exit criteria: users can inspect workspace changes and service/hook status.

### Phase 6: Settings, credentials, providers, resources

- Port settings dialogs and forms.
- Use Datastar form submissions for validation, OAuth/device-flow progression,
  preferences, sandbox resources, and credential management.

Exit criteria: main app configuration no longer depends on Svelte.

### Phase 7: High-JS islands

Do not force specialized browser widgets into pure Datastar:

- terminal emulator
- Monaco/editor-like file editing
- noVNC desktop viewer
- embedded VS Code/service iframes
- markdown/code-block enhancements where client behavior is necessary

Wrap these as small JS islands. Datastar should own shell lifecycle and server
patches; the island should own only widget internals.

Exit criteria: rich panels work without rebuilding the whole app as an SPA.

### Phase 8: Cutover

- Run both UIs behind a flag or separate route during validation.
- Verify core flows: create session, submit prompt, answer approval, inspect diff,
  run service/hook, and manage credentials/settings.
- Make `ui-go` the default UI once parity is sufficient.
- Remove Svelte stores/components gradually after routes are no longer used.

## First implementation milestone

Build a minimal vertical slice:

1. `GET /` renders the full Discobot shell with Datastar loaded.
2. `GET /ui/stream` opens one primary Datastar read stream.
3. The stream patches sidebar/session/workspace regions on project events.
4. `POST /ui/commands/sessions` creates a session.
5. Selecting a session updates Datastar signals and patches the main workspace
   placeholder.

This proves the intended Datastar/CQRS shape before porting chat streaming, which
is the hardest part of the migration.
