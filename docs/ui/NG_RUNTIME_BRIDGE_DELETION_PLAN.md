# Ng Runtime Bridge Deletion Plan

This plan tracks the remaining migration from the legacy runtime bridge under
`ui/src/lib/context/runtime` to the direct `NgContext` architecture.

The earlier context migration removed the old app/session/thread Svelte context
providers. This plan is narrower: remove the runtime bridge that still creates
`SessionRuntimeState`, `ThreadRuntimeSnapshot`, file/service/command runtime
domains, and projections back into `NgContext`.

## Target architecture

`NgContext` is the only app state root:

- `context.data` stores backend-backed caches for sessions, threads, files,
  hooks, services, commands, credentials, models, workspaces, startup tasks, and
  project/runtime state.
- `context.view` stores UI state such as selection, mounted sessions, workspace
  view mode, composer draft, pending comments, file buffers, hook dialogs,
  command credential dialogs, and conversation scroll positions.
- `context.commands` is the write/effect boundary for backend mutations,
  activations, refreshes, streaming, persistence, and coordinated UI changes.

The runtime bridge should stop owning duplicate state and should stop projecting
runtime state into `NgContext`. The component-level migration goal is that no
Svelte component imports `$lib/context/runtime` or any subpackage under that path.

## Current bridge responsibilities

The runtime bridge currently still owns or coordinates:

- Session state creation in `runtime/app.svelte.ts` and
  `runtime/session-state.svelte.ts`.
- Thread streaming, submit, cancel, refresh, queued prompts, and tool approval
  behavior in `runtime/thread-state.ts` and
  `runtime/thread-conversation.svelte.ts`.
- A runtime session view facade in `runtime/session-view-facade.ts`.
- File, hook, service, thread, and command domains under
  `runtime/session-domains/*`.
- Agent command credential dialog behavior through
  `ng/domains/agent-commands.ts`.
- Runtime delegation from `ng/domains/thread-composer.ts` for the hardest thread
  actions.

## Component migration queue

The component-level goal is zero imports from `$lib/context/runtime` or any
subpackage under that path. This queue has been completed: no Svelte component
currently imports `$lib/context/runtime` or a runtime subpackage.

Completed component migrations:

1. `ui/src/lib/components/app/parts/ConversationSelectionComment.svelte`
2. `ui/src/lib/components/app/parts/ConversationComposerHooksControl.svelte`
3. `ui/src/lib/components/app/parts/ServicePanel.svelte`
4. `ui/src/lib/components/app/parts/VSCodePanel.svelte`
5. `ui/src/lib/components/app/ConversationCredentialsControl.svelte`
6. `ui/src/lib/components/app/ConversationHooksPanel.svelte`
7. `ui/src/lib/components/app/ConversationWorkspaceSelector.svelte`
8. `ui/src/lib/components/app/ConversationPane.svelte`
9. `ui/src/lib/components/app/ConversationComposer.svelte`
10. `ui/src/lib/components/app/DockPanel.svelte`
11. `ui/src/lib/components/app/ThreadWorkspace.svelte`
12. `ui/src/lib/components/app/ThreadWorkspaceActive.svelte`
13. `ui/src/lib/components/app/SessionWorkspace.svelte`
14. `ui/src/routes/+layout.svelte`

Keep focused source-level tests in place so component runtime imports do not
return.

## Phased migration

### Phase 1: Migrate reads directly to `NgContext`

Move component read paths directly to the canonical `NgContext` state shape
instead of adding runtime-shaped `ng` compatibility snapshots.

Components should read from:

- `context.data.sessions.byId[sessionId]`
- `context.data.sessions.byId[sessionId].threads.byId[threadId]`
- `context.view.sessions[sessionId]`
- `context.view.sessions[sessionId].threads[threadId]`

If repeated derived logic is needed, first verify that the logic is actually
shared by at least two components. Only then add a narrow pure helper under
`ui/src/lib/` rather than putting helpers inside the context or `ng` data layer.
Helpers must not own state, cache data, or define an alternate component-facing
read model. If the logic is used by only one component, keep it local to that
component during migration.

### Phase 2: Migrate read-heavy components

