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

- Root, view, and data types: `ui/src/lib/context/context.types.ts`

## Migration phases

| Phase | Area                                                  | Status      | Notes                                                                                                                                                                                                                                                                 |
| ----- | ----------------------------------------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0     | Type design                                           | Done        | Root/view/data type files exist.                                                                                                                                                                                                                                      |
| 1     | Root CQRS context scaffold                            | Done        | Root `$state()` context and command lookup boundary added.                                                                                                                                                                                                            |
| 2     | App shell/root layout                                 | Done        | `routes/+layout.svelte` assigns the new context and initializes the root command runtime without the old app-state bridge.                                                                                                                                            |
| 3     | App-level view state                                  | Done        | Sidebar/dialog, preference, update, startup, and recent/mounted navigation projections moved to root context/commands.                                                                                                                                                |
| 4     | Session selection/navigation                          | Done        | Root selection/mounted-session state is managed by command runtime; session/thread navigation actions now use root commands.                                                                                                                                          |
| 5     | Session workspace provider removal                    | Done        | `SessionWorkspace` no longer sets the old session context or bridge.                                                                                                                                                                                                  |
| 6     | Thread workspace provider removal                     | Done        | `ThreadWorkspace` no longer sets the old thread context or bridge.                                                                                                                                                                                                    |
| 7     | Conversation data                                     | In progress | Messages, browser events, streaming, pending question, prompt queue, and conversation scroll positions are projected into root data/view; app component submit/cancel/refresh/connect/dispose, queue edit/delete, comment queue, and scroll writes use root commands. |
| 8     | Composer state                                        | In progress | Draft/options/comments are projected into root view; composer draft, next model/reasoning/tier, pending comments, and queued prompt mutations now use root command shims in `ConversationComposer.svelte`/`ConversationPane.svelte`.                                  |
| 9     | Files domain                                          | In progress | Backend file data and editor/view state are projected into root data/view; dock/files/diff actions use root commands; `FilesPanel` receives pure root data/view props plus callbacks instead of `SessionFilesDomain`.                                                 |
| 10    | Hooks domain                                          | In progress | Hook status/output is projected into root data/view; root hook command entry points exist.                                                                                                                                                                            |
| 11    | Services domain                                       | In progress | Service data and active service view state are projected into root data/view; root service command entry points exist; dock handlers use commands.                                                                                                                    |
| 12    | Commands domain                                       | In progress | Agent command list and credential dialog view state are projected into root data/view; toolbar run and credential dialog actions use root commands; the command credential dialog compatibility getter is removed.                                                    |
| 13    | Workspaces/models/credentials/startup/support/updates | In progress | Global backend/runtime refresh and update commands are root-context backed; deeper domain cleanup remains.                                                                                                                                                            |
| 14    | Remove old contexts                                   | Done        | Deleted old app/session/thread provider files and the temporary legacy bridge.                                                                                                                                                                                        |

## Component migration checklist

A component is **Done** only when its state reads and effectful behavior are fully
root-context / command-backed, with no legacy session/thread domain props or
compatibility adapters. **Partial** means the component no longer imports old
contexts or directly calls legacy domain methods, but still receives explicit
session/thread domain props, uses compatibility adapters, or has local data/view
splitting left for a later pass. Old context entry points include
`useAppContext()`, `useSessionContext()`, `useThreadContext()`,
`setAppContext()`, `setSessionContext()`, `setThreadContext()`,
`getAppContextIfPresent()`, or similar helpers.

