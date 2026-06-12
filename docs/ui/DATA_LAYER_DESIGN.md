# UI Data Layer Design

## Goal

Rewrite the frontend data layer around two explicit state patterns:

1. Component-level state
2. Global application state

The target end state removes the current store structure, generic request caching,
request deduplication layer, reactive loading, and compatibility adapters. Backwards
compatibility with the existing frontend data layer is not a requirement. The end
state should be clean, direct, and easy to reason about.

The core rule is:

> Global state is an explicit, event-driven cache of backend resources. It is
> initialized through list-watch, changed by commands and backend events, and
> never loaded reactively.

The matching component rule is:

> If data is local to one component and does not need to survive or coordinate
> beyond that component, keep it local and let the component own its loading.

## State patterns

### Component-level state

Component-level state is local, scoped, and disposable. Components may use local
Svelte state and may call the API client directly when the data is only needed by
that component.

Component-local state is appropriate for:

- Dialogs, popovers, and one-off panels that fetch their own details.
- Form state and temporary UI input.
- Local loading/error state.
- Data that can be discarded when the component is destroyed.
- UI interactions that do not need to coordinate with sibling components.

Component-local code may call the API client directly:

```ts
const details = await api.someResource.get(id);
```

If the data needs to be shared across components, updated from backend events,
cached outside the component lifecycle, or coordinated with other app state, it
belongs in global state instead.

### Global state

Global state is the shared read model for backend/runtime resources. Components
read it directly from the context data structure. They do not use a separate
frontend query abstraction for global reads.

Global state is appropriate for:

- Project runtime state.
- Sessions.
- Threads.
- Messages and conversation runtime state.
- Files and file tree scopes.
- Hooks.
- Services.
- Models, credentials, workspaces, preferences, and other shared app data.

Global state changes only in response to:

1. Page load initialization.
2. User-driven commands.
3. Backend events.
4. Explicit watch recovery/reconnect flows.

Global state must not load data by observing other state. Do not add effects or
derived values that reactively fetch data.

Avoid patterns like:

```ts
$effect(() => {
	if (sessionId) {
		loadThreads(sessionId);
	}
});
```

Instead, loading is explicit, and the caller can choose whether to wait for the
activated data:

```ts
await commands.activateSession(sessionId, { wait: true });
```

This keeps data movement auditable: something happened, so a command ran or an
event was applied.

## Context rules

The context layer should be thin. It should expose a root state object and command
entry points. It should not contain hidden loading behavior.

Rules:

- Prefer one root `$state` object that wraps the whole context value.
- Avoid `.svelte.ts` files except where the root `$state` requires Svelte runes.
- Do not put `$effect` in context modules.
- Do not put derived values in context modules.
- Do not put API loading logic in context modules.
- Do not create wrapper stores, request caches, or compatibility adapters.
- Components read global data directly from the context state object.
- Components call commands for global writes, activations, and coordinated effects.

Conceptually:

```ts
const context = $state<AppContext>({
	data: createInitialDataState(),
	view: createInitialViewState(),
	commands: createCommands(),
});
```

The context object is the global read model. Commands are the write/effect
boundary.

## Commands and reads

### Commands

Commands are explicit actions invoked by components or page lifecycle code. They
may call the API client, activate scopes, start or recover list-watch flows, or
perform user-requested backend mutations.

Examples:

```ts
await commands.activateProject("local");
await commands.activateSession(sessionId, { wait: true });
await commands.activateThread(sessionId, threadId);
await commands.activateFileSubtree(sessionId, path);
await commands.sendMessage(sessionId, threadId, input);
await commands.stopSession(sessionId);
await commands.updateHook(sessionId, hookId, patch);
```

Commands are driven by:

- Page load.
- User events.
- Explicit user-requested refreshes.
- Explicit recovery/reconnect flows.

Backend events do not call the command layer. They are handled by the project
watch/event runtime, which may call API-backed data loading helpers such as
`loadSession`, `loadThread`, or `loadFileEntry`, then apply the result to the
appropriate cache under `context.data`.

The normal user mutation loop is:

```text
User event
  -> UI command
  -> API request
  -> backend state change
  -> backend event
  -> frontend event handler
  -> load/apply helper updates context.data
```

Optimistic updates should be rare and deliberate. The default should be
backend-confirmed state via events.

### Reads

There is no separate frontend query layer for global state.

Components read global state directly:

```ts
const context = useAppContext();
const session = context.data.sessions.byId[sessionId];
const thread = context.data.sessions.byId[sessionId]?.threads.byId[threadId];
```

For component-local data, components call the API client directly and keep the
result locally.

The split is:

```text
Global reads:
  read from context state

Global writes / activations:
  call commands

Component-local reads:
  call API client directly

Component-local state:
  keep inside the component
```

## Global cache location

Every global cache should be stored on an explicit field under `context.data`.
There should not be hidden module-level caches, standalone store instances, or
request-cache objects that hold backend resource data outside the context tree.

For example, the active read model should look conceptually like:

```ts
context.data.sessions.byId[sessionId];
context.data.sessions.allIds;
context.data.sessions.status;

context.data.sessions.byId[sessionId].threads.byId[threadId];
context.data.sessions.byId[sessionId].threads.status;
context.data.sessions.byId[sessionId].files.nodesByPath[path];
context.data.sessions.byId[sessionId].files.activeSubtrees[path];
context.data.sessions.byId[sessionId].files.statusBySubtree[path];
context.data.sessions.byId[sessionId].hooks.byId[hookId];
context.data.sessions.byId[sessionId].hooks.status;
context.data.sessions.byId[sessionId].services.byId[serviceId];
context.data.sessions.byId[sessionId].services.status;
```

Temporary caches used while rebuilding a list-watch scope are allowed, but they
are local working data. Once the list-watch flow is ready to publish, it swaps
that working cache into the corresponding `context.data` field.

This gives the app one visible source of truth for shared data:

```text
component-local state
or
context.data
```

## Loading and refresh status

Global caches need explicit status fields under `context.data` so components can
render loading, refreshing, stale, missing, and error states without guessing.
Status is part of the global read model; it should not live in hidden request
objects or module-level caches.

A representative status shape:

```ts
type ResourceStatus = {
	state: "idle" | "loading" | "ready" | "refreshing" | "missing" | "error";
	error?: string;
	lastLoadedAt?: number;
	refreshingSince?: number;
};
```

Collections and activated scopes should each have their own status:

- `context.data.sessions.status` for the project session list.
- `session.threads.status` for a session's thread collection.
- `session.files.statusBySubtree[path]` for each activated file subtree.
- `session.hooks.status` for hook collection state.
- `session.services.status` for service collection state.
- Thread/message state should have status for activated thread content.

Use `loading` when no cache has been activated yet. Use `refreshing` when stale
visible data exists and a new load is in progress. Use `missing` when activation
or refresh proves the resource does not exist. Components should assume an
activated resource may not exist yet and render the appropriate loading or
missing state.

Commands should set the relevant status immediately when they begin work. For
example, `activateSession(sessionId)` can create a placeholder session-scoped
entry with `threads.status.state = "loading"` before the thread list has loaded.
A refresh command with existing data should generally set `state = "refreshing"`
and keep stale data visible.

## Command completion options

Commands should support both non-blocking and blocking usage. The caller decides
whether it needs to wait for the intended cache update.

A representative option shape:

```ts
type CommandOptions = {
	wait?: boolean;
};
```

Default command behavior should be non-blocking where that keeps the UI
responsive: start the API request or activation flow, update status in
`context.data`, and return once the work has been scheduled or the initiating API
call has completed.

When `wait: true` is provided, the command should resolve only after its intended
read-model update has been applied to `context.data`, or reject if that update
fails. This is useful when the component needs to sequence follow-up behavior
against the loaded state.

Examples:

```ts
// Navigate immediately; the destination renders from status fields.
void commands.activateSession(sessionId);

// Open a dependent UI only after the session cache is ready or known missing.
await commands.activateSession(sessionId, { wait: true });
```

