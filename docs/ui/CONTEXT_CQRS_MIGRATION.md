# Context CQRS Migration

## Goal

Move from the current nested `AppContext` → `SessionContext` → `ThreadContext`
hierarchy to one app-wide, deeply reactive Svelte `$state()` object:

```ts
context.view;
context.data;
```

Commands form the write boundary. Components should read state directly from the
single context and call command functions for behavior that involves effects,
backend APIs, persistence, streaming, or coordinated state changes.

## Boundary rules

- `context.data` contains backend/runtime/domain data only.
- `context.view` contains frontend, UI, selection, editor, form, dialog, and
  locally persisted preference state.
- Components may directly assign simple view fields.
- Components should call command functions for backend calls or multi-field
  behavior.
- Commands look up the current root context and mutate `context.view` /
  `context.data` directly.
- Avoid getters, setters, wrapper stores, and nested Svelte context providers in
  the new model.

## Current state files

- Root and data types: `ui/src/lib/context/context.types.ts`
- View types: `ui/src/lib/context/context-view.types.ts`

## Migration phases

| Phase | Area                                                  | Status      | Notes                                                                                                                         |
| ----- | ----------------------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------------------------------------- |
| 0     | Type design                                           | Done        | Root/view/data type files exist.                                                                                              |
| 1     | Root CQRS context scaffold                            | Done        | Root `$state()` context and command lookup boundary added.                                                                    |
| 2     | App shell/root layout                                 | Done        | `routes/+layout.svelte` assigns the new context and initializes the root command runtime without the old app-state bridge.     |
| 3     | App-level view state                                  | Done        | Sidebar/dialog, preference, update, startup, and recent/mounted navigation projections moved to root context/commands.         |
| 4     | Session selection/navigation                          | In progress | Root selection/mounted-session state is managed by command runtime; session/thread navigation actions now use root commands.   |
| 5     | Session workspace provider removal                    | Done        | `SessionWorkspace` no longer sets the old session context or bridge.                                                          |
| 6     | Thread workspace provider removal                     | Done        | `ThreadWorkspace` no longer sets the old thread context or bridge.                                                            |
| 7     | Conversation data                                     | Pending     | Port messages, browser events, streaming, pending question, prompt queue.                                                     |
| 8     | Composer state                                        | Pending     | Port drafts, model/reasoning/service tier overrides, pending comments.                                                        |
| 9     | Files domain                                          | Pending     | Split backend file data from editor/view state and port file commands.                                                        |
| 10    | Hooks domain                                          | Pending     | Port hook status/output and hook commands.                                                                                    |
| 11    | Services domain                                       | Pending     | Port service data and start/stop/bind commands.                                                                               |
| 12    | Commands domain                                       | Pending     | Port agent command list and credential dialog view state.                                                                     |
| 13    | Workspaces/models/credentials/startup/support/updates | In progress | Global backend/runtime refresh and update commands are root-context backed; deeper domain cleanup remains.                     |
| 14    | Remove old contexts                                   | Done        | Deleted old app/session/thread provider files and the temporary legacy bridge.                                                |

## Component migration checklist

A component is **Done** only when it no longer imports or calls any old context
entry points, including `useAppContext()`, `useSessionContext()`,
`useThreadContext()`, `setAppContext()`, `setSessionContext()`,
`setThreadContext()`, `getAppContextIfPresent()`, or similar old context
helpers. Components that read the new root context or call commands but still
reference old contexts remain **Partial**. **First-pass** means the component no
longer imports or calls old context entry points directly, but may still use the
temporary bridge while its data/view/command split is completed.