| Component / module                              | Reads new context | Uses commands | Old context removed | Status                                                                                                                                                                                                                                                                     |
| ----------------------------------------------- | ----------------- | ------------- | ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `routes/+layout.svelte`                         | Yes               | N/A           | Yes                 | New context initialized without the old app context provider.                                                                                                                                                                                                              |
| `AppHeader.svelte`                              | Yes               | Partial       | Yes                 | Second-pass navigation complete; header environment/update/preference projections read root data/view and settings/new-session use commands.                                                                                                                               |
| `AppKeyboardShortcuts.svelte`                   | Yes               | Partial       | Yes                 | Switcher projections read root data/view and navigation/dialog/view-toggle behavior uses commands; no direct app-state accessor.                                                                                                                                           |
| `AppMacWindowSpacer.svelte`                     | Yes               | N/A           | Yes                 | Done; reads native window control environment from `context.data.environment`.                                                                                                                                                                                             |
| `AppSessionStatus.svelte`                       | Yes               | N/A           | Yes                 | Done; reads session status from root data.                                                                                                                                                                                                                                 |
| `AppShell.svelte`                               | Yes               | Partial       | Yes                 | Second-pass navigation complete; mounted sessions, sidebar state, selection, and startup banner projections read root data/view.                                                                                                                                           |
| `AppSidebar.svelte`                             | Yes               | Partial       | Yes                 | Session/recent/workspace/thread projections, sidebar preferences, and sidebar mutations use root context/commands; no direct app-state accessor.                                                                                                                           |
| `AppThreadStatus.svelte`                        | Yes               | N/A           | Yes                 | Done; reads thread/conversation status projections from root data.                                                                                                                                                                                                         |
| `ConversationComposer.svelte`                   | Yes               | Partial       | Yes                 | Reads app models/preferences and the composer draft projection from root context; composer draft/options/comments/queue writes and submit/cancel/history/dialog actions use root commands. Session/thread remain explicit props.                                           |
| `ConversationComposerSessionSetupStatus.svelte` | Yes               | Partial       | Yes                 | Reads workspace loading from root data and uses root credential command; remaining setup view reads explicit session/thread props.                                                                                                                                         |
| `ConversationCredentialsControl.svelte`         | Yes               | Partial       | Yes                 | Reads credential types from root data and calls root commands for credential management; session is an explicit prop.                                                                                                                                                      |
| `ConversationHooksPanel.svelte`                 | No                | Partial       | Yes                 | No old context entry points; hook pause/rerun behavior is provided by root-command callbacks, while dialog/download view state remains explicit props.                                                                                                                     |
| `ConversationPane.svelte`                       | Yes               | Partial       | Yes                 | Reads selected session/chat preferences and conversation scroll projection from root context; comment queue and scroll writes use root commands. Session/thread remain explicit props.                                                                                     |
| `ConversationWorkspaceSelector.svelte`          | Yes               | Partial       | Yes                 | Reads workspace data from root context and calls workspace validation/refresh commands; session is an explicit prop.                                                                                                                                                       |
| `CredentialsManager.svelte`                     | Yes               | Partial       | Yes                 | Reads credential data/dialog flow from root context and uses API/credential commands; no direct app-state accessor.                                                                                                                                                        |
| `DockPanel.svelte`                              | Yes               | Partial       | Yes                 | Reads theme/sidebar/files/services projections from root context, receives `sessionId`/`threadId` plus the workspace view controller, passes pure file data/view props to `FilesPanel`, and uses command-backed callbacks; no explicit session/thread domain props remain. |
| `SandboxProvidersManager.svelte`                | Yes               | Partial       | Yes                 | Reads credential data from root context and refreshes credentials through commands.                                                                                                                                                                                        |
| `SessionToolbar.svelte`                         | Yes               | Partial       | Yes                 | Reads IDE preferences and command credential dialog view from root context; agent command run and credential dialog actions use root commands.                                                                                                                             |
| `SessionToolbarStack.svelte`                    | Yes               | Partial       | Yes                 | Reads mounted session toolbar projections from root context.                                                                                                                                                                                                               |
| `SessionWorkspace.svelte`                       | Yes               | Partial       | Yes                 | Uses command-backed session ensure/release; no direct app-state accessor.                                                                                                                                                                                                  |
| `SettingsDialog.svelte`                         | Yes               | Partial       | Yes                 | Reads models/environment/preferences/update data from root context and mutates via commands.                                                                                                                                                                               |
| `SupportInfoDialog.svelte`                      | Yes               | Partial       | Yes                 | Reads support info from root data and fetches/closes through root commands.                                                                                                                                                                                                |
| `ThreadWorkspace.svelte`                        | No                | Partial       | Yes                 | No old context entry points; ensures the active thread through a root command while setup rendering still receives explicit session/thread props.                                                                                                                          |
| `ThreadWorkspaceActive.svelte`                  | No                | Partial       | Yes                 | No old context entry points; stream connect/dispose now use root commands while active view rendering still receives explicit session/thread props.                                                                                                                        |
| Old context definitions                         | N/A               | N/A           | Yes                 | Deleted `app-context.svelte.ts`, `session-context.svelte.ts`, `thread-context.svelte.ts`, and `legacy-context-bridge.svelte.ts`.                                                                                                                                           |