Blocking does not mean the command bypasses backend events. If the intended
update normally arrives through the project event stream, `wait: true` should wait
for that event-driven load/apply path to update `context.data`. If the event does
not arrive within the expected flow, the command may use an explicit load helper
or fail with a useful error, depending on the operation.

Commands should not require blocking for normal rendering. Components should be
able to read status from `context.data` and show loading, refreshing, missing, or
error states.

## List-watch pattern

Global resource collections should follow a list-watch pattern similar to
Kubernetes informers.

For any activated global scope:

1. Start watching changes and queue incoming events in memory.
2. List the current resources from the API.
3. Build a fresh cache separate from the active cache.
4. Replay queued events onto the fresh cache.
5. Swap the fresh cache into the active global state.
6. Apply future events directly to the active cache.
7. If the watch errors or reconnects, repeat the process.

During reconnect or recovery, keep the stale active cache visible. Build the new
cache on the side, replay pending deltas, and then swap. Do not clear visible
data just because the watch is reconnecting.

If an event includes the full resource, apply it directly. If the event only
includes an ID or path, issue a `GET` for that resource and then update the cache.

## Project event stream

For now, the UI assumes the project is named `local`.

On page load, the app should subscribe to the project-level event stream for
`local`. That single stream is the source of backend events for the app.

Avoid the current complex model of:

- Base channels.
- Sub-channels.
- Multiplexed frontend subscriptions.
- Nested event stream abstractions.

Use one straightforward project event handler that inspects each event and calls
the corresponding data loading/apply helper. The event runtime is not the command
layer; commands are for component and page lifecycle actions. In `$lib/context`, the
list-watch orchestration lives in `domains/project.ts`, while low-level WebSocket
subscribe/reconnect handling lives in `project-subscription.ts`. This runtime
opens the project WebSocket directly and must not reuse the legacy `chatStreams`
abstraction.

Conceptually:

```ts
function handleProjectEvent(event: ProjectEvent) {
	switch (event.type) {
		case "session.changed":
			void loadSessionIntoCache(context, api, event.sessionId);
			break;
		case "thread.changed":
			void loadThreadIntoCache(context, api, event.sessionId, event.threadId);
			break;
		case "file.changed":
			void loadFileEntryIntoCache(context, api, event.sessionId, event.path);
			break;
	}
}
```

The exact event names can follow the backend API, but the shape should stay
simple: one project stream, one handler, and explicit load/apply functions that
update `context.data`.

## Activation model

Activation means the user or page lifecycle has expressed interest in a scope, so
the app should list that scope, maintain a cache for it, and apply future events
to it.

Activation is hierarchical.

### Project activation

On page load:

- Subscribe to the project event stream for `local`.
- Queue incoming events while the initial list is loading.
- List all sessions.
- Build the session cache.
- Replay queued session events.
- Activate the session dataset.
- Continue updating sessions from project events.

The app should maintain a cache of all sessions because session events can arrive
at any time and the session list is app-global navigation state.

### Session activation

When the user opens a session:

```ts
await commands.activateSession(sessionId);
```

The app should:

- Ensure the session has a placeholder cache entry if it is not already present.
- Mark that session's activated collections as `loading` or `refreshing` as
  appropriate.
- List threads.
- List root-level files.
- List hooks.
- List services if applicable.
- Build a session-scoped cache.
- Replay queued events relevant to that session.
- Swap the fresh session cache into active state.
- Mark the session or its collections as `missing` if the backend reports that
  the session no longer exists.

### Thread activation

When the user opens a thread:

```ts
await commands.activateThread(sessionId, threadId);
```

The app should:

- Mark the thread content status as `loading` or `refreshing` as appropriate.
- Load the conversation/messages for that thread.
- Build or refresh the active thread cache.
- Apply queued thread/message events.
- Continue applying future events from the project stream.

### File subtree activation

Files follow the same activation and caching pattern as other global resources,
but the resource itself is hierarchical.

Do not eagerly load the whole file tree. On session activation, load only the
root level. As the user expands directories or otherwise asks to see nested
content, activate that subtree:

```ts
await commands.activateFileSubtree(sessionId, path);
```