| Component / module                              | Reads new context | Uses commands | Old context removed | Status                                                                                                                                       |
| ----------------------------------------------- | ----------------- | ------------- | ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `routes/+layout.svelte`                         | Yes               | N/A           | Yes                 | New context initialized without the old app context provider.                                                                                 |
| `AppHeader.svelte`                              | Yes               | Partial       | First-pass          | Second-pass navigation complete; header environment/update/preference projections read root data/view and settings/new-session use commands. |
| `AppKeyboardShortcuts.svelte`                   | Yes               | Partial       | Yes                 | Switcher projections read root data/view and navigation/dialog/view-toggle behavior uses commands; no direct app-state accessor.             |
| `AppMacWindowSpacer.svelte`                     | Yes               | N/A           | Yes                 | Done; reads native window control environment from `context.data.environment`.                                                               |
| `AppSessionStatus.svelte`                       | Yes               | Pending       | Yes                 | Old context entry points and temporary bridge removed.                                                                                        |
| `AppShell.svelte`                               | Yes               | Partial       | First-pass          | Second-pass navigation complete; mounted sessions, sidebar state, selection, and startup banner projections read root data/view.             |
| `AppSidebar.svelte`                             | Yes               | Partial       | Yes                 | Session/recent/workspace/thread projections, sidebar preferences, and sidebar mutations use root context/commands; no direct app-state accessor. |
| `AppThreadStatus.svelte`                        | Yes               | Pending       | Yes                 | Old context entry points and temporary bridge removed.                                                                                        |
| `ConversationComposer.svelte`                   | Yes               | Partial       | Yes                 | Reads app models/preferences from root context and calls root commands for app dialogs/history; session/thread are explicit props.           |
| `ConversationComposerSessionSetupStatus.svelte` | Yes               | Pending       | Yes                 | Old context entry points and temporary bridge removed; session/thread are explicit props.                                                     |
| `ConversationCredentialsControl.svelte`         | Yes               | Partial       | Yes                 | Reads credential types from root data and calls root commands for credential management; session is an explicit prop.                        |
| `ConversationHooksPanel.svelte`                 | Yes               | Pending       | Yes                 | Old context entry points and temporary bridge removed; session is an explicit prop.                                                           |
| `ConversationPane.svelte`                       | Yes               | Partial       | Yes                 | Reads selected session/chat preferences from root context; session/thread are explicit props.                                                |
| `ConversationWorkspaceSelector.svelte`          | Yes               | Partial       | Yes                 | Reads workspace data from root context and calls workspace validation/refresh commands; session is an explicit prop.                         |
| `CredentialsManager.svelte`                     | Yes               | Partial       | Yes                 | Reads credential data/dialog flow from root context and uses API/credential commands; no direct app-state accessor.                          |
| `DockPanel.svelte`                              | Yes               | Partial       | Yes                 | Reads theme/sidebar view state from root context and uses command-backed service output subscription; session/thread are explicit props.     |
| `SandboxProvidersManager.svelte`                | Yes               | Partial       | Yes                 | Reads credential data from root context and refreshes credentials through commands.                                                          |
| `SessionToolbar.svelte`                         | Yes               | Partial       | Yes                 | Reads IDE preferences from root context and uses command-backed session/preference access.                                                   |
| `SessionToolbarStack.svelte`                    | Yes               | Partial       | Yes                 | Reads mounted session toolbar projections from root context.                                                                                 |
| `SessionWorkspace.svelte`                       | Yes               | Partial       | Yes                 | Uses command-backed session ensure/release; no direct app-state accessor.                                                                    |
| `SettingsDialog.svelte`                         | Yes               | Partial       | Yes                 | Reads models/environment/preferences/update data from root context and mutates via commands.                                                 |
| `SupportInfoDialog.svelte`                      | Yes               | Partial       | Yes                 | Reads support info from root data and fetches/closes through root commands.                                                                  |
| `ThreadWorkspace.svelte`                        | Yes               | Pending       | Yes                 | Old context entry points and temporary bridge removed.                                                                                        |
| `ThreadWorkspaceActive.svelte`                  | Yes               | Pending       | Yes                 | Old context entry points and temporary bridge removed; session/thread are explicit props.                                                     |
| Old context definitions                         | N/A               | N/A           | Yes                 | Deleted `app-context.svelte.ts`, `session-context.svelte.ts`, `thread-context.svelte.ts`, and `legacy-context-bridge.svelte.ts`.             |

## Command migration checklist