## Command migration checklist

| Command area                             | Status      | Notes                                                                                                                                                                                                                                                                                           |
| ---------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| App startup/refresh                      | Done        | `StartupGate.svelte` calls root command-backed refresh/project-event functions backed by `app-runtime.svelte.ts`.                                                                                                                                                                               |
| App dialogs/preferences/sidebar          | Done        | Settings, support info, keyboard overlay, sidebar, prompt history, IDE, and update preference commands mutate root context directly.                                                                                                                                                            |
| Session select/create/rename/stop/delete | In progress | Navigation/sidebar session actions use root commands; broader session domain internals remain.                                                                                                                                                                                                  |
| Thread select/create/rename/delete       | In progress | Navigation/sidebar thread actions use root commands; broader thread domain internals remain.                                                                                                                                                                                                    |
| Chat submit/cancel/stream                | In progress | Root submit/cancel/refresh/connect/dispose command entry points exist and app component call sites use them; command internals still delegate to the thread domain.                                                                                                                             |
| Composer/conversation view writes        | In progress | Root command shims cover composer draft clearing/updating, next model/reasoning/service-tier edits, pending comment add/remove/clear, queued prompt delete/update, and conversation scroll position writes; command internals still delegate to session/thread state and sync root projections. |
| Files open/save/rename/delete/diff       | In progress | Root file command entry points cover file open/close/refresh, diff target, tree toggles, buffer save/discard/conflict handling, rename/delete, and editor model/view-state callbacks. `FilesPanel` no longer receives a file-domain adapter.                                                    |
| Hooks refresh/rerun/pause                | In progress | Root hook command entry points exist and app component call sites use command-backed callbacks; command internals still delegate to the session hook domain.                                                                                                                                    |
| Services refresh/start/stop/bind         | In progress | Root service command entry points exist and dock handlers use them; display data reads root service projections.                                                                                                                                                                                |
| Agent commands run/credential confirm    | In progress | Root command refresh/run entry points and credential dialog action entry points exist; toolbar and the pure dialog part consume root view data plus callbacks, while internals still delegate to the session command domain.                                                                    |
| Workspaces validate/create/rename/delete | In progress | Workspace selector/settings call root workspace commands; remaining command internals/domain coverage remains.                                                                                                                                                                                  |
| Credentials OAuth/create/update/remove   | In progress | Credential components read root credential data and refresh through commands; OAuth/create/remove APIs still local/command-backed.                                                                                                                                                              |
| Updates check/install/ignore             | Done        | Settings dialog calls root update commands; command internals use shell APIs and root update data directly.                                                                                                                                                                                     |

## Latest status

- Type coverage reviewed against the existing context hierarchy.
- Root CQRS context is initialized from `routes/+layout.svelte`; app commands are initialized through `initializeAppCommands(...)` and the runtime seam in `app-runtime.svelte.ts`.
- Second-pass navigation/sidebar migration is complete for `AppHeader.svelte`, `AppKeyboardShortcuts.svelte`, `AppShell.svelte`, `AppSidebar.svelte`, `SessionToolbarStack.svelte`, and composer submit behavior.
- Sidebar open state, selected session/thread, pending session, mounted sessions, visible recent threads, session list/by-id, workspace list/by-id, startup banner projections, header update badge, and sidebar/header preferences are stored in root `context.view` / `context.data`.
- `AppSidebar.svelte` reads session list, session selection, visible recent threads, workspace lookup projections, and sidebar preference UI state from the root context; sidebar session/thread/workspace mutations now call root commands.
- Keyboard shortcut help and recent thread switcher overlay state are migrated to `context.view.app.dialogs` with command functions in domain-specific modules such as `context/commands/dialog.ts`, `context/commands/navigation.ts`, and `context/commands/session.ts`; switcher session/recent-thread projections read root data.
- Session/thread navigation actions for app header, app shell, keyboard shortcuts, sidebar, toolbar stack, and pending composer submit now go through root command functions.
- Mounted session projections now include conversation, composer, files, hooks,
  services, and agent command state under root `context.data` / `context.view`.