The app should:

- Fetch that directory's children.
- Mark that subtree as active.
- Mark that subtree status as `loading` or `refreshing` as appropriate.
- Cache the loaded subtree in global state.
- Apply future file events to active portions of the tree.
- Avoid fetching unseen subtrees reactively.

The important distinction is not that files use a different model. They use the
same model recursively: each visible or user-selected subtree is an explicitly
activated scope.

## Target file shape

The exact module names can evolve, but the final data layer should have this
shape:

```text
ui/src/lib/context/
  context.svelte.ts        # root $state creation and set/use context helpers only
  context.types.ts         # root data/view/command types
  initial-state.ts         # pure initial data/view factories
  commands.ts              # thin command registry; delegates to domains/*
  domains/project.ts      # project-level websocket/list-watch lifecycle
  project-events.ts        # event decoding and dispatch to domain load/apply helpers
  cache.ts                 # shared cache/status helpers

  domains/
    sessions.ts            # session state, loading, activation, mutations
    threads.ts             # thread/message state, loading, commands, mutations
    files.ts               # hierarchical file state, loading, commands, mutations
    hooks.ts               # hook state, loading, commands, mutations
    services.ts            # service state, loading, commands, mutations
    workspaces.ts          # workspace state, loading, commands, mutations
    credentials.ts         # credential/model state, loading, commands, mutations

ui/src/lib/api-client.ts    # canonical API client for commands and component-local reads
```

Guidelines for these files:

- `context.svelte.ts` should be the only `.svelte.ts` file required for global
  state, because it owns the root `$state`.
- `context.svelte.ts` should not import the API client.
- `context.svelte.ts` should not contain effects, derived values, watches, or
  resource-loading logic.
- `domains/project.ts` owns the single project event stream lifecycle.
- `project-events.ts` maps backend events to explicit domain load/apply helpers.
- `commands.ts` should stay small. It creates the public command API with a
  type-safe object registration helper, binds `context` once, handles optional
  command scheduling, and delegates each command to the appropriate `domains/*`
  function. The registration map should be checked with `satisfies` so its keys
  and function signatures match the public command type.
- `domains/*` consolidate all logic for that resource type: state shape, status
  transitions, API-backed loading, event application, command implementations,
  and pure cache mutations.
- Components should not import `cache.ts` or domain helpers directly. Components
  read context and call commands.

Components should not import domain functions directly. `commands.ts` should wire
them through the registration helper:

```ts
const commands = {
	...register(context, {
		activateProject,
		activateSession,
		refreshSessions: loadSessionsIntoCache,
		refreshThread: loadThreadIntoCache,
		startService,
	} satisfies CommandRegistrationSpec<Omit<AppCommands, "shutdown">>),
	shutdown: () => {},
} satisfies AppCommands;
```

This keeps the public command object readable while still checking that every
registered property matches the declared command API.

## Target API surface

The command API should be explicit and resource-oriented. A representative shape:

```ts
interface AppCommands {
	startup(): Promise<void>;
	shutdown(): void;

	activateProject(projectId: string, options?: CommandOptions): Promise<void>;
	activateSession(sessionId: string, options?: CommandOptions): Promise<void>;
	activateThread(sessionId: string, threadId: string, options?: CommandOptions): Promise<void>;
	activateFileSubtree(sessionId: string, path: string, options?: CommandOptions): Promise<void>;

	refreshSessions(options?: CommandOptions): Promise<void>;
	refreshSession(sessionId: string, options?: CommandOptions): Promise<void>;
	refreshThread(sessionId: string, threadId: string, options?: CommandOptions): Promise<void>;
	refreshFileSubtree(sessionId: string, path: string, options?: CommandOptions): Promise<void>;
	refreshHooks(sessionId: string, options?: CommandOptions): Promise<void>;
	refreshServices(sessionId: string, options?: CommandOptions): Promise<void>;

	createSession(input: CreateSessionInput): Promise<void>;
	renameSession(sessionId: string, name: string): Promise<void>;
	stopSession(sessionId: string): Promise<void>;
	deleteSession(sessionId: string): Promise<void>;

	createThread(sessionId: string, input: CreateThreadInput): Promise<void>;
	renameThread(sessionId: string, threadId: string, name: string): Promise<void>;
	deleteThread(sessionId: string, threadId: string): Promise<void>;

	sendMessage(sessionId: string, threadId: string, input: SendMessageInput): Promise<void>;
	cancelRun(sessionId: string, threadId: string): Promise<void>;

	openFile(sessionId: string, path: string): Promise<void>;
	saveFile(sessionId: string, path: string, content: string): Promise<void>;
	renameFile(sessionId: string, from: string, to: string): Promise<void>;
	deleteFile(sessionId: string, path: string): Promise<void>;

	updateHook(sessionId: string, hookId: string, patch: HookPatch): Promise<void>;
	rerunHook(sessionId: string, hookId: string): Promise<void>;
	pauseHook(sessionId: string, hookId: string, paused: boolean): Promise<void>;

	startService(sessionId: string, serviceId: string): Promise<void>;
	stopService(sessionId: string, serviceId: string): Promise<void>;
}
```