Move components off `SessionRuntimeState` and `ThreadRuntimeSnapshot` imports
where behavior already flows through `context.commands`:

1. `ConversationComposerSessionSetupStatus.svelte`
2. `ConversationPane.svelte`
3. `ConversationHooksPanel.svelte`
4. `ConversationComposer.svelte`
5. `ThreadWorkspace.svelte`

### Phase 3: Replace session mount state

Migrate `SessionWorkspace.svelte` away from:

```ts
ensureRuntimeSessionState(...)
releaseRuntimeSessionState(...)
```

Use `NgContext` selection/navigation state and `context.commands.activateSession`
instead.

### Phase 4: Replace thread connection state

Migrate `ThreadWorkspaceActive.svelte` away from:

```ts
connectRuntimeThread(...)
releaseRuntimeThreadState(...)
```

Use direct thread activation/deactivation commands while preserving the current
behavior where active thread streams stay live even if inactive conversation DOM
nodes are unmounted.

### Phase 5: Move thread actions out of runtime

Rewrite `ng/domains/thread-composer.ts` so these actions no longer delegate to
runtime helpers:

- `submitThread`
- `refreshThread`
- `cancelThread`
- `addToolApprovalResponse`
- `deleteQueuedPrompt`
- `updateQueuedPrompt`

Use direct API methods, `ng/domains/threads.ts`, `ng/thread-subscription.ts`, and
`thread-stream.ts` primitives.

### Phase 6: Replace `SessionViewFacade`

Move remaining view-controller behavior to direct `NgContext` view helpers or
commands:

- chat/file/service/VS Code/diff view switching
- dock maximize toggling
- active service selection and service view mode
- hook dialog open/close and selected hook ID
- pending workspace setup fields
- file buffer and editor model/view-state helpers

Then `DockPanel.svelte` should consume `sessionId` and `threadId` plus
`NgContext`, not `SessionViewFacade`.

### Phase 7: Migrate agent command credential flow

Rewrite `ng/domains/agent-commands.ts` to operate on:

- `context.data.sessions.byId[sessionId].commands`
- `context.view.sessions[sessionId].commands.credentialDialog`
- credential/session-credential caches and commands

Preserve the current behavior that running an agent command ultimately submits a
chat message of the form `/${command.name}` after credential requirements are
satisfied.

### Phase 8: Delete runtime bridge

Once no production code imports `ui/src/lib/context/runtime/*`, delete:

- `runtime/app.svelte.ts`
- `runtime/session-state.svelte.ts`
- `runtime/thread-state.ts`
- `runtime/thread-conversation.svelte.ts` after direct parity exists
- `runtime/session-view-facade.ts`
- `runtime/session-context.types.ts`
- `runtime/recent-threads.ts` if direct recent-thread state replaces it
- `runtime/thread-selection-storage.ts` if direct selection persistence replaces
  it
- `runtime/session-domains/*`

## Verification checkpoints

After each phase, run focused source-level tests for the affected shell and
conversation components, then the UI typecheck:

```bash
cd ui && pnpm vitest run src/lib/components/test/session-workspace.vitest.ts
cd ui && pnpm vitest run src/lib/components/test/thread-workspace.vitest.ts
cd ui && pnpm vitest run src/lib/components/test/thread-workspace-active.vitest.ts
cd ui && pnpm vitest run src/lib/test/app-runtime.vitest.ts
cd ui && pnpm typecheck
```

For broader confidence before deleting runtime files, run:

```bash
pnpm check:frontend
```

## Guardrails

- Do not introduce new compatibility adapters, snapshots, or facades that own or
  reorganize duplicate state.
- Do not add new component-facing runtime APIs.
- No Svelte component should import `$lib/context/runtime` or any subpackage under
  that path in the final state.
- Keep `NgContext` as the single source of truth for shared state.
- Keep components reading state from `NgContext` and invoking commands for
  effects.
- Keep one-off derived logic local to the component. Extract a pure helper under
  `ui/src/lib/` only after confirming the same logic is used by at least two
  components.
- Preserve current streaming, pending question, queued prompt, retry/error, hook,
  service, file-buffer, and command credential behavior before deleting runtime
  equivalents.