| Command area                             | Status      | Notes                                                                                                                |
| ---------------------------------------- | ----------- | -------------------------------------------------------------------------------------------------------------------- |
| App startup/refresh                      | Done        | `StartupGate.svelte` calls root command-backed refresh/project-event functions backed by `app-runtime.svelte.ts`.       |
| App dialogs/preferences/sidebar          | Done        | Settings, support info, keyboard overlay, sidebar, prompt history, IDE, and update preference commands mutate root context directly. |
| Session select/create/rename/stop/remove | In progress | Navigation/sidebar session actions use root commands; broader session domains pending.                               |
| Thread select/create/rename/remove       | In progress | Navigation/sidebar thread actions use root commands; broader thread domains pending.                                 |
| Chat submit/cancel/stream                | Pending     | Replaces `app.submit`, `session.submit`, thread submit/cancel/connect.                                               |
| Files open/save/rename/remove/diff       | Pending     | Replaces `session.files.*`.                                                                                          |
| Hooks refresh/rerun/pause                | Pending     | Replaces `session.hooks.*`.                                                                                          |
| Services refresh/start/stop/bind         | Pending     | Replaces `session.services.*`.                                                                                       |
| Agent commands run/credential confirm    | Pending     | Replaces `session.commands.*`.                                                                                       |
| Workspaces validate/create/update/remove | In progress | Workspace selector/settings call root workspace commands; remaining command internals/domain coverage pending.       |
| Credentials OAuth/create/update/remove   | In progress | Credential components read root credential data and refresh through commands; OAuth/create/remove APIs still local/command-backed. |
| Updates check/install/ignore             | Done        | Settings dialog calls root update commands; command internals use shell APIs and root update data directly.          |

## Latest status

- Type coverage reviewed against the existing context hierarchy.
- Root CQRS context is initialized from `routes/+layout.svelte`; app commands are initialized through `initializeAppCommands(...)` and the runtime seam in `app-runtime.svelte.ts`.
- Second-pass navigation/sidebar migration is complete for `AppHeader.svelte`, `AppKeyboardShortcuts.svelte`, `AppShell.svelte`, `AppSidebar.svelte`, `SessionToolbarStack.svelte`, and pending composer submit behavior.
- Sidebar open state, selected session/thread, pending session, mounted sessions, visible recent threads, session list/by-id, workspace list/by-id, startup banner projections, header update badge, and sidebar/header preferences are stored in root `context.view` / `context.data`.
- `AppSidebar.svelte` reads session list, session selection, visible recent threads, workspace lookup projections, and sidebar preference UI state from the root context; sidebar session/thread/workspace mutations now call root commands.
- Keyboard shortcut help and recent thread switcher overlay state are migrated to `context.view.app.dialogs` with command functions in `context/commands/app-view.ts`; switcher session/recent-thread projections read root data.
- Session/thread navigation actions for app header, app shell, keyboard shortcuts, sidebar, toolbar stack, and pending composer submit now go through root command functions.
- `AppMacWindowSpacer.svelte` is fully migrated and reads native window control environment from `context.data.environment`.
- All old app/session/thread context provider files and the temporary legacy context bridge are deleted.
- `context.actions.app` has been removed from the root context type and runtime.
- The transitional app-state bridge has been removed: `app-state.svelte.ts` and `create-app-state.svelte.ts` are deleted, and no production source references `getAppState()`, `setAppState()`, or `createAppState()`.
- Legacy-shaped `app-context.types.ts` remains temporarily as a type-alias home for shared view/data enums and shapes while remaining domain modules are split.
- All app root components have migration coverage for old context/app-state removal: no direct old context imports, old context entry point calls, temporary bridge imports, `context.actions.app` reads, or direct `getAppState()` imports remain in app components.
- `cd ui && pnpm typecheck` passes with `svelte-check found 0 errors and 0 warnings` after removing the app-state bridge.
- `cd ui && pnpm check` passes with `svelte-check found 0 errors and 0 warnings`.
- `cd ui && pnpm test:vitest` passes with `35` tests passing and `0` failures.
- Focused migration source tests pass with `46` tests passing and `0` failures.
- `pnpm test:ui` passes with `226` node tests and `35` Vitest tests passing.