This is illustrative, not a final contract. The important API qualities are:

- Commands are named after user/page lifecycle actions.
- Commands are explicit about scope IDs.
- Commands do not hide reactive dependencies.
- Refresh commands are component-invoked force refreshes, not backend event
  handlers.
- Backend event handlers use `loadXIntoCache`/`applyX` helpers instead of calling
  commands.
- Commands can optionally block until the intended `context.data` update has
  been applied.
- Commands can be tested by asserting API calls, status transitions, and state
  mutations.
- Reads do not require command calls unless they are activating a new scope or
  forcing a refresh.

## Migration plan

Build the new data layer in parallel first, then migrate components after the
shape is proven. Avoid breaking the current UI while the new model is still being
designed.

The temporary implementation should live under a net-new package:

```text
ui/src/lib/context/
```

`ng` is a staging namespace for the next-generation data layer. It should not
import from the old store/resource/domain layers except for stable API types and
the canonical API client. Once the design is validated, we can move the contents
into their final `$lib` locations and then update components.

### Phase 1: Freeze the target design

- Use this document as the source of truth.
- Update UI architecture docs to point here for data-layer rules.
- Stop adding new store/resource-cache abstractions.
- Stop adding new reactive global loading.
- Agree on the `$lib/context` package boundary and naming before implementation.

### Phase 2: Create the `$lib/context` package skeleton

Create the new data layer without wiring it into existing app components:

```text
ui/src/lib/context/
  context.svelte.ts        # root $state creation and set/use helpers for ng only
  context.types.ts         # ng root data/view/command types
  initial-state.ts         # pure initial data/view factories
  commands.ts              # thin command registry; delegates to domains/*
  domains/project.ts      # project-level websocket/list-watch lifecycle
  project-events.ts        # event decoding and dispatch to domain load/apply helpers
  cache.ts                 # shared cache/status helpers
  domains/
    sessions.ts
    threads.ts
    files.ts
    hooks.ts
    services.ts
    workspaces.ts
    credentials.ts
```

The package should compile independently and be testable without replacing the
current UI data layer.

### Phase 3: Design the state shape in `$lib/context`

- Define the root `context.data` and `context.view` shape.
- Define status fields for every global collection and activated scope.
- Define command options, including optional `{ wait: true }` behavior.
- Define cache mutation helpers for sessions, threads, files, hooks, and
  services.
- Keep all global caches under explicit `context.data` fields.
- Avoid compatibility shims to old state shapes.

At this phase, we should be able to review the type shape and decide whether it
matches the desired model before any component migration begins.

### Phase 4: Implement list-watch and event handling in `$lib/context`

- Subscribe to the `local` project event stream.
- Queue events while initial listing is in progress.
- List sessions.
- Build a fresh session cache.
- Replay queued events.
- Swap the cache into `context.data.sessions`.
- Implement reconnect recovery using the same list-watch flow.
- Route backend events to `loadXIntoCache`/`applyX` helpers, not commands.