- Root command entry points exist for chat submit/cancel/refresh, composer draft/options/comment/queue/scroll writes, file
  open/close/save/rename/remove/diff refresh/tree/buffer/editor actions, hook refresh/rerun/pause, service
  refresh/open/start/stop/bind, and agent command refresh/run plus credential dialog close/confirm/selection/input/OAuth/refresh behavior. These
  currently delegate to the existing session/thread domain implementations while
  keeping root projections synchronized.
- `ConversationComposer.svelte`, `ConversationPane.svelte`,
  `DockPanel.svelte`, `SessionToolbar.svelte`, `ThreadWorkspace.svelte`, and
  `ThreadWorkspaceActive.svelte` now use root commands for chat
  submit/cancel/refresh/connect/dispose, hook-file open, diff-file open, file
  diff target refresh, FilesPanel file/tree/buffer/editor actions, service
  start/stop/bind, hook pause/rerun, agent command run actions, and command
  credential dialog actions.
- App components no longer directly call `session.files.*`,
  `session.hooks.*`, `session.services.*`, `session.commands.*`,
  `session.submit(...)`, `session.ensureThread(...)`, `session.ui.setComposerDraft(...)`,
  `session.conversationScrollTopByThreadId.*`, `thread.submit(...)`,
  `thread.cancel(...)`, `thread.refresh(...)`, `thread.connect(...)`,
  `thread.dispose(...)`, `thread.clearComposerDraft(...)`, `thread.setNextModelId(...)`,
  `thread.setNextReasoning(...)`, `thread.setNextServiceTier(...)`,
  `thread.clearNextComposerValues(...)`, `thread.addPendingComment(...)`,
  `thread.removePendingComment(...)`, `thread.clearPendingComments(...)`,
  `thread.deleteQueuedPrompt(...)`, or `thread.updateQueuedPrompt(...)`; source-level coverage now enforces this for app root
  components.
- `DockPanel.svelte` no longer receives explicit `SessionContextValue` or
  `ThreadContextValue` props; `ThreadWorkspaceActive.svelte` passes only
  `sessionId`, `threadId`, and the existing workspace view controller for dock
  open/close/maximize behavior. `ThreadWorkspaceActive.svelte`,
  `ConversationPane.svelte`, and `ConversationComposer.svelte` still keep
  explicit session/thread domain objects where they drive runtime lifecycle,
  pending-session setup, or broad composer/conversation reads that are not yet
  safely split into root projections.
- The files compatibility adapter has been removed: `FilesPanel` is pure and
  receives root file data/view plus callbacks from `DockPanel`. The command
  credential dialog compatibility getter has also been removed:
  `SessionToolbar.svelte` reads `context.view.sessions[sessionId].commands.credentialDialog`
  and passes pure dialog data plus root-command callbacks into
  `SessionCommandCredentialsDialog.svelte`.
- `AppMacWindowSpacer.svelte` is fully migrated and reads native window control environment from `context.data.environment`.
- All old app/session/thread context provider files and the temporary legacy context bridge are deleted.
- `context.actions.app` has been removed from the root context type and runtime.
- The transitional app-state bridge has been removed: `app-state.svelte.ts` and `create-app-state.svelte.ts` are deleted, and no production source references `getAppState()`, `setAppState()`, or `createAppState()`.
- Legacy-shaped `app-context.types.ts` remains temporarily as a type-alias home for shared view/data enums and shapes while remaining domain modules are split.
- All app root components have migration coverage for old context/app-state removal: no direct old context imports, old context entry point calls, temporary bridge imports, `context.actions.app` reads, or direct `getAppState()` imports remain in app components.
- `pnpm --dir ./ui typecheck` passes with `svelte-check found 0 errors and 0 warnings`.
- Focused component migration tests pass with `node --test ui/src/lib/components/test/component-conventions.test.ts ui/src/lib/components/test/conversation-composer.test.ts ui/src/lib/components/test/conversation-pane.test.ts ui/src/lib/components/test/dock-panel.test.ts ui/src/lib/components/test/thread-workspace-active.test.ts` (`11` tests, `0` failures).
- Focused `node --test ui/src/lib/app-runtime.test.ts` passes with `5` tests and `0` failures.