This phase can be validated with unit tests and small harnesses before app
components consume it.

### Phase 5: Implement activation and refresh commands in `$lib/context`

- Implement `activateSession`, `activateThread`, and
  `activateFileSubtree` from scratch.
- Implement `refreshSessions`, `refreshSession`, `refreshThread`,
  `refreshFileSubtree`, `refreshHooks`, and `refreshServices`.
- Set status immediately when commands begin work.
- Support both non-blocking and `{ wait: true }` behavior.
- Treat missing resources as first-class status, not exceptional UI crashes.
- Keep stale visible data active during refresh/reconnect.

### Phase 6: Implement domain coverage in `$lib/context`

- Sessions and project navigation data.
- Session activation data: threads, root files, hooks, services.
- Thread activation data: messages/conversation state.
- File subtree activation and file mutation commands.
- Hook event loading and commands.
- Service event loading and commands.
- Remaining shared app data such as workspaces, models, credentials,
  preferences, environment, update state, and startup state.

This phase should produce a complete new data-layer backend that is usable by
components, but still isolated from the existing UI.

### Phase 7: Validate the `$lib/context` design

Before migrating components, verify that the new package is the shape we want:

- The public command API feels right.
- The `context.data` layout is easy for components to read.
- Loading, refreshing, missing, and error states are represented explicitly.
- Blocking command behavior is testable and understandable.
- Backend events update caches without calling commands.
- No reactive global loading exists.
- No hidden request caches or module-level backend-data stores exist.

If the shape is wrong, change `$lib/context` while it is still isolated.

### Phase 8: Promote `$lib/context` to final locations

Once the design is approved, replace the old context package with the new
implementation. Conceptually:

```bash
rm -rf ui/src/lib/context
mv ui/src/lib/context ui/src/lib/context
```

The exact move should be done carefully to avoid removing unrelated files before
the component migration is ready, but the intent is that `$lib/context` is temporary
and eventually becomes `$lib/context`. The final code should not keep an `ng`
namespace.

### Phase 9: Migrate components to the new data layer

Update components in focused vertical slices:

1. App/page startup and root context provider.
2. Session list and navigation.
3. Session workspace activation.
4. Thread workspace activation and conversation rendering.
5. Files panel and file subtree activation.
6. Hooks panel.
7. Services panel.
8. Workspaces, credentials, models, preferences, updates, and remaining shared
   app state.

As each slice moves, components should read global state from context and call
commands for activations, refreshes, and mutations. Component-local data can keep
calling the API client directly.

### Phase 10: Delete legacy completely

After all components are migrated, delete all remaining modules that provide:

- Generic request caching.
- Store/resource abstractions.
- Reactive global loading.
- Nested frontend event subscription channels.
- Compatibility adapters.
- Old context/domain shapes that are no longer used.

Do not keep unused compatibility types for future safety. The target is a single
clean data layer.

### Phase 11: Add enforcement tests

Add source-level tests that fail if the old patterns return. Useful checks:

- No imports from deleted store/resource-cache paths.
- No `$effect` in global context/data modules.
- No `derived` usage in global context/data modules.
- `context.svelte.ts` does not import the API client.
- App components do not import cache mutation helpers directly.
- Global loading APIs are only exposed through commands or event loaders.
- Backend event handlers do not call component-facing commands.

The command inventory and old-to-new mapping register lives in
`docs/ui/DATA_LAYER_COMMAND_MAPPING.md`. Use it to expand `NgCommands`
deliberately instead of accidentally dropping behavior during migration.

## Review checklist

When reviewing new UI data-layer code, ask:

- Is this component-local state or global state?
- If global, what command or backend event changes it?
- Is any global data loaded reactively? If yes, reject it.
- Is the context layer only exposing state and commands? If not, simplify it.
- Does this introduce a second cache of backend data? If yes, remove it.
- Does this data need to survive the component lifecycle? If not, keep it local.
- Is this list-watch flow preserving stale visible data during reconnect? If not,
  fix it.
- Are files handled as activated hierarchical scopes rather than eager full-tree
  loads? If not, fix it.
