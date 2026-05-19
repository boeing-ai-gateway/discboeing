# ui-go Porting Status

This file is the shared checklist for porting Svelte components to `ui-go`.
Agents running in parallel should update only the checkbox and notes for the
component they were assigned, plus any shared-file notes they need to report.

Status markers:

- `[ ]` Not started
- `[~]` In progress or partial parity
- `[x]` Ported with current known parity
- `[!]` Blocked by missing shared infrastructure

When updating a component, keep its section in place and append concise notes.
Do not reorder the file during parallel work.

## Runtime/Integration Checklist

The component shell inventory is broad enough to shift the remaining port work
from per-component markup parity to runtime behavior. Keep component entries
below intact, and update this checklist as integration slices land.

- [x] Define the Go UI runtime contract: route shape, command payload format,
  read-model refresh boundaries, error reporting, and optimistic-update rules.
- [x] Inventory action/data hooks into command domains and keep the inventory
  current as new hooks are added.
- [x] Build a command dispatcher that accepts existing `data-*action` hooks and
  routes them to typed server handlers.
- [x] Wire the first vertical slice for the core chat loop: composer submit,
  user-message append, assistant/status update, and conversation fragment patch.
- [ ] Replace hard-coded shell snapshots with read-model builders backed by the
  Discobot API/server state.
- [x] Add event propagation for session/thread updates using Datastar SSE
  patches or an equivalent fragment-refresh mechanism.
- [x] Add initial HTTP cache policy: dynamic UI routes are `no-store`,
  unversioned static assets revalidate with `no-cache`, generated chunk assets
  under `/assets/chunks/` use long-lived immutable caching, and static assets
  bypass session-cookie creation.
- [ ] Implement small JS islands only for browser-local behavior that cannot be
  server-rendered, such as clipboard, keyboard shortcuts, VNC, terminals, and
  Pierre diff lifecycle.
- [ ] Add an integration test matrix for the critical path: session selection,
  composer send/cancel, credentials, settings, dock panels, files, services,
  and diff review.
- [ ] Reclassify `[~]` entries as integration-ready, behavior-complete, or
  intentionally JS-only once the command/runtime layer exists.

### Runtime command-domain inventory

Initial hook scan from templ source:

- Shell/navigation: `data-action` on session/thread row menus, plus existing
  sidebar refresh command endpoint `/ui/commands/sidebar-refresh`.
- Core composer/chat: `data-composer-submit-action`, `data-branch-action`,
  slash-command metadata (`data-command-count`, `data-command-description`),
  `data-queued-prompt-action`, and `data-schedule-action`.
- Credentials: `data-credential-action`, `data-credential-type-action`,
  `data-env-var-action`, `data-oauth-scopes-action`, `data-oauth-wizard-action`,
  and command-credential request/select/create/validity/launch fields.
- Dock/panels: `data-desktop-action`, `data-diff-action`, and
  `data-service-action`; terminal, VS Code, files, and other dock panels still
  expose state hooks but need command hooks as their behavior is wired.
- Browser-local islands: `data-clipboard-text`, `data-dev-error-action`, Pierre
  diff rendering/style refresh hooks, keyboard shortcuts, VNC, terminal, and
  dialog focus/open state.

### Command implementation backlog

Current server command routes:

- [x] `POST /ui/commands/composer-submit` — resolve/create the pending
  workspace/session as needed, submit the prompt through the backend chat API,
  save the session-scoped view, and let `/ui/stream` patch the UI.
- [x] `POST /ui/commands/composer-stop` — clears temporary generating state in
  the session-scoped composer view until real agent cancellation is wired.
- [x] `POST /ui/commands/composer-workspace` — handles the pending-session
  workspace selector option changes, source input validation, suggestion
  selection, and reset-to-dropdown flow.
- [x] `POST /ui/commands/composer-schedule` — handles the composer run-after
  popover toggle, preset times, pause, run-now, and custom datetime selection.
- [x] `POST /ui/commands/sidebar-refresh` — rebuild the left sidebar from the
  client read side while preserving the current selection.
- [x] `POST /ui/commands/sidebar/new-session` — create a session through the
  client, select it, save the session view, and let `/ui/stream` patch the UI.
- [x] `POST /ui/commands/sidebar/select-session` — select a session row and
  rebuild the session workspace/sidebar view from the client read side.
- [x] `POST /ui/commands/sidebar/select-thread` — select a recent/nested thread
  row for the current session view.
- [x] `POST /ui/commands/sidebar/toggle-grouping` — toggle grouped-by-workspace
  sidebar rendering and rebuild the sidebar.
- [x] `POST /ui/commands/sidebar/session-menu` — opens the server-owned
  session action menu in the session-scoped sidebar view.
- [x] `POST /ui/commands/sidebar/thread-menu` — opens the server-owned thread
  action menu in the session-scoped sidebar view.
- [x] `POST /ui/commands/sidebar/close-menu` — clears sidebar menu and dialog
  state.
- [x] `POST /ui/commands/sidebar/session-action` — dispatches session menu
  actions for new thread, rename, stop, and delete confirmation.
- [x] `POST /ui/commands/sidebar/thread-action` — dispatches thread menu
  actions for rename and delete confirmation.
- [x] `POST /ui/commands/sidebar/rename` — renames the selected session or
  thread through the client seam and rebuilds the sidebar view.
- [x] `POST /ui/commands/sidebar/delete` — deletes the selected session or
  thread through the client seam and rebuilds the sidebar view.
- [x] `POST /ui/commands/sidebar/workspace-menu` — opens the server-owned
  workspace action menu in the session-scoped sidebar view.
- [x] `POST /ui/commands/sidebar/workspace-action` — dispatches workspace menu
  actions for rename and delete confirmation.
- [x] `POST /ui/commands/sidebar/toggle-section` — toggles the server-owned
  Recent and All sessions section open state.
- [x] `POST /ui/commands/sidebar/toggle-collapsed` — toggles the
  session-scoped sidebar rail state and patches the shell layout.
- [x] `POST /ui/commands/sidebar/toggle-floating` — toggles the collapsed
  sidebar's floating body without fully expanding the panel.
- [x] `POST /ui/commands/message/branch` — switches the active branch for a
  branched conversation message in the session-scoped view.
- [x] `POST /ui/commands/prompt-queue/action` — handles queued prompt row
  controls for the temporary session-scoped read model.
- [x] `POST /ui/commands/credentials/action` — handles configured credential
  row toggle-inactive, edit-placeholder, delete, add-editor, close-editor, and
  provider selection actions in the session-scoped settings view.
- [x] `POST /ui/commands/credentials/env-var` — handles add-row,
  show-value-input, hide-value-input, and remove-row controls in the
  server-owned credential editor.
- [x] `POST /ui/commands/credentials/oauth-scopes` — handles mode switching,
  customize, reset-defaults, and enable/disable controls in the server-owned
  OAuth scope picker.
- [x] `POST /ui/commands/credentials/oauth-wizard` — handles temporary
  server-owned OAuth wizard flow selection, fake auth/device-code startup,
  copy-state flags, polling toggle, and close actions.
- [x] `POST /ui/commands/settings/action` — opens/closes the global settings
  dialog, switches settings tabs, mutates the remaining server-owned
  appearance/chat preferences, and opens/closes the support-info dialog. Theme
  mode and color scheme are intentionally browser-local.
- [x] `GET /ui/stream` — session-scoped Datastar patch stream fed by state save
  events.

Action hooks extracted from the ported templ UI that still need typed command
routes or browser-local islands:

- [~] Shell/navigation
  - `data-action="session-menu"`
  - `data-action="thread-menu"`
  - Session selection, thread selection, new session, sidebar refresh, workspace
    grouping, Recent/All section collapse, and collapsed floating sidebar
    expansion now use typed CQRS command routes backed by `FakeClient` and
    session-scoped view state.
  - Session, thread, and workspace row action buttons now open server-owned
    menus with rename/delete dialogs. Session menu actions can create a thread,
    rename, stop, or delete; thread menu actions can rename or delete
    non-primary threads; workspace menu actions can rename or delete the
    workspace and its sessions in the fake client.
- [x] Message branching
  - `data-branch-action="previous"`
  - `data-branch-action="next"`
  - Branch controls now post to `/ui/commands/message/branch` with message ID
    and direction. The current fake composer response includes placeholder
    assistant branches so branch cycling can be exercised before real Discobot
    thread branch data is wired.
- [~] Composer submit state
  - `data-composer-submit-action` currently describes submit/stop/disabled UI
    state. Stop now posts to a typed command that clears temporary generating
    state; real chat-stream cancellation still depends on agent API wiring.
- [~] Queued prompts
  - `data-queued-prompt-action="cancel-edit"`
  - `data-queued-prompt-action="save-edit"`
  - `data-queued-prompt-action="move-up"`
  - `data-queued-prompt-action="move-down"`
  - `data-queued-prompt-action="edit"`
  - `data-queued-prompt-action="schedule"`
  - `data-queued-prompt-action="run-now"`
  - `data-queued-prompt-action="pause"`
  - `data-queued-prompt-action="delete"`
  - Row controls now post to `/ui/commands/prompt-queue/action`; reorder,
    edit/cancel, schedule-toggle, run-now, pause, and delete mutate the
    session-scoped queue. Saving edited text still needs form/signal plumbing.
- [~] Prompt scheduling
  - `data-schedule-action="later"`
  - `data-schedule-action="pause"`
  - `data-schedule-action="run-now"`
  - `data-schedule-action="save-custom"`
  - Composer run-after scheduling now matches the Svelte popover layout and is
    command-backed for presets, pause, run-now, and custom datetime values.
    Queue row preset scheduling, pause, and run-now are command-backed; queue
    row custom datetime input remains future plumbing.
- [~] Settings
  - Header settings button now posts to `/ui/commands/settings/action?action=open`.
  - The settings dialog shell is server-owned: Done closes it, tabs switch
    through the command route, recent-limit and chat/appearance switches toggle
    session-scoped preferences, and Support information opens/closes a nested
    server-owned dialog with placeholder diagnostic JSON.
  - Theme mode (`light`/`dark`/`system`) and color scheme now mimic the Svelte
    app's browser-local model: an inline first-paint script applies
    `localStorage["theme"]` and the resolved mode's
    `localStorage["theme.colorScheme.light"]` /
    `localStorage["theme.colorScheme.dark"]` before CSS loads, while the
    settings dialog uses Datastar signals/bindings for the theme mode controls,
    separate light/dark palette selectors, resolved-mode label, and active
    palette application. A small browser-local helper still owns localStorage,
    `<html class="dark">`, `data-theme`, system preference changes, and Pierre
    diff rendering side effects.
  - Remaining settings work includes real persistence for non-theme
    preferences, model selection, update actions, clear-cache, and full
    provider/credential editor API integration.
- [~] Credentials
  - `data-credential-action="toggle-inactive"`
  - `data-credential-action="edit"`
  - `data-credential-action="delete"`
  - `data-credential-type-action="choose"`
  - Configured credential rows now post to `/ui/commands/credentials/action`.
    Toggle-inactive flips the row's inactive state, edit opens the placeholder
    editor in edit mode, delete removes the row from the session-scoped
    settings view, Add credential opens the provider picker, and provider
    choices initialize the appropriate editor shell. Real credential API
    persistence and the full editor remain future work.
- [x] Credential environment variables
  - `data-env-var-action="add-row"`
  - `data-env-var-action="show-value-input"`
  - `data-env-var-action="hide-value-input"`
  - `data-env-var-action="remove-row"`
  - Env-var row controls now post to `/ui/commands/credentials/env-var`.
    Add-row, show/hide stored value replacement, and remove-row mutate the
    session-scoped credential editor snapshot. Key/value text submission and
    real secure storage persistence remain future work.
- [x] OAuth scopes
  - `data-oauth-scopes-action="reset-defaults"`
  - `data-oauth-scopes-action="mode"`
  - `data-oauth-scopes-action="customize"`
  - `data-oauth-scopes-action="set-enabled"`
  - OAuth scope controls now post to
    `/ui/commands/credentials/oauth-scopes`. Simple/advanced mode, customize,
    reset-to-defaults, and enable/disable mutate the session-scoped picker
    snapshot. Real provider scope persistence remains future work.
- [~] OAuth wizard
  - `data-oauth-wizard-action="select-kind"`
  - `data-oauth-wizard-action="open-auth-url"`
  - `data-oauth-wizard-action="use-device-code"`
  - `data-oauth-wizard-action="copy-auth-url"`
  - `data-oauth-wizard-action="submit-code"`
  - `data-oauth-wizard-action="start-device"`
  - `data-oauth-wizard-action="open-verification-url"`
  - `data-oauth-wizard-action="copy-code"`
  - `data-oauth-wizard-action="start-polling"`
  - `data-oauth-wizard-action="close"`
  - OAuth wizard buttons now post to
    `/ui/commands/credentials/oauth-wizard`. Selecting a GitHub OAuth provider
    opens the temporary wizard; flow switching, fake sign-in URL/device-code
    startup, copied-state flags, polling toggle, and close mutate the
    session-scoped settings snapshot. Real OAuth browser launch, callback code
    parsing, clipboard writes, and provider polling remain future work.
- [ ] Desktop panel
  - `data-desktop-action="toggle-maximized"`
  - `data-desktop-action="close"`
  - `data-desktop-action="reconnect"`
- [ ] Diff review
  - `data-diff-action="target-menu"`
  - `data-diff-action="set-style"`
  - `data-diff-action="refresh"`
  - `data-diff-action="toggle-maximized"`
  - `data-diff-action="close"`
  - `data-diff-action="mark-all-approved"`
  - `data-diff-action="toggle-file"`
  - `data-diff-action="ignore-whitespace"`
  - `data-diff-action="toggle-approved"`
  - `data-diff-action="open-file"`
  - `data-diff-action="submit-comment"`
  - `data-diff-action="clear-comment"`
- [ ] Services
  - `data-service-action={ service.ID }` start/stop/open behavior needs a typed
    command route that includes service ID and requested operation.
- [ ] Browser-local islands
  - `data-clipboard-text` copy buttons should remain browser-local.
  - `data-dev-error-action="clear"`, `"copy"`, and `"dismiss"` are dev-overlay
    browser-local actions unless persisted error state becomes server-owned.
  - Link-safety modal close/copy, attachment remove click shell, keyboard
    shortcuts, VNC, terminal, and Pierre diff lifecycle remain JS-island work.

### Runtime progress notes

- Added `/ui/commands/composer-submit` as the first command route. It parses the
  composer form, appends session-scoped user/assistant placeholder messages, and
  relies on `/ui/stream` to patch `#conversation-pane`.
- Added the first read-model refresh boundary for the conversation slice. It now
  reads from the session-scoped view and must still be replaced by real
  Discobot session/thread API data.
- Updated the composer form to submit to the composer command route instead of
  the sidebar refresh placeholder.
- Refactored server-side runtime code out of `cmd/ui-go/main.go` into
  `internal/config`, `internal/state`, `internal/readmodel`, `internal/command`,
  and `internal/server`; `main.go` now only boots the configured server.
  Command package wiring lives in `handler.go`, with one command handler per
  file such as `composer_submit.go` and `sidebar_refresh.go`.
- Added `internal/discobot` as the ui-go-owned client interface seam. Its
  service interfaces mirror the existing `server/api` client shape without
  importing the concrete server API client into command/read-model packages.
- Added cookie-backed ui-go browser sessions with `ui_go_session_id`. Requests
  without a session cookie now receive one, and handlers look up state through
  the request-scoped session ID.
- Reworked the temporary state store around session-scoped frontend view
  snapshots. Commands mutate the current session's view model through `Save`,
  and each save publishes an event for `/ui/stream`.
- Replaced the periodic stream ticker with per-session save subscriptions. The
  Datastar stream now patches the sidebar and conversation pane when the
  session view changes.
- Adjusted command routes to follow the CQRS split: commands now parse input,
  update and save the session view, then return `204 No Content`; `/ui/stream`
  is the only route that sends Datastar patch streams.
- Simplified session view storage by encoding each session's frontend snapshot
  to gob bytes in memory. Reads and saves decode/encode the snapshot, avoiding
  hand-written deep-clone helpers while preserving copy-on-read behavior.
- Updated the templ generator command to `v0.3.1020`, matching the current
  `github.com/a-h/templ` version in `go.mod`; the previous generator/version
  warning is resolved.
- Added `internal/discobot.FakeClient`, seeded with fake projects, workspaces,
  and an empty workspace, to exercise ui-go command/read-model flows before the
  concrete server API adapter is wired.
- Implemented the left-sidebar command slice: refresh, new session,
  select-session, select-thread, workspace grouping toggle, and row action-menu
  request commands. Sidebar controls now post to typed command routes, commands
  save the browser session view, and `/ui/stream` sends the resulting patches.
- Removed sample sessions/threads from the default view. Initial page load now
  starts from a no-session-selected state with an empty sidebar and disabled
  composer until the user creates or selects a session.
- Added the message-branching command slice. Branched messages render the
  existing previous/page/next controls, branch buttons post to a typed command,
  and the active branch is stored in the session-scoped conversation message.
- Added typed queued-prompt row commands. Queue controls can now reorder,
  toggle edit state, toggle the schedule popover, apply preset schedule
  offsets, pause, run, and delete entries from the browser session view.
- Added `/ui/commands/composer-stop` for the submit button's stop state. It
  clears temporary streaming/submitted flags in the session view; real
  cancellation remains part of the future Discobot API chat-stream slice.
- Adjusted collapsed sidebar styling to match the Svelte desktop UI: the
  collapsed state now overlays a small transparent floating sessions header
  instead of rendering a full-height narrow rail.
- Matched the Svelte collapsed floating sidebar behavior. The PanelLeft button
  fully expands the sessions panel, while the Sessions chevron posts to
  `/ui/commands/sidebar/toggle-floating` to open or close the floating body in
  place.
- Implemented server-owned session/thread/workspace command menus. Sidebar
  menu/dialog state now lives in the view model, menu actions post to typed
  command routes, and the fake client supports session rename/delete/stop,
  thread create/rename/delete, and workspace rename/delete operations.
- Added server-owned Recent and All sessions collapse state. Section headers now
  post to `/ui/commands/sidebar/toggle-section`, and sidebar rebuilds preserve
  the user's open/closed state.
- Added no-store headers for dynamic ui-go routes (`/` and `/ui/*`) so
  session-scoped HTML, commands, and Datastar streams are not browser-cached.
- Matched dialog overlays to the Svelte UI's `bg-black/50` backdrop styling
  without blur.
- Added the first credentials command route. Configured credential row controls
  are no longer disabled: toggle, edit, and delete post to the server and mutate
  the session-scoped settings snapshot while the real credentials API/editor
  integration remains pending.
- Added the first settings dialog command slice. The upper-right settings icon
  opens the global dialog, Done closes it, tabs and simple appearance/chat
  controls post back to the server, sidebar rebuilds preserve dialog state, and
  the Support information nested dialog can open/close with placeholder JSON.
- Extended the credentials command slice. Add credential opens the provider
  picker, provider choices initialize env-var or OAuth editor state, env-var
  row controls are command-backed, and OAuth scope mode/reset/toggle controls
  are command-backed in the session-scoped settings snapshot.
- Added the first OAuth wizard command slice. GitHub provider selection opens a
  server-owned wizard shell, and the visible wizard controls update fake
  authorization/device-code state without leaving the page.
- Validation: `pnpm --dir ui-go generate` and `pnpm --dir ui-go check` passed.


### [x] `ui/src/lib/components/ai/attachments/Attachment.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/{types.go,attachment.templ}
- Validation: generate/check passed
- Notes:
  - Added explicit AttachmentView props to replace Svelte attachment context for templ callers.

### [x] `ui/src/lib/components/ai/attachments/AttachmentEmpty.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment_empty.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/attachment_empty.templ
- Validation: generate/check passed
- Notes:
  - Ported empty-state wrapper with default “No attachments” fallback and optional children.

### [~] `ui/src/lib/components/ai/attachments/AttachmentHoverCard.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment_hover_card.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/attachment_hover_card.templ, ui-go/content/lib/components/ui/hover-card/*.templ
- Validation: generate/check passed
- Notes:
  - Wired attachment hover-card root to the shared hover-card shell while preserving attachment delay metadata; hover visibility/focus behavior remains client-side.

### [~] `ui/src/lib/components/ai/attachments/AttachmentHoverCardContent.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment_hover_card_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/attachment_hover_card_content.templ, ui-go/content/lib/components/ui/hover-card/*.templ
- Validation: generate/check passed
- Notes:
  - Wired content through shared HoverCardContent with attachment sizing and start alignment; positioning/open behavior remains client-side.

### [~] `ui/src/lib/components/ai/attachments/AttachmentHoverCardTrigger.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment_hover_card_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/attachment_hover_card_trigger.templ, ui-go/content/lib/components/ui/hover-card/*.templ
- Validation: generate/check passed
- Notes:
  - Wired trigger through shared HoverCardTrigger; hover/focus behavior remains client-side.

### [x] `ui/src/lib/components/ai/attachments/AttachmentInfo.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment_info.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/{types.go,attachment_info.templ}
- Validation: generate/check passed
- Notes:
  - Ported non-grid label/media-type rendering using Go attachment label helper.

### [~] `ui/src/lib/components/ai/attachments/AttachmentPreview.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment_preview.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/{types.go,attachment_preview.templ}
- Validation: generate/check passed
- Notes:
  - Ported media/icon preview and image link fallback; modal fullscreen parity now depends on client-side dialog open/close state rather than missing shared primitives.
  - Added Go helper for Svelte-equivalent fullscreen gating plus media-category metadata on the preview shell.

### [~] `ui/src/lib/components/ai/attachments/AttachmentRemove.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachment_remove.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/{types.go,attachment_remove.templ}
- Validation: generate/check passed
- Notes:
  - Ported removable button styling and default label fallback; actual remove command callback must be wired by future consumer-specific Datastar command.

### [x] `ui/src/lib/components/ai/attachments/Attachments.svelte`

- Target: `ui-go/content/lib/components/ai/attachments/attachments.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/attachments/attachments.templ
- Validation: generate/check passed
- Notes:
  - Ported variant container classes and explicit variant data attribute for child callers.

### [x] `ui/src/lib/components/ai/code-block/CodeBlock.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/{types.go,code_block.templ}
- Validation: generate/check passed
- Notes:
  - Ported wrapper that renders optional header children and code content with explicit View props.

### [x] `ui/src/lib/components/ai/code-block/CodeBlockActions.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_actions.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_actions.templ
- Validation: generate/check passed
- Notes:
  - Ported action-row wrapper classes and child slot.

### [x] `ui/src/lib/components/ai/code-block/CodeBlockContainer.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_container.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_container.templ
- Validation: generate/check passed
- Notes:
  - Ported bordered container, data-language, and content-visibility style.

### [x] `ui/src/lib/components/ai/code-block/CodeBlockContent.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/{types.go,code_block_content.templ}
- Validation: generate/check passed
- Notes:
  - Ported pre/code layout and line-number classes using Go line splitting.

### [~] `ui/src/lib/components/ai/code-block/CodeBlockCopyButton.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_copy_button.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/{types.go,code_block_copy_button.templ}
- Validation: generate/check passed
- Notes:
  - Ported button and clipboard write data hook; copied-state icon/timeout callback parity needs a small JS island.
  - Added explicit copy label/title and `data-clipboard-text` metadata for future JS behavior.

### [x] `ui/src/lib/components/ai/code-block/CodeBlockFilename.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_filename.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_filename.templ
- Validation: generate/check passed
- Notes:
  - Ported font-mono span wrapper.

### [x] `ui/src/lib/components/ai/code-block/CodeBlockHeader.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_header.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_header.templ
- Validation: generate/check passed
- Notes:
  - Ported muted header layout wrapper.

### [~] `ui/src/lib/components/ai/code-block/CodeBlockLanguageSelector.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_language_selector.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_language_selector.templ, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Wired language selector root through the shared select shell while preserving selected value metadata; selection behavior remains client-side.

### [~] `ui/src/lib/components/ai/code-block/CodeBlockLanguageSelectorContent.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_language_selector_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_language_selector_content.templ, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Wired language selector content through shared SelectContent; popup positioning/open behavior remains client-side.

### [~] `ui/src/lib/components/ai/code-block/CodeBlockLanguageSelectorItem.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_language_selector_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_language_selector_item.templ, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Wired language option through shared SelectItem with selected indicator support; item activation remains client-side.

### [~] `ui/src/lib/components/ai/code-block/CodeBlockLanguageSelectorTrigger.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_language_selector_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_language_selector_trigger.templ, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Wired language selector trigger through shared SelectTrigger with original compact classes; open behavior remains client-side.

### [x] `ui/src/lib/components/ai/code-block/CodeBlockLanguageSelectorValue.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_language_selector_value.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_language_selector_value.templ
- Validation: generate/check passed
- Notes:
  - Ported select-value span classes and data-slot attribute.

### [x] `ui/src/lib/components/ai/code-block/CodeBlockTitle.svelte`

- Target: `ui-go/content/lib/components/ai/code-block/code_block_title.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/code-block/code_block_title.templ
- Validation: generate/check passed
- Notes:
  - Ported title row wrapper classes and child slot.

### [~] `ui/src/lib/components/ai/image-attachment/ImageAttachment.svelte`

- Target: `ui-go/content/lib/components/ai/image-attachment/image_attachment.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/image-attachment/{image_attachment.go,image_attachment.templ}
- Validation: generate/check passed
- Notes:
  - Added thumbnail/open/download rendering; full zoom modal, wheel handling, Escape handling, and shell download parity need a JS island.
  - Added filename fallback plus explicit download aria label to match the Svelte shell more safely when filename is missing.

### [~] `ui/src/lib/components/ai/link-safety-modal/LinkSafetyModal.svelte`

- Target: `ui-go/content/lib/components/ai/link-safety-modal/link_safety_modal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/link-safety-modal/{link_safety_modal.go,link_safety_modal.templ}
- Validation: generate/check passed
- Notes:
  - Ported modal markup, copy button, and external link fallback; shell openUrl/onClose/onConfirm copied-state parity needs command/JS wiring.
  - Added URL fallback label, copy metadata, and explicit close/open/copy aria labels.

### [x] `ui/src/lib/components/ai/loader.svelte`

- Target: `ui-go/content/lib/components/ai/loader.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/loader.templ
- Validation: generate/check passed
- Notes:
  - Ported inline SVG loader and size/class props.

### [x] `ui/src/lib/components/ai/message/Message.svelte`

- Target: `ui-go/content/lib/components/ai/message/message.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/{types.go,message.templ}
- Validation: generate/check passed
- Notes:
  - Ported role-based wrapper classes and conversation message id attribute.

### [~] `ui/src/lib/components/ai/message/MessageAction.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_action.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_action.templ, ui-go/content/lib/components/ui/tooltip/*.templ
- Validation: generate/check passed
- Notes:
  - Added tooltip provider/root/content shell around actions with tooltip text while preserving title and sr-only label; hover/focus visibility remains client-side.

### [x] `ui/src/lib/components/ai/message/MessageActions.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_actions.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_actions.templ
- Validation: generate/check passed
- Notes:
  - Ported actions row wrapper.

### [~] `ui/src/lib/components/ai/message/MessageBranch.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_branch.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/{types.go,message_branch.templ}
- Validation: generate/check passed
- Notes:
  - Ported branch container with explicit BranchView; branch-change callbacks need command wiring.
  - Reviewed against Svelte context root; static shell preserves branch data for child controls.

### [x] `ui/src/lib/components/ai/message/MessageBranchContent.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_branch_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_branch_content.templ
- Validation: generate/check passed
- Notes:
  - Ported current-branch conditional rendering from explicit BranchView props.

### [~] `ui/src/lib/components/ai/message/MessageBranchNext.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_branch_next.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/{types.go,message_branch_next.templ}
- Validation: generate/check passed
- Notes:
  - Ported next button markup; click behavior needs branch command wiring.
  - Matched ghost icon-sm button styling and retained disabled state from branch total.

### [x] `ui/src/lib/components/ai/message/MessageBranchPage.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_branch_page.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_branch_page.templ
- Validation: generate/check passed
- Notes:
  - Ported page text with clamped current/total branch helpers.

### [~] `ui/src/lib/components/ai/message/MessageBranchPrevious.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_branch_previous.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/{types.go,message_branch_previous.templ}
- Validation: generate/check passed
- Notes:
  - Ported previous button markup; click behavior needs branch command wiring.
  - Matched ghost icon-sm button styling and retained disabled state from branch total.

### [~] `ui/src/lib/components/ai/message/MessageBranchSelector.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_branch_selector.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_branch_selector.templ, ui-go/content/lib/components/ui/button-group/*.templ
- Validation: generate/check passed
- Notes:
  - Wired branch selector through the shared ButtonGroup shell while preserving conditional rendering and message role metadata; previous/next commands remain client-side.

### [x] `ui/src/lib/components/ai/message/MessageContent.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_content.templ
- Validation: generate/check passed
- Notes:
  - Ported user/assistant content classes and stack margin rule.

### [~] `ui/src/lib/components/ai/message/MessageResponse.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_response.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_response.templ, ui-go/content/lib/components/ai/streamdown/svelte_streamdown.templ
- Validation: generate/check passed
- Notes:
  - Wired text rendering through the Streamdown shell while preserving message response layout and children fallback; full markdown parsing/animation remains limited by the current Streamdown port.

### [x] `ui/src/lib/components/ai/message/MessageToolbar.svelte`

- Target: `ui-go/content/lib/components/ai/message/message_toolbar.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/message/message_toolbar.templ
- Validation: generate/check passed
- Notes:
  - Ported toolbar layout wrapper.

### [~] `ui/src/lib/components/ai/reasoning/Reasoning.svelte`

- Target: `ui-go/content/lib/components/ai/reasoning/reasoning.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/reasoning/{types.go,reasoning.templ}, ui-go/content/lib/components/ui/collapsible/*.templ
- Validation: generate/check passed
- Notes:
  - Wired reasoning root through the shared Collapsible shell while preserving data-ai attributes and view state; automatic streaming/open transitions still need client command wiring.

### [~] `ui/src/lib/components/ai/reasoning/ReasoningContent.svelte`

- Target: `ui-go/content/lib/components/ai/reasoning/reasoning_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/reasoning/reasoning_content.templ, ui-go/content/lib/components/ai/streamdown/svelte_streamdown.templ
- Validation: generate/check passed
- Notes:
  - Wired reasoning text through the Streamdown shell and added the Svelte slide/fade state classes; preview extraction and animation state remain simplified in the Go read model.

### [~] `ui/src/lib/components/ai/reasoning/ReasoningTrigger.svelte`

- Target: `ui-go/content/lib/components/ai/reasoning/reasoning_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/reasoning/reasoning_trigger.templ, ui-go/content/lib/components/ui/collapsible/*.templ
- Validation: generate/check passed
- Notes:
  - Wired non-streaming reasoning controls through shared CollapsibleTrigger shells while preserving icons, messages, and chevron state; click behavior remains client-side.

### [x] `ui/src/lib/components/ai/shimmer.svelte`

- Target: `ui-go/content/lib/components/ai/shimmer.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/shimmer.templ
- Validation: generate/check passed
- Notes:
  - Ported shimmer markup, inline style variables, and keyframes.

### [~] `ui/src/lib/components/ai/streamdown/SvelteStreamdown.svelte`

- Target: `ui-go/content/lib/components/ai/streamdown/svelte_streamdown.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/streamdown/svelte_streamdown.templ
- Validation: generate/check passed
- Notes:
  - Added safe pre-wrapped text fallback; full markdown block parser, plugins, link safety interception, and DOM sync need a Go/JS markdown pipeline.

### [~] `ui/src/lib/components/ai/tool/Tool.svelte`

- Target: `ui-go/content/lib/components/ai/tool/tool.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool/{types.go,tool.templ}
- Validation: generate/check passed
- Notes:
  - Ported root tool container with explicit View props; collapsible binding needs command/JS wiring.
  - Wired root content through shared Collapsible shell while preserving tool data attributes.

### [x] `ui/src/lib/components/ai/tool/ToolContent.svelte`

- Target: `ui-go/content/lib/components/ai/tool/tool_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool/tool_content.templ
- Validation: generate/check passed
- Notes:
  - Ported open-gated content wrapper and animation classes.

### [~] `ui/src/lib/components/ai/tool/ToolHeader.svelte`

- Target: `ui-go/content/lib/components/ai/tool/tool_header.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool/{types.go,tool_header.templ}
- Validation: generate/check passed
- Notes:
  - Ported header layout, derived name/verb split, status, and controls; collapse behavior needs command wiring.
  - Wired collapse-capable header label through shared CollapsibleTrigger shell.

### [~] `ui/src/lib/components/ai/tool/ToolHeaderControls.svelte`

- Target: `ui-go/content/lib/components/ai/tool/tool_header_controls.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool/tool_header_controls.templ, ui-go/content/lib/components/ui/collapsible/*.templ
- Validation: generate/check passed
- Notes:
  - Ported raw/collapse control buttons; toggle handlers need consumer command wiring.
  - Wired collapse button through shared CollapsibleTrigger shell; raw toggle remains a data-marked static button.

### [x] `ui/src/lib/components/ai/tool/ToolHeaderStatus.svelte`

- Target: `ui-go/content/lib/components/ai/tool/tool_header_status.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool/{types.go,tool_header_status.templ}
- Validation: generate/check passed
- Notes:
  - Ported effective state labels and status icons, including queued/running behavior.

### [x] `ui/src/lib/components/ai/tool/ToolInput.svelte`

- Target: `ui-go/content/lib/components/ai/tool/tool_input.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool/tool_input.templ
- Validation: generate/check passed
- Notes:
  - Ported parameters block; callers provide already-rendered JSON string.

### [x] `ui/src/lib/components/ai/tool/ToolOutput.svelte`

- Target: `ui-go/content/lib/components/ai/tool/tool_output.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool/tool_output.templ
- Validation: generate/check passed
- Notes:
  - Ported result/error rendering for string output and error text.

### [x] `ui/src/lib/components/ai/tool-renderers/ApplyPatchToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/apply_patch_tool_renderer.templ`
- Status: Ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/apply_patch.go`
  - `ui-go/content/lib/components/ai/tool-renderers/apply_patch_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported optimized patch summary, per-file operations, hunks, output entries, and error/unparsed-output fallback.
  - Uses explicit Go view props instead of Svelte tool renderer props/context; collapse/raw toggles remain data-attribute placeholders handled by surrounding ui-go behavior.

### [~] `ui/src/lib/components/ai/tool-renderers/AskUserQuestionToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/ask_user_question_tool_renderer.templ`
- Status: Partial shell ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/ask_user_question.go`
  - `ui-go/content/lib/components/ai/tool-renderers/ask_user_question_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported header, tool status/control chrome, question summaries, parsed answer summaries, output fallback, and error states.
  - Approval-requested state now embeds the Go wizard shell when question input is present; fetching pending questions and live answer submission remain client/API wiring tasks.

### [~] `ui/src/lib/components/ai/tool-renderers/AskUserQuestionWizard.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/ask_user_question_wizard.templ`
- Status: Partial shell ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/ask_user_question.go`
  - `ui-go/content/lib/components/ai/tool-renderers/ask_user_question_tool_renderer.templ`
  - `ui-go/content/lib/components/ai/tool-renderers/ask_user_question_wizard.templ`
  - `ui-go/content/lib/components/ai/message/message_response.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported the wizard layout shell: heading, streamdown-backed notes panel, multi-question step row, active-question options, Other option, and submit action styling.
  - Full parity still needs client-side state, auto-advance, dialog expansion, editable Other text, and async answer submission callbacks.

### [~] `ui/src/lib/components/ai/tool-renderers/BashToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/bash_tool_renderer.templ`
- Status: Partial shell ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/bash.go`
  - `ui-go/content/lib/components/ai/tool-renderers/bash_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported command header, credential-use badge/list, command card, background/timeout metadata, stdout/stderr/error rendering, exit-code badges, truncated/numbered stdout parsing, and unparsed-output fallback.
  - Full parity still needs clipboard copied-state and session credential assignment lookup because the Svelte renderer uses browser clipboard state plus API calls to resolve credential names and approved-use descriptions.

### [x] `ui/src/lib/components/ai/tool-renderers/EditToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/edit_tool_renderer.templ`
- Status: Ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/apply_patch.go`
  - `ui-go/content/lib/components/ai/tool-renderers/edit.go`
  - `ui-go/content/lib/components/ai/tool-renderers/edit_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported file header, path summary, replacement metadata, LCS-based inline diff preview, success/replacement output summary, edit error, and unparsed-output fallback.
  - Shared `shortenPath` helper was adjusted to leave non-Discobot paths unchanged.

### [x] `ui/src/lib/components/ai/tool-renderers/GlobToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/glob_tool_renderer.templ`
- Status: Ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/glob.go`
  - `ui-go/content/lib/components/ai/tool-renderers/glob_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported glob header, pattern/path summary, matched files list, content fallback, no-match state, and error rendering.

### [x] `ui/src/lib/components/ai/tool-renderers/GrepToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/grep_tool_renderer.templ`
- Status: Ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/grep.go`
  - `ui-go/content/lib/components/ai/tool-renderers/grep_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported search header, pattern/path/glob/mode summary, count card, structured matches, files list, content fallback, no-match state, and error rendering.

### [~] `ui/src/lib/components/ai/tool-renderers/OptimizedToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
- Status: Partial shell ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/optimized.go`
  - `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported raw-vs-optimized shell, default-open rules, title derivation, raw fallback, and dispatch to currently implemented Go optimized renderers.
  - Full registry parity depends on finishing the remaining specialized command/approval renderers, but the dispatcher shell now routes every implemented Go optimized renderer and falls back to raw content.

### [x] `ui/src/lib/components/ai/tool-renderers/ReadToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/read_tool_renderer.templ`
- Status: Ported
- Owner/task: sequential port
- Shared files touched:
  - `ui-go/content/lib/components/ai/tool-renderers/optimized.go`
  - `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
  - `ui-go/content/lib/components/ai/tool-renderers/read.go`
  - `ui-go/content/lib/components/ai/tool-renderers/read_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Ported read header image preview, file metadata, numbered/truncated text rendering, rich text extraction, image rendering, empty-content state, and error rendering.
  - Added Read/read dispatch to the Go optimized renderer.

### [~] `ui/src/lib/components/ai/tool-renderers/RequestCommitPullDiffFile.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/request_commit_pull_diff_file.templ`
- Status: Partial ported / enhanced with browser Pierre diff island
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/{request_commit_pull.go,request_commit_pull_diff_file.templ}`, `ui-go/assets/js/app.ts`, `ui-go/scripts/build-js.mjs`, `ui-go/content/root.templ`, `ui-go/package.json`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported the file metadata, status/addition/deletion badges, binary/no-preview states, and loading skeleton.
  - Added a no-JS unified patch fallback plus a Pierre diff mount payload so the browser can enhance textual patches into rich unified/split rendering.
  - Full parity still lacks the Svelte approval-selection callbacks around the rendered file.

### [~] `ui/src/lib/components/ai/tool-renderers/RequestCommitPullDiffViewer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/request_commit_pull_diff_viewer.templ`
- Status: Partial ported / enhanced with client-side diff style switching
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/{request_commit_pull_diff_viewer.templ,request_commit_pull_diff_file.templ}`, `ui-go/assets/js/app.ts`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported changed-file summary, diff-style controls, file rows, status badges, totals, and expandable per-file sections.
  - Diff-style controls now trigger the Pierre diff island to re-render mounted files as unified or split where textual patch data is available.
  - Matched the Svelte viewer-row fallback status label (`Changed`) while keeping the file-card fallback label as `Modified`.
  - Svelte approval tracking remains local client state; Go port still uses native `<details>` and disabled approval placeholders.

### [x] `ui/src/lib/components/ai/tool-renderers/RequestCommitPullNotesDialogContent.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/request_commit_pull_notes_dialog_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool-renderers/request_commit_pull_notes_dialog_content.templ, ui-go/content/lib/components/ui/dialog/*.templ
- Validation: generate/check passed
- Notes:
  - Wired notes content through shared DialogHeader/DialogTitle/DialogDescription primitives while preserving notes body; outer dialog open/approval workflow remains outside this content component.

### [x] `ui/src/lib/components/ai/tool-renderers/RequestCommitPullRawPatchDialogContent.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/request_commit_pull_raw_patch_dialog_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ai/tool-renderers/request_commit_pull_raw_patch_dialog_content.templ, ui-go/content/lib/components/ui/dialog/*.templ, ui-go/content/lib/components/ai/code-block/{types.go,code_block_copy_button.templ}
- Validation: generate/check passed
- Notes:
  - Wired raw patch content through shared DialogHeader/DialogTitle/DialogDescription primitives while preserving diff code block and copy button.
  - Added configurable Go code-block copy button labels so the raw patch button exposes the Svelte `Copy raw patch` aria/title text.

### [~] `ui/src/lib/components/ai/tool-renderers/RequestCommitPullToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/request_commit_pull_tool_renderer.templ`
- Status: Partial ported / approval API workflow pending
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/request_commit_pull.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported header, status controls, pending approval card, commit metadata/stats, approved/rejected/summary/waiting states, and optional inline diff/raw patch/notes sections.
  - Added preview-error retry/reject action placeholders to match the Svelte approval layout while API submission remains client-driven.
  - Wired `RequestCommitPull` into the Go optimized renderer dispatch.
  - Svelte fetches pending questions/previews and submits approve/reject answers through APIs; those dynamic workflows and modal open/close state remain pending in Go.

### [~] `ui/src/lib/components/ai/tool-renderers/RequestUserCredentialToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/request_user_credential_tool_renderer.templ`
- Status: Partial ported / credential API workflow pending
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/request_user_credential.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported header, rejected/granted states, approval-requested review card, sudo notice, requested credential details, justifications, and approved-use lists.
  - Enriched the granted-state shell with sudo-specific copy, internal-token labeling, and original request purpose derived from input metadata.
  - Wired `RequestUserCredential` into the Go optimized renderer dispatch.
  - Credential selection, custom secret entry, OAuth flow, validity controls, sudo token generation, and approve/deny submission remain pending on API-backed client state/forms.

### [x] `ui/src/lib/components/ai/tool-renderers/SkillToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/skill_tool_renderer.templ`
- Status: Ported
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/skill.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported skill header, parsed skill/args details, result rendering, completion empty state, error display, and unparsed-output fallback.
  - Wired `Skill` into the Go optimized renderer dispatch.

### [x] `ui/src/lib/components/ai/tool-renderers/TaskToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/task_tool_renderer.templ`
- Status: Ported
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/task.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported task header, subagent/background badge, description, prompt, runtime metadata, result rendering, empty launch state, error display, and unparsed-output fallback.
  - Wired `Task` into the Go optimized renderer dispatch.

### [~] `ui/src/lib/components/ai/tool-renderers/TodoWriteToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/todo_write_tool_renderer.templ`
- Status: Partial ported / raw toggle command unavailable
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/todo_write.go`, `ui-go/content/lib/components/ai/tool-renderers/todo_write_tool_renderer.templ`, `ui-go/content/lib/components/ai/tool-renderers/optimized.go`, `ui-go/content/lib/components/ai/tool-renderers/optimized_tool_renderer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported progress summary, current/up-next card, recent completed list, remaining work list, success/error states, and unparsed-output fallback.
  - Added optional `PreviousTodos` support so callers can label newly completed tasks as “Completed since last update”; without previous entries it falls back to the latest two completed tasks like Svelte.
  - Matched Svelte row labels: current work prefers active form, while completed and remaining rows use todo content as the primary label.
  - Raw toggle callback remains a future command/client wiring task.
  - Wired `TodoWrite` into the Go optimized renderer dispatch.

### [~] `ui/src/lib/components/ai/tool-renderers/WebFetchToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/web_fetch_tool_renderer.templ`
- Status: Partial ported / backend-client safety command needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/{web_fetch.go,web_fetch_tool_renderer.templ,optimized.go,optimized_tool_renderer.templ}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported header, URL/prompt summary, content/error/unparsed-output states, and optimized renderer dispatch.
  - Added explicit link aria/title metadata, `rel="noopener noreferrer"`, and `data-link-safety-url` on the Go header link as a hook for future safety interception.
  - Svelte uses `LinkSafetyState` and `LinkSafetyModal` before opening URLs; Go port currently uses a plain external anchor.
  - Record for later backend/client pass: implement reusable link-safety state/command or JS island before considering link-opening parity complete.

### [x] `ui/src/lib/components/ai/tool-renderers/WebSearchToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/web_search_tool_renderer.templ`
- Status: Ported
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/{web_search.go,web_search_tool_renderer.templ,optimized.go,optimized_tool_renderer.templ}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported header, query/domain summary, result list, content/error/unparsed-output states, and optimized renderer dispatch.

### [x] `ui/src/lib/components/ai/tool-renderers/WriteToolRenderer.svelte`

- Target: `ui-go/content/lib/components/ai/tool-renderers/write_tool_renderer.templ`
- Status: Ported
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/ai/tool-renderers/{write.go,write_tool_renderer.templ,optimized.go,optimized_tool_renderer.templ}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported header, file summary, character/line counts, content preview, output status, error/unparsed-output states, and optimized renderer dispatch.

### [~] `ui/src/lib/components/app/AppHeader.svelte`

- Target: `ui-go/content/lib/components/app/app_header.templ`
- Status: Partial ported / backend and client chrome state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/app_header.templ` (existing)
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Existing Go header ports desktop/mobile branding, session toolbar shell, new-session button markup, refresh button, settings button shell, update badge slot, and native right-window-controls slot.
  - Added header snapshot fields for refresh preference, update badge visibility, right-side native controls, and session-toolbar visibility so backend chrome state can drive the Svelte conditionals.
  - Record for later backend/client pass: wire new-session command, settings dialog open state, mobile sidebar toggle, and native window-control commands.

### [~] `ui/src/lib/components/app/AppKeyboardShortcuts.svelte`

- Target: `ui-go/content/lib/components/app/app_keyboard_shortcuts.templ`
- Status: Partial placeholder ported
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/app_keyboard_shortcuts.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a hidden placeholder component so the app-level shortcut slot exists in ui-go.
  - Record for later backend/client pass: implement global key handling, recent-thread switcher state, keyboard help dialog state, and commands for new session/thread plus session view toggles.
  - Full parity depends on command endpoints/state for `sessions.startNew`, `createThread`, `openThread`, mobile sidebar state, and terminal/desktop/editor/files/diff/services view toggles.

### [~] `ui/src/lib/components/app/AppMacWindowSpacer.svelte`

- Target: `ui-go/content/lib/components/app/app_mac_window_spacer.templ`
- Status: Partial ported / environment fullscreen state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{app_mac_window_spacer.templ,app_header.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported the left native-control spacer as an explicit read-model boolean and wired it into the desktop header area.
  - AppMacWindowSpacer now delegates to the shared Go LeftWindowControls shell, matching the Svelte component boundary.
  - Record for later backend/client pass: populate `ShowMacWindowSpacer` from native-window environment support, left-control side, and current fullscreen state.
  - Svelte listens for desktop-window resize/fullscreen changes; ui-go does not yet have that client/native bridge.

### [~] `ui/src/lib/components/app/AppSessionStatus.svelte`

- Target: `ui-go/content/lib/components/app/app_session_status.templ`
- Status: Partial ported / context lookup needs read-model state
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{app_session_status.templ,view_helpers.go}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported the rendered `SessionStatus` icon/label/tone behavior as an explicit-status templ component.
  - Fixed Go Lucide icon wrapper names to use available `MessageCircleQuestion` and `GitCommitHorizontal` icons.
  - Record for later backend/read-model pass: Svelte accepts `sessionId` and resolves status from `AppContext`; ui-go callers must provide the resolved display status until session lookup state is available.

### [~] `ui/src/lib/components/app/AppShell.svelte`

- Target: `ui-go/content/lib/components/app/app_shell.templ`
- Status: Partial ported / mobile and resizable shell state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/app_shell.templ`, `ui-go/content/lib/components/app/parts/{startup_tasks_banner.templ,startup_tasks_banner.go}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Wired the existing desktop shell scaffold to render the global shortcut slot and startup tasks banner slot.
  - Added a `StartupSnapshot` read model and templ startup banner with status badges, progress display, and byte/detail text.
  - Record for later backend/client pass: implement mobile sheet sidebar state, desktop resizable/collapsible sidebar persistence, mounted session list handling, and dismissible startup banner state.

### [~] `ui/src/lib/components/app/AppSidebar.svelte`

- Target: `ui-go/content/lib/components/app/app_sidebar.templ`
- Status: Partial ported / command dialogs and floating modes needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{app_sidebar.templ,session_item.templ}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Existing ui-go scaffold covers the sidebar chrome, recent threads, all sessions, workspace grouping, row actions, and empty state.
  - Updated session rows to reuse the explicit-status `AppSessionStatus` component for closer icon/tone parity.
  - Record for later backend/client pass: implement real select/new-session/new-thread/rename/delete/stop commands, dropdown menus, dialogs, workspace preference toggles, floating collapsed sidebar behavior, and mobile close-on-select behavior.

### [~] `ui/src/lib/components/app/AppThreadStatus.svelte`

- Target: `ui-go/content/lib/components/app/app_thread_status.templ`
- Status: Partial ported / context lookup needs read-model state
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{app_thread_status.templ,thread_item.templ,recent_thread_item.templ,view_helpers.go}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported the rendered thread status slot as an explicit-status wrapper around the shared session status visuals.
  - Wired regular and recent thread rows to use `AppThreadStatus` instead of the older dot-only helper.
  - Record for later backend/read-model pass: Svelte resolves status from session/thread context and local activity; ui-go callers must provide the resolved display status until that lookup state exists.

### [~] `ui/src/lib/components/app/ConversationComposer.svelte`

- Target: `ui-go/content/lib/components/app/conversation_composer.templ`
- Status: Partial ported / composer command and control state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{conversation_composer.templ,conversation_composer.go,thread_workspace.templ}`, `ui-go/content/lib/viewmodel/view.go`, `ui-go/cmd/ui-go/main.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model backed composer shell with disabled/error messaging, textarea, basic attachment/model/reasoning/service-tier controls, schedule placeholder, submit button, and pending workspace/hook placeholders.
  - Replaced the schedule placeholder text with the matching Clock icon shell while keeping scheduling disabled until command/state wiring exists.
  - Wired the composer shell into the thread workspace footer.
  - Record for later backend/client pass: implement draft persistence, file attachments, prompt submit/cancel commands, model/reasoning/service-tier selectors, schedule picker, prompt queue, autocomplete/slash commands, sandbox provider selection, and pending workspace setup integration.

### [~] `ui/src/lib/components/app/ConversationComposerSessionSetupStatus.svelte`

- Target: `ui-go/content/lib/components/app/conversation_composer_session_setup_status.templ`
- Status: Partial ported / pending workspace and auth command state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{conversation_composer_session_setup_status.templ,conversation_composer_session_setup_status.go,conversation_composer.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Ported the pending/not-ready session setup status strip with creating-session spinner, status label, error text, workspace loading text, validation messages, and GitHub auth prompt placeholder.
  - Wired it into the composer shell above pending submit errors.
  - Record for later backend/client pass: populate setup/readiness fields from session, workspace validation, and auth state; implement the GitHub credential flow command.

### [~] `ui/src/lib/components/app/ConversationCredentialsControl.svelte`

- Target: `ui-go/content/lib/components/app/conversation_credentials_control.templ`
- Status: Partial ported / credential save commands and dropdown state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{conversation_credentials_control.templ,conversation_credentials_control.go,conversation_composer.templ,conversation_composer.go}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model credential control with visible credential count, assignment list, runtime visibility controls for tools/console/services/hooks, all-runtimes checkbox placeholder, authorized use summaries, and manage placeholder.
  - Matched the Svelte control chrome more closely with KeyRound/Hammer/Terminal/Server/Webhook/Settings icons, accessibility labels, use-count chevron, and disabled remove-use action placeholders.
  - Wired it into the composer for non-pending sessions.
  - Record for later backend/client pass: load session credentials from real state, implement visibility toggles/save commands, authorized-use removal, global visibility warning dialog, credential change events, and credentials manager opening.

### [~] `ui/src/lib/components/app/ConversationHooksPanel.svelte`

- Target: `ui-go/content/lib/components/app/conversation_hooks_panel.templ`
- Status: Partial ported / hook dialog and rerun/download commands needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{conversation_hooks_panel.templ,conversation_hooks_panel.go,conversation_composer.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model hooks panel with passed count, hook status rows, status icons/tone/labels, rerun placeholder, and expandable output preview.
  - Added hook row IDs, keyboard/ARIA metadata, rerun/download aria-labels, and an output-dialog placeholder marker for the future selected-hook dialog shell.
  - Wired it into the composer shell for non-pending sessions.
  - Record for later backend/client pass: populate hook status/output from real session state, implement selected-hook modal state, rerun hook command, and full-log download command.

### [~] `ui/src/lib/components/app/ConversationPane.svelte`

- Target: `ui-go/content/lib/components/app/conversation_pane.templ`
- Status: Partial ported / rich message rendering and scroll state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{conversation_pane.templ,conversation_pane.go,thread_workspace.templ}`, `ui-go/content/lib/viewmodel/view.go`, `ui-go/cmd/ui-go/main.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added the Go conversation pane boundary with status badge passthrough, session/thread error banners, AI message primitive-backed message rows, empty state, and composer placement.
  - Matched the Svelte pane shell more closely with top-level error banner area, scrollbar-gutter scroll container, chat-width read-model field, selection-comment placement, and a scroll-to-bottom placeholder hook.
  - Replaced the inline thread workspace body with `ConversationPane`.
  - Record for later backend/client pass: implement chat message read model conversion, turn grouping, rich assistant/user parts, tool rendering integration, browser activity timeline, hook failure previews, selection comments, scroll restoration, auto-scroll, and retry actions.

### [~] `ui/src/lib/components/app/ConversationPromptHistoryDropdown.svelte`

- Target: `ui-go/content/lib/components/app/conversation_prompt_history_dropdown.templ`
- Status: Partial ported / keyboard and prompt mutation commands needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{conversation_prompt_history_dropdown.templ,conversation_prompt_history_dropdown.go,conversation_composer.templ,conversation_composer.go}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model prompt history dropdown with pinned and recent prompt sections, navigation hint header, selected-row state, and pin/delete placeholders.
  - Added Svelte-compatible `data-pinned-index`/`data-history-index` metadata plus explicit action/use aria labels for future keyboard and pointer wiring.
  - Wired the dropdown slot into the composer input wrapper.
  - Record for later backend/client pass: implement textarea keyboard navigation, draft insertion, outside-click close state, pin/unpin prompt commands, remove history command, and prompt history persistence.

### [~] `ui/src/lib/components/app/ConversationWorkspaceSelector.svelte`

- Target: `ui-go/content/lib/components/app/conversation_workspace_selector.templ`
- Status: Runtime parity for selector flow / local directory picker and branch selector still pending
- Owner/task: Discobot sequential port
- Shared files touched: `server/api/workspaces.go`, `ui-go/assets/js/app.ts`, `ui-go/content/lib/components/app/{conversation_workspace_selector.templ,conversation_workspace_selector.go,icons.templ,conversation_composer.templ}`, `ui-go/content/lib/viewmodel/view.go`, `ui-go/internal/{command,readmodel,server}`
- Validation: `cd ui-go && pnpm js:build && pnpm generate && go test ./...`; `cd server && go test ./api`
- Notes:
  - Added a read-model workspace selector for pending sessions with source input, return-to-dropdown icon button, local-directory button placeholder, autocomplete suggestions, existing/new workspace select, and branch selector shell.
  - Added suggestion listbox/option metadata, selected-suggestion read-model fields, and input autocomplete expanded state for future keyboard/pointer wiring.
  - Wired the selector into the pending composer controls.
  - Added `/ui/commands/composer-workspace` and browser-local event hooks so the existing workspace select, local/GitHub input transition, reset icon, validation debounce, and suggestion clicks follow the Svelte selector flow.
  - Added workspace validation to the Go API client and submit-time workspace resolution: existing workspaces are respected, local/GitHub inputs create workspaces after validation, and "Create New Workspace" submits without forcing the first existing workspace.
  - Updated the NativeSelect wrapper/classes and workspace icons to match the Svelte styling more closely, including GitHub vs generic git icon selection.
  - Record for later backend/client pass: implement native desktop directory picker, branch selection when re-enabled, auth credential launch flow, richer keyboard suggestion navigation, and setup-status message rendering for validation/auth details.

### [~] `ui/src/lib/components/app/CredentialsManager.svelte`

- Target: `ui-go/content/lib/components/app/credentials_manager.templ`
- Status: Partial ported / credential command and OAuth flows needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{credentials_manager.templ,credentials_manager.go}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model credentials manager shell with loading/error states, configured credential rows, runtime visibility indicators, inactive badges, credential count text, and add/edit/delete placeholders.
  - Added an editor/picker placeholder covering provider cards, optional name/description fields, disabled save/cancel controls, Svelte-like dialog ARIA/max-height scroll chrome, and inline editor error display.
  - Record for later backend/client pass: implement credential CRUD commands, OAuth authorization/polling/callback flows, custom env-var editor, bulk env-var paste dialog, credential type picker parity, scope picker, clipboard/open-url actions, and credentials-changed events.

### [~] `ui/src/lib/components/app/DockPanel.svelte`

- Target: `ui-go/content/lib/components/app/dock_panel.templ`
- Status: Partial ported / panel implementations and dock commands needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{dock_panel.templ,dock_panel.go,thread_workspace.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model dock panel boundary that preserves mounted panel slots and switches visibility for terminal, desktop, editor, files, diff review, and services.
  - Wired `ThreadWorkspace` to render `DockPanel` when the active dock kind is non-empty, otherwise it keeps the chat conversation pane.
  - Added placeholder chrome for close/maximize controls, files/diff counts, and service rows.
  - Added Svelte-compatible dock state metadata for active kind, maximized state, shifted window controls, mounted slots, and active slot flags.
  - Record for later backend/client pass: implement active-view commands, dock maximize state, VS Code file-open fallback, diff selection comments, service start/stop/open commands, mounted-panel lifecycle, and native window-control shifting.

### [~] `ui/src/lib/components/app/RecentThreadSwitcherDialog.svelte`

- Target: `ui-go/content/lib/components/app/recent_thread_switcher_dialog.templ`
- Status: Partial ported / keyboard switcher commands needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{recent_thread_switcher_dialog.templ,recent_thread_switcher_dialog.go,app_shell.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model recent-thread switcher overlay with header help text, selected-row styling, thread status badges, thread keys, and empty state.
  - Added dialog/listbox/option semantics, `aria-activedescendant`, stable selected-item IDs, and `aria-selected` metadata for future keyboard and scroll-into-view wiring.
  - Wired the overlay into the app shell using `HeaderSnapshot.ThreadSwitcher`.
  - Record for later backend/client pass: implement global shortcut state, hover selection, enter selection, escape close, selected item scroll-into-view, recent thread ordering, and command-backed navigation.

### [~] `ui/src/lib/components/app/SandboxProvidersManager.svelte`

- Target: `ui-go/content/lib/components/app/sandbox_providers_manager.templ`
- Status: Partial ported / sandbox provider commands and runtime controls needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{sandbox_providers_manager.templ,sandbox_providers_manager.go}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`; `cd /home/discobot/workspace/ui-go && go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint run --fix`
- Notes:
  - Added a read-model sandbox providers manager shell with error state, driver picker, provider form placeholder, runtime controls placeholder, default-provider selector shell, active provider rows, provider icons/monograms, and disabled command controls.
  - Added provider type/instance/config field read-model types and helper functions for provider names, descriptions, defaults, capabilities, and available driver filtering.
  - Added Svelte-compatible state metadata for current manager view, loading/saving state, and the sandbox-provider update event; added alert semantics for errors.
  - Added driver/type IDs, provider row metadata, aria labels for row actions, disabled switch semantics for enable/disable state, default-selector label/disabled option parity, runtime provider icon display, refresh spinner class parity, and config-field IDs/type/required/advanced metadata.
  - Record for later backend/client pass: implement provider refresh, create/update/delete, enable/disable, default selection, driver picker commands, credential-backed config fields, inline credential creation, icon picker/preview parity, provider resources/inspection controls, and `discobot:sandbox-providers-updated` events.

### [~] `ui/src/lib/components/app/SessionToolbar.svelte`

- Target: `ui-go/content/lib/components/app/session_toolbar.templ`
- Status: Partial ported / toolbar commands and IDE actions needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{session_toolbar.templ,session_toolbar.go,app_header.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model session toolbar with desktop/mobile view buttons, diff stats badge, primary/secondary command shell, preferred IDE menu shell, and services learn-more prompt placeholder.
  - Wired the header to render `SessionToolbarStack` instead of a plain session title placeholder.
  - Added toolbar state metadata for active view, busy/pending state, services count, VS Code availability, and diff file count; added `aria-pressed` view-toggle semantics and diff summary labels.
  - Added grouped secondary command menu rendering with command name/group metadata, active command spinner parity, IDE option ID/family metadata, and dialog semantics for the services learn-more prompt.
  - Record for later backend/client pass: implement active-view toggles, service learn-more prompt submission, command execution and credential dialog integration, dynamic command icons, IDE launch URLs/preferences, VS Code availability, mobile behavior, and SSH host/port URL construction.

### [~] `ui/src/lib/components/app/SessionToolbarStack.svelte`

- Target: `ui-go/content/lib/components/app/session_toolbar_stack.templ`
- Status: Partial ported / mounted-session stack state needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{session_toolbar_stack.templ,app_header.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added the stack wrapper slot and render path for the selected session toolbar read model.
  - Added Svelte-compatible stack metadata for selected session ID and current mounted-session count so later mounted-session preservation can target the wrapper.
  - Record for later backend/client pass: preserve per-mounted-session toolbar instances, selected-session lookup, and session context binding parity.

### [~] `ui/src/lib/components/app/SessionWorkspace.svelte`

- Target: `ui-go/content/lib/components/app/session_workspace.templ`
- Status: Partial ported / resizable layout and session lifecycle needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{session_workspace.templ,session_workspace.go}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Extended the existing session workspace wrapper with visible/hidden shell handling, configurable main class, chat-only mode, split dock mode, and maximized dock mode.
  - Preserved the thread workspace connection-only placeholder in maximized dock mode and routed non-chat active views through `DockPanel`.
  - Added Svelte-compatible workspace metadata for session ID, visibility, pending state, reserve-sidebar state, active layout mode, connection-only placeholder mode, dock region, and resizable pane group/handle/default-size/min-size settings.
  - Record for later backend/client pass: implement mounted session context lifecycle, selected thread keying, resizable pane persistence, exact `connection-only` thread mode, pending-session dock suppression, and client active-view state updates.

### [~] `ui/src/lib/components/app/SettingsDialog.svelte`

- Target: `ui-go/content/lib/components/app/settings_dialog.templ`
- Status: Partial ported / settings commands and support dialogs needed
- Owner/task: Discobot sequential port
- Shared files touched: `ui-go/content/lib/components/app/{settings_dialog.templ,settings_dialog.go,settings_dialog_style.go,app_header.templ}`, `ui-go/content/lib/viewmodel/view.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model settings dialog shell with Appearance, Chat, Providers, Update, and Credentials tabs; update badge; disabled controls; progress display; project cache section; and footer support/done controls.
  - Wired settings rendering into the app header and embedded the Go `SandboxProvidersManager` and `CredentialsManager` shells in their tabs.
  - Added dialog ARIA semantics, active-tab metadata, tablist/tab/tabpanel roles with stable IDs, labeled selects with option values, radiogroup/radio semantics for theme mode and recent-list presets, switch semantics, update status metadata, check button shell, progressbar ARIA, and support-info button label.
  - Record for later backend/client pass: implement dialog open/close commands, tab switching, preference persistence, model grouping/deduping, theme updates, update check/download/install/ignore/pre-release commands, project cache clearing confirmation, support info dialog, and exact alert/dialog interaction behavior.

### [~] `ui/src/lib/components/app/StartupGate.svelte`

- Target: `ui-go/content/lib/components/app/startup_gate.templ`
- Status: Partial ported / startup polling and auth commands needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/root.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model startup gate wrapper around the Go app shell with ready/hidden shell classes, auth overlay, startup overlay, progress, retry/API status, error display, startup task rows, and disabled background-continue control.
  - Preserved default Go UI visibility when no startup phase is provided so the existing sample shell stays visible.
  - Added startup phase/API/retry/active-task metadata, auth dialog semantics, overlay live-region labeling, progressbar ARIA, alert semantics for errors, task list metadata, per-task IDs/states/progress values, and a dismiss-overlay data hook.
  - Record for later backend/client pass: implement desktop/server config initialization, system status polling, current-user auth check, login URL construction, app refresh/project event connection, minimum visible/fade timing, details toggling, and dismiss-in-background command/state.

### [~] `ui/src/lib/components/app/SupportInfoDialog.svelte`

- Target: `ui-go/content/lib/components/app/support_info_dialog.templ`
- Status: Partial ported / support-info commands needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/settings_dialog.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a read-model support information dialog with loading, error, empty, and JSON display states plus disabled copy/download/close button shells.
  - Wired the dialog into the settings shell through `SettingsDialogSnapshot.SupportInfo`.
  - Added dialog ARIA semantics, status/has-JSON metadata, labeled content region, loading live status, error alert semantics, JSON code label, and copy/download/close data hooks.
  - Record for later backend/client pass: fetch support information on open, close through app UI state, copy JSON to clipboard, download JSON through the desktop/browser shell, show copied feedback, and manage lifecycle cleanup for timers.

### [~] `ui/src/lib/components/app/ThreadWorkspace.svelte`

- Target: `ui-go/content/lib/components/app/thread_workspace.templ`
- Status: Partial ported / thread context lifecycle needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/components/app/session_workspace.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Existing Go thread workspace now mirrors the Svelte boundary more closely by rendering only `ThreadWorkspaceHeader` and `ConversationPane`; dock composition stays in `SessionWorkspace`.
  - Preserved a connection-only placeholder in maximized dock mode without rendering duplicate conversation content.
  - Added thread workspace/header metadata for conversation mode, visibility, reserved sidebar space, title, and thread state; matched Svelte by gating `ConversationPane` rendering on `snapshot.Visible`.
  - Record for later backend/client pass: implement thread context creation/disposal, visible-thread start behavior, selected-thread title derivation, transitioning sandbox status title, exact connection-only behavior, and conversation-pane visibility gating.

### [~] `ui/src/lib/components/app/parts/ConversationComposerAttachmentButton.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_attachment_button.templ`
- Status: Partial ported / file picker behavior needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a pure composer attachment button part with hidden file input, paperclip trigger styling, and a disabled dropdown-menu shell for adding photos or files.
  - Replaced the composer’s text-only `Attach` placeholder with the new part.
  - Added stable file-input/menu IDs, disabled-state metadata, trigger menu ARIA, menu/menuitem roles, and data hooks for the hidden input and add-files action.
  - Record for later backend/client pass: wire file input refs, open-file-dialog behavior, file-list change handling, dropdown menu visibility/focus behavior, upload/attachment state, and disabled-state parity.

### [~] `ui/src/lib/components/app/parts/ConversationComposerAttachments.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_attachments.templ`
- Status: Partial ported / remove command needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added `ComposerAttachment` read-model data and a pure attachment-pill renderer matching the Svelte input-group addon placement above the textarea.
  - Rendered filename truncation, per-file IDs, and disabled remove buttons with `X` icons.
  - Added attached-files list/listitem semantics, attachment count metadata, filename title text for truncated pills, and per-file remove action data hooks.
  - Record for later backend/client pass: wire attachment staging, remove-by-ID command, file metadata beyond filenames, upload/progress/error states, and keyboard/focus behavior.

### [~] `ui/src/lib/components/app/parts/ConversationComposerHooksControl.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_hooks_control.templ`
- Status: Partial ported / expansion command needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a pure hooks control button that appears when hooks exist, shows running/failure/success icons, and displays the passed-hook count from the hooks read model.
  - Replaced the composer’s generic `Hooks` placeholder with the new control.
  - Added expanded-state ARIA, accessible hook count label, total/passed hook count metadata, and aggregate hook-state metadata for future panel toggling.
  - Record for later backend/client pass: wire expanded-state toggling, pending hook IDs/display-state derivation, live hook updates, accessible button state, and exact Button primitive/dropdown behavior.

### [~] `ui/src/lib/components/app/parts/ConversationComposerModelControl.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_model_control.templ`
- Status: Partial ported / model selection command needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`, `ui-go/content/lib/components/app/conversation_composer.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added composer model read-model fields and a pure model control shell with selected-model display, default model item, provider grouping, descriptions, and checkmark state.
  - Replaced the composer’s generic model button with the new model control and removed an unused helper surfaced by lint.
  - Added selected model/count metadata, trigger menu ARIA, stable menu ID, menu/menuitemradio roles, selected-state ARIA, and per-option model/provider data hooks.
  - Added Svelte-parity model deduping/sorting, enabled the trigger/menu with the ui-go JS island, and wired option clicks to `/ui/commands/composer-model`.
  - Model selection is now session-scoped, supports the default/null option, preserves backend thread model defaults until the user chooses an override, and updates reasoning/service-tier availability from selected model metadata.
  - Record for later backend/client pass: submit selected model/reasoning/service-tier through the real chat API and match Svelte's exact "clear next model when selecting the thread model" behavior once thread composer state is split from the rendered snapshot.

### [~] `ui/src/lib/components/app/parts/ConversationComposerQueueControl.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_queue_control.templ`
- Status: Partial ported / queue expansion state needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`, `ui-go/content/lib/components/app/parts/conversation_composer_queue_control.{go,templ}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a shared `PlanEntry` read-model type and a pure queue control button that appears when entries exist and shows completed/total counts with the check-circle icon.
  - Wired the existing queue-expanded read-model field into the composer control and added expanded-state ARIA plus completed/total count metadata for later toggle behavior.
  - Record for later backend/client pass: wire expanded-state toggling, prompt queue read model, queue panel placement, completed-status derivation, and prompt update/delete commands.

### [x] `ui/src/lib/components/app/parts/ConversationComposerReasoningControl.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_reasoning_control.templ`
- Status: Ported
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`, `ui-go/assets/js/app.ts`, `ui-go/internal/command/composer_reasoning_service_tier.go`, `ui-go/internal/readmodel/client_shell.go`, `ui-go/internal/server/server.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added reasoning read-model fields and a pure reasoning control shell with selected/default label resolution, default item, level items, descriptions, and checkmark state.
  - Replaced the composer’s plain reasoning label button with the new control.
  - Added selected/default/count metadata, trigger menu ARIA, stable menu ID, menu/menuitemradio roles, selected-state ARIA, and per-level data hooks.
  - Enabled the dropdown with open/close/Escape/outside-click behavior, selected-option focus on open, and option commands via `/ui/commands/composer-reasoning`.
  - Reasoning choices are session-scoped, preserve backend thread defaults until explicit user selection, and reset to model defaults when the model selection changes.
  - Record for later backend/client pass: submit selected model/reasoning/service-tier through the real chat API once composer submit leaves placeholder mode.

### [x] `ui/src/lib/components/app/parts/ConversationComposerServiceTierControl.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_service_tier_control.templ`
- Status: Ported
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`, `ui-go/assets/js/app.ts`, `ui-go/internal/command/composer_reasoning_service_tier.go`, `ui-go/internal/readmodel/client_shell.go`, `ui-go/internal/server/server.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added service-tier read-model fields and a pure service-tier control shell with Standard/default item, tier items, fast/priority label formatting, descriptions, secondary selected styling, and checkmark state.
  - Replaced the composer’s plain service-tier label button with the new control.
  - Added selected value/count metadata, trigger menu ARIA, stable menu ID, menu/menuitemradio roles, selected-state ARIA, and per-tier data hooks.
  - Enabled the dropdown with open/close/Escape/outside-click behavior, selected-option focus on open, and option commands via `/ui/commands/composer-service-tier`.
  - Service-tier choices are session-scoped, preserve backend thread defaults until explicit user selection, and reset to provider defaults when the model selection changes.
  - Record for later backend/client pass: submit selected model/reasoning/service-tier through the real chat API once composer submit leaves placeholder mode.

### [~] `ui/src/lib/components/app/parts/ConversationComposerSubmitButton.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_submit_button.templ`
- Status: Partial ported / press and hover behavior needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/components/app/conversation_composer.templ`, `ui-go/content/lib/components/app/conversation_composer.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a pure submit button part with submitted/streaming/error/default icons, button type switching, disabled state, and ARIA labels.
  - Replaced the composer’s text submit button and removed an unused text-label helper surfaced by lint.
  - Added explicit status/action, empty-input, pending, generating, disabled, and plus-on-hover metadata so a later client pass can distinguish submit, stop, and new-session behavior.
  - Record for later backend/client pass: wire stop-vs-submit press behavior, pending empty hover plus-icon behavior, new-session action, status updates, and exact input-group button styling.

### [~] `ui/src/lib/components/app/parts/ConversationComposerTextarea.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_composer_textarea.templ`
- Status: Partial ported / keyboard and autocomplete behavior needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a pure textarea part with draft value, disabled state, placeholder, field-sizing/max-height classes, and composer data marker.
  - Replaced the inline composer textarea with the new part.
  - Added stable textarea ID, message label, autocomplete/spellcheck attributes, draft length/empty metadata, attachment-count metadata, and data hooks for enter submit, shift-enter newline, paste-file handling, file mentions, slash commands, and prompt history.
  - Record for later backend/client pass: wire draft change commands, IME composition state, enter-to-submit, backspace-remove-last-attachment, paste-file handling, focus/selection APIs, file mention autocomplete, slash command autocomplete, and prompt history keyboard handling.

### [~] `ui/src/lib/components/app/parts/ConversationFileMentionDropdown.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_file_mention_dropdown.templ`
- Status: Partial ported / autocomplete behavior needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added file mention dropdown read-model types and a pure dropdown shell with header, loading, empty, selected-row, file/folder icons, and path rendering.
  - Mounted the dropdown in the composer relative container so it shares the textarea overlay boundary.
  - Added listbox/option semantics, stable dropdown and active-option IDs, `aria-activedescendant`, selected-state ARIA, loading/empty live regions, query/loading/count/selected-index metadata, and per-option path/type data hooks.
  - Record for later backend/client pass: detect active `@` mentions, debounce `searchSessionFiles`, abort stale requests, keyboard navigation/selection, textarea range replacement, outside-click close, scroll selected item into view, and pending-session autocomplete creation.

### [~] `ui/src/lib/components/app/parts/ConversationPromptQueuePanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_prompt_queue_panel.templ`
- Status: Partial ported / queue update commands needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added `QueuedPrompt` read-model data and a prompt queue panel shell with count header, prompt text/editing state, attachments/model/run-after metadata, and disabled move/edit/schedule/run/pause/delete controls.
  - Mounted the queue panel above the hooks panel for non-pending sessions.
  - Added region/list/listitem semantics, heading linkage, queue count metadata, per-entry saving/editing/index/attachment/model/run-after metadata, edit textarea label/data hook, action data hooks, action ARIA labels, and schedule popover trigger ARIA.
  - Record for later backend/client pass: derive queued prompt text/file counts from message parts, live relative run-after labels, edit textarea state, save/cancel, move positions, schedule picker popovers, pause/run-now/delete commands, saving state, and timer updates.

### [~] `ui/src/lib/components/app/parts/ConversationPromptSchedulePicker.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_prompt_schedule_picker.templ`
- Status: Partial ported / scheduling commands needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/components/app/parts/conversation_prompt_queue_panel.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added a pure schedule picker shell with quick choices, pause, custom datetime input, run-now, and save controls.
  - Embedded the picker in the prompt queue panel’s hidden schedule popover body for queued prompts with `runAfter` state.
  - Added group/heading semantics, current-run-after and disabled metadata, labeled custom datetime input, and action data hooks for quick later choices, pause, run-now, and save-custom.
  - Record for later backend/client pass: compute default local datetime values, parse/save custom times, quick-select relative times, build long pause date, run-now clearing, disabled/saving behavior, and popover open/close state.

### [~] `ui/src/lib/components/app/parts/ConversationQueuePanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_queue_panel.templ`
- Status: Partial ported / queue expansion behavior needed
- Owner/task: Go UI rewrite
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate`; `pnpm --dir /home/discobot/workspace/ui-go check`
- Notes:
  - Added plan-entry queue expansion fields and a pure todo queue panel with completed count, running spinner, completed dot, pending dot, line-through completed text, and max-height scroll container.
  - Wired the panel above prompt queue/hooks and wired `ConversationComposerQueueControl` into the composer controls using the same plan entries.
  - Added region/list/listitem semantics, heading linkage, expanded/count/completed metadata, per-entry ID/index/status data hooks, and hidden decorative status icons/dots from assistive tech.
  - Record for later backend/client pass: map plan entries from thread context, implement expanded-state toggling, keep queue/control state synchronized, and preserve exact plan entry keying/content semantics.

### [~] `ui/src/lib/components/app/parts/ConversationSelectionComment.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_selection_comment.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/conversation_pane.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added a `ConversationSelectionCommentSnapshot` and a pure templ shell for the floating Comment affordance and editor popover.
  - Wired the shell into the conversation scroll area with `data-conversation-root` and selection-comment data hooks for a later browser-selection island.
  - Added pending/submitting/error/snippet-length metadata, dialog semantics for the editor, labeled/described snippet content, textarea label, error alert semantics, and open button dialog ARIA.
  - Record for later backend/client pass: implement range ownership checks, mouseup/keyup/scroll listeners, viewport clamping, focus management, draft validation, submit command, selection clearing, and error handling parity.

### [~] `ui/src/lib/components/app/parts/ConversationSlashCommandDropdown.svelte`

- Target: `ui-go/content/lib/components/app/parts/conversation_slash_command_dropdown.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/conversation_composer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added slash-command read-model fields and a templ dropdown with loading, empty, sorted/filtered command suggestions, selected-row styling, and command data hooks.
  - Wired the dropdown into the composer overlay stack before file mentions and prompt history.
  - Added listbox/option semantics, stable dropdown and active-option IDs, `aria-activedescendant`, selected-state ARIA, loading/empty live regions, open/loading/count/selected-index metadata, and per-command description data hooks.
  - Record for later backend/client pass: load agent commands by session, sync query from textarea cursor, implement ArrowUp/ArrowDown/Tab/Enter/Escape handling, outside-click close, selected-row scroll-into-view, and command insertion into the draft.

### [~] `ui/src/lib/components/app/parts/CredentialEnvVarEditor.svelte`

- Target: `ui-go/content/lib/components/app/parts/credential_env_var_editor.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/credentials_manager.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added credential env-var row read-model fields and a templ editor with key/value inputs, stored-value replacement messaging, add/remove/update command hooks, and disabled command placeholders.
  - Wired the editor into the credentials manager editor shell when env-var rows are supplied.
  - Added row-count and per-row stored/replace/focus metadata, stable key/value input IDs, input labels, row group semantics, stored/help message hooks, per-input row IDs, secret-value metadata, and explicit add/remove/show/hide action hooks.
  - Record for later backend/client pass: implement row add/remove/update commands, paste parsing for `.env` blocks, focus-driven secret visibility, stored-value replacement toggles, validation, and save integration.

### [~] `ui/src/lib/components/app/parts/CredentialListItem.svelte`

- Target: `ui-go/content/lib/components/app/parts/credential_list_item.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/credentials_manager.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added a reusable credential list item with media/monogram rendering, title/subtitle text, inactive switch styling, edit/delete buttons, and credential action data hooks.
  - Switched the credentials manager list rendering to use the shared parts component.
  - Added auth-type, inactive, toggling, and deleting metadata, media/title/subtitle hooks, per-action labels, and explicit toggle/edit/delete action metadata.
  - Record for later backend/client pass: wire inactive toggle, edit/delete handlers, tooltip primitives, deletion/toggling pending state, provider icon image mapping, and any runtime visibility indicators outside this Svelte list-item boundary.

### [~] `ui/src/lib/components/app/parts/CredentialOAuthScopePicker.svelte`

- Target: `ui-go/content/lib/components/app/parts/credential_oauth_scope_picker.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/credentials_manager.go`
  - `ui-go/content/lib/components/app/credentials_manager.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added OAuth scope picker read-model types and a templ component for simple checkbox, bullet-summary, and advanced grouped scope layouts.
  - Wired the picker into the credentials manager editor shell when OAuth scope data is supplied.
  - Added mode and scope-count metadata, mode button group semantics with `aria-pressed`, reset/customize action hooks, labeled scope containers, summary hooks, stable checkbox/group IDs, per-scope enabled/access metadata, and set-enabled action hooks.
  - Record for later backend/client pass: mode switching, reset-to-defaults, scope enabled/disabled commands, provider default scope mapping, and OAuth save/auth integration.

### [~] `ui/src/lib/components/app/parts/CredentialOAuthWizardDialog.svelte`

- Target: `ui-go/content/lib/components/app/parts/credential_oauth_wizard_dialog.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/credentials_manager.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added OAuth wizard read-model fields and a templ dialog shell with sign-in flow selection, optional scope picker, redirect-code flow, device-code flow, copied-state messages, loading icons, and error display.
  - Mounted the wizard from `CredentialsManager` when the snapshot is open.
  - Added dialog semantics, flow/provider/start/polling/scope/error metadata, section and action hooks, flow `aria-pressed`, labeled redirect input, auth URL and device-code hooks, copy button labels, copied/polling live regions, and error alert semantics.
  - Record for later backend/client pass: dialog open/close, flow selection, OAuth auth URL creation/open/copy, pasted callback/code parsing, device-code polling, clipboard feedback timers, scope synchronization, and provider-specific wizard instances.

### [~] `ui/src/lib/components/app/parts/CredentialTypePicker.svelte`

- Target: `ui-go/content/lib/components/app/parts/credential_type_picker.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/credentials_manager.go`
  - `ui-go/content/lib/components/app/credentials_manager.templ`
  - `ui-go/content/lib/components/app/parts/credential_type_picker.{templ,go}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added a reusable credential type picker with grouped provider cards, image/monogram media, descriptions, and provider selection data hooks.
  - Replaced the inline provider picker shell in `CredentialsManager` with the shared parts component.
  - Added group/option count metadata, labeled group semantics, stable group heading IDs, per-option action/auth/label/description hooks, media/title/summary hooks, and explicit choose labels for later command wiring.
  - Record for later backend/client pass: wire provider selection commands, provider image mapping, keyboard/focus behavior, selected-provider state, and exact group labels/order from API metadata.

### [~] `ui/src/lib/components/app/parts/DesktopPanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/desktop_panel.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/dock_panel.templ`
  - `ui-go/content/lib/components/app/parts/desktop_panel.{templ,go}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added desktop panel read-model fields and a templ dock panel with title/status dot, maximize/close controls, connection overlay, reconnect affordance, and VNC host mount point.
  - Wired the desktop dock kind to render the real desktop panel shell instead of the generic placeholder.
  - Added region/status semantics, status/availability/maximized/dimensions/retry metadata, close/maximize/reconnect action hooks, live overlay message metadata, VNC host focusability, websocket subdomain metadata, and clipboard/paste bridge hooks.
  - Record for later backend/client pass: noVNC import/connect/retry lifecycle, authenticated desktop websocket URL construction, clipboard read/write bridge, Ctrl/Cmd+V forwarding, desktop-name events, reconnect action, and exact DockWindowChrome parity.

### [~] `ui/src/lib/components/app/parts/DevErrorOverlay.svelte`

- Target: `ui-go/content/lib/components/app/parts/dev_error_overlay.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/root.templ`
  - `ui-go/content/lib/components/app/parts/dev_error_overlay.{templ,go}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added development error overlay read-model fields and a templ overlay with latest-error list, message/stack rendering, clear/copy/dismiss data hooks, and copied state label.
  - Mounted the overlay at the root alongside the startup/app shell.
  - Added region/live semantics, error/max/copied count metadata, list/listitem roles, stable per-error title IDs, title/message/stack data hooks, copied-state metadata, and explicit clear/copy/dismiss action labels/hooks.
  - Record for later backend/client pass: browser dev-mode gating, console.error interception, global error/unhandled-rejection listeners, max-error trimming, clipboard copy fallback, copied reset timer, clear/dismiss commands, and JS island state updates.

### [~] `ui/src/lib/components/app/parts/DiffReviewFileRenderer.svelte`

- Target: `ui-go/content/lib/components/app/parts/diff_review_file_renderer.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/parts/diff_review_file_renderer.{templ,go}`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added diff review file read-model fields and a templ renderer that emits the existing `data-pierre-diff` mount payload for the browser Pierre diff island.
  - Included binary/empty states, virtualized host sizing, loading overlay, and unified patch fallback text with basic line coloring.
  - Added per-file path/status/commit/style/virtualized/rendering/approval/count metadata, region/status semantics, Pierre renderer kind hooks, virtualizer config hooks, and line-selection/gutter/hover option metadata for the future JS island.
  - Record for later backend/client pass: selected-line range state, render-state callbacks, Pierre instance lifecycle, virtualizer setup/cleanup, file identity/cache-key parity, and review comment selection integration.

### [~] `ui/src/lib/components/app/parts/DiffReviewPanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/diff_review_panel.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/parts/diff_review_panel.go`
  - `ui-go/content/lib/components/app/parts/diff_review_panel.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added diff review panel read-model fields, file rows, totals, approval/loading state, target/style controls, expanded file sections, embedded diff renderer, and selected-text comment popover shell.
  - Added panel-level target/style/refreshing/approval/count/maximized metadata, region/list/listitem semantics, target/style/refresh/approve/maximize/close action hooks, style `aria-pressed`, per-file expanded/loading/binary/virtualized/count metadata, file action labels, loading/error semantics, and selected-comment dialog metadata.
  - Record for later backend/client pass: diff target changes, refresh, approve-all/per-file approvals, ignore-whitespace state, open-file commands, maximized/close commands, selected-line callbacks, comment submission, approval persistence, and virtualized renderer lifecycle parity.

### [x] `ui/src/lib/components/app/parts/DiscobotBrand.svelte`

- Target: `ui-go/content/lib/components/app/parts/discobot_brand.templ`
- Status: Ported with current known parity
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/components/app/parts/discobot_brand.templ`
  - `ui-go/content/lib/components/app/parts/branding.go`
  - `ui-go/static/branding-assets/discobot-brand-gradient.svg`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Verified the Go component renders the gradient brand image with default title/height handling and passthrough classes matching the Svelte component.

### [x] `ui/src/lib/components/app/parts/DiscobotLogo.svelte`

- Target: `ui-go/content/lib/components/app/parts/discobot_logo.templ`
- Status: Ported with current known parity
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/components/app/parts/discobot_logo.templ`
  - `ui-go/content/lib/components/app/parts/branding.go`
  - `ui-go/static/branding-assets/discobot-logo-purple.svg`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Verified the Go component renders the purple logo image with default size/title handling and passthrough classes matching the Svelte component.

### [x] `ui/src/lib/components/app/parts/DiscobotWordmark.svelte`

- Target: `ui-go/content/lib/components/app/parts/discobot_wordmark.templ`
- Status: Ported with current known parity
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/components/app/parts/discobot_wordmark.templ`
  - `ui-go/content/lib/components/app/parts/branding.go`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Verified the Go component inlines the gradient wordmark SVG with default title handling and passthrough classes matching the Svelte component.
  - Known issue retained from current implementation: the static SVG gradient id can collide if multiple wordmark instances render on one page.

### [~] `ui/src/lib/components/app/parts/DockWindowChrome.svelte`

- Target: `ui-go/content/lib/components/app/parts/dock_window_chrome.templ`
- Status: Partial ported / command wiring needed
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/components/app/parts/dock_window_chrome.go`
  - `ui-go/content/lib/components/app/parts/dock_window_chrome.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added reusable dock chrome with shell/header/content classes, traffic-light close/minimize/maximize controls, maximized ring state, optional sidebar control offset, and title/actions/children slots.
  - Record for later backend/client pass: wire close/minimize/maximize click handlers, header double-click behavior with ignore selectors, and replace inline panel chrome in dock panel implementations where appropriate.

### [~] `ui/src/lib/components/app/parts/FilesPanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/files_panel.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/dock_panel.templ`
  - `ui-go/content/lib/components/app/parts/files_panel.go`
  - `ui-go/content/lib/components/app/parts/files_panel.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added files panel read-model fields and a templ shell with dock chrome, changed-only/refresh controls, open tabs, active file metadata/actions, conflict banner, text/image/PDF/binary preview states, and recursive explorer tree.
  - Wired the file dock slot to render the files panel and the diff-review slot to render the existing diff review panel.
  - Record for later backend/client pass: Monaco editor worker loading/theme setup, resizable pane persistence, file open/close/refresh/tree toggle, changed-only toggle, save/discard/download, markdown preview/split modes, dirty buffers, rename/delete dialogs, conflict resolution, context menus, and keyboard shortcuts.

### [~] `ui/src/lib/components/app/parts/KeyboardShortcutHelpDialog.svelte`

- Target: `ui-go/content/lib/components/app/parts/keyboard_shortcut_help_dialog.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/app_shell.templ`
  - `ui-go/content/lib/components/app/parts/keyboard_shortcut_help_dialog.go`
  - `ui-go/content/lib/components/app/parts/keyboard_shortcut_help_dialog.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added shortcut-help read-model fields and overlay rendering with shortcut rows, key groups, inline kbd styling, and empty state.
  - Mounted the overlay in the app shell beside the existing keyboard shortcut infrastructure.
  - Record for later backend/client pass: populate shortcuts from the global-shortcut registry and wire the open/close command state.

### [x] `ui/src/lib/components/app/parts/LeftWindowControls.svelte`

- Target: `ui-go/content/lib/components/app/parts/left_window_controls.templ`
- Status: Ported with current known parity
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/components/app/parts/left_window_controls.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the matching fixed-width macOS window-control spacer component.

### [~] `ui/src/lib/components/app/parts/MessageResponseWithCommand.svelte`

- Target: `ui-go/content/lib/components/app/parts/message_response_with_command.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/parts/message_response_with_command.go`
  - `ui-go/content/lib/components/app/parts/message_response_with_command.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added command-aware message response read-model fields and a templ wrapper for original command rows, command args, script suppressed-output notice, generated text expansion, and original-text fallback rendering.
  - Record for later backend/client pass: populate original command metadata from chat messages, map user renderable parts, and wire generated-text expand/collapse state.

### [~] `ui/src/lib/components/app/parts/ProjectInspectionTerminalDialog.svelte`

- Target: `ui-go/content/lib/components/app/parts/project_inspection_terminal_dialog.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/app_shell.templ`
  - `ui-go/content/lib/components/app/parts/project_inspection_terminal_dialog.go`
  - `ui-go/content/lib/components/app/parts/project_inspection_terminal_dialog.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added inspection terminal dialog read-model fields and a full-screen shell with title/description, connection status dot, overlay messages, reconnect hook, and terminal host mount point.
  - Mounted the dialog in the app shell.
  - Record for later backend/client pass: Ghostty init/theme/link providers, inspection terminal WebSocket URL/auth, input/resize messages, reconnect lifecycle, resize debounce, and open/close state wiring.

### [~] `ui/src/lib/components/app/parts/ProjectSettingsTabContent.svelte`

- Target: `ui-go/content/lib/components/app/parts/project_settings_tab_content.templ`
- Status: Partial read-model shell ported
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/viewmodel/view.go`
  - `ui-go/content/lib/components/app/parts/project_settings_tab_content.go`
  - `ui-go/content/lib/components/app/parts/project_settings_tab_content.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added project/provider settings read-model fields and a templ shell for resources, provider/CPU summary, memory/disk inputs, validation/save messages, refresh/save hooks, inspection availability, open-shell hook, and embedded inspection terminal dialog snapshot.
  - Used `RefreshCw` as the loading spinner because the Go lucide package does not expose `Loader2`.
  - Record for later backend/client pass: load resources/inspection on active tab, save resource changes, validate drafts live, enforce disk growth rules, refresh controls, open inspection terminal state, and integrate this tab into the settings/provider manager flows.

### [~] `ui/src/lib/components/app/parts/ProviderIcon.svelte`

- Target: `ui-go/content/lib/components/app/parts/provider_icon.templ`
- Status: Partial ported / simple-icons registry lookup deferred
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/components/app/parts/provider_icon.go`
  - `ui-go/content/lib/components/app/parts/provider_icon.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added ProviderIcon with base styling, image-reference rendering, sanitized inline SVG rendering, and initials fallback.
  - Record for later backend/client pass: provide simple-icons path data or a Go icon registry so `simple:*`/slug/title references render as branded SVGs instead of initials.

### [~] `ui/src/lib/components/app/parts/RightWindowControls.svelte`

- Target: `ui-go/content/lib/components/app/parts/right_window_controls.templ`
- Status: Partial ported / native window command wiring needed
- Owner/task: Current sequential porting pass
- Shared files touched:
  - `ui-go/content/lib/components/app/parts/right_window_controls.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go generate` and `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the Windows-style right-side minimize/maximize/close control shell with matching SVG glyphs and data hooks.
  - Record for later backend/client pass: wire controls to Tauri/native window minimize, maximize/unmaximize, and close commands, and mount conditionally for desktop window chrome.

### [~] `ui/src/lib/components/app/parts/ServicePanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/service_panel.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/dock_panel.templ`, `ui-go/content/lib/components/app/parts/service_panel.go`, `ui-go/content/lib/components/app/parts/service_panel.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added a services dock shell with service tabs, status/actions, preview URL bar, viewport controls, iframe mount, logs view, and unread-log affordance.
  - Record for later backend/client pass: wire service selection, start/stop/restart, preview refresh/path/viewport state, external open command, iframe load/error handling, and service output SSE streaming.

### [~] `ui/src/lib/components/app/parts/SessionCommandCredentialsDialog.svelte`

- Target: `ui-go/content/lib/components/app/parts/session_command_credentials_dialog.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/session_toolbar.templ`, `ui-go/content/lib/components/app/parts/session_command_credentials_dialog.go`, `ui-go/content/lib/components/app/parts/session_command_credentials_dialog.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the command credential approval dialog shell with request cards, credential selector, OAuth helper actions, validity preset/custom controls, custom credential inputs, and approve/deny actions.
  - Record for later backend/client pass: wire credential refresh events, option selection, validity mutations, custom secret handling, OAuth wizard launch, approve, and deny commands.

### [x] `ui/src/lib/components/app/parts/SessionStatus.svelte`

- Target: `ui-go/content/lib/components/app/parts/session_status.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/app/parts/session_status.go`, `ui-go/content/lib/components/app/parts/session_status.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the reusable pure status component with matching label formatting, tone mapping, spinning states, and status-specific icons.

### [~] `ui/src/lib/components/app/parts/StartupScreen.svelte`

- Target: `ui-go/content/lib/components/app/parts/startup_screen.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/parts/startup_screen.go`, `ui-go/content/lib/components/app/parts/startup_screen.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the standalone startup screen shell with brand mark, status/progress, API/retry details, detail toggle, dismiss hook, error block, step list, and shell-preview skeletons/cards.
  - Record for later consolidation pass: route StartupGate through this reusable component and wire detail toggle/dismiss actions to client state.

### [~] `ui/src/lib/components/app/parts/StartupTasksBanner.svelte`

- Target: `ui-go/content/lib/components/app/parts/startup_tasks_banner.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/app/parts/startup_tasks_banner.go`, `ui-go/content/lib/components/app/parts/startup_tasks_banner.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Existing Go port matches the task banner structure: active spinner, task count, dismiss button, task status badges, details, and progress bars.
  - Record for later client pass: wire local dismissed state / dismiss button behavior.

### [~] `ui/src/lib/components/app/parts/TerminalPanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/terminal_panel.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/dock_panel.templ`, `ui-go/content/lib/components/app/parts/terminal_panel.go`, `ui-go/content/lib/components/app/parts/terminal_panel.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the terminal dock shell with DockWindowChrome, session/status title, root toggle, copy SSH/pull command controls, overlay/reconnect state, and terminal host mount.
  - Record for later client pass: wire Ghostty initialization, WebSocket input/output, resize debouncing, root reconnect, clipboard copy/reset state, link providers, and terminal selection copy shortcuts.

### [x] `ui/src/lib/components/app/parts/ThreadStateBadge.svelte`

- Target: `ui-go/content/lib/components/app/parts/thread_state_badge.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/app/parts/thread_state_badge.go`, `ui-go/content/lib/components/app/parts/thread_state_badge.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the pure thread-state badge with label/tone mapping and class passthrough.

### [x] `ui/src/lib/components/app/parts/ThreadWorkspaceHeader.svelte`

- Target: `ui-go/content/lib/components/app/parts/thread_workspace_header.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/app/parts/thread_workspace_header.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the parts header shell with optional reserved sidebar spacer, title, and thread-state badge.

### [~] `ui/src/lib/components/app/parts/VSCodePanel.svelte`

- Target: `ui-go/content/lib/components/app/parts/vscode_panel.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/viewmodel/view.go`, `ui-go/content/lib/components/app/dock_panel.templ`, `ui-go/content/lib/components/app/parts/vscode_panel.go`, `ui-go/content/lib/components/app/parts/vscode_panel.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the editor dock shell with DockWindowChrome, loading/error overlays, retry hook, iframe mount, and unavailable-service fallback.
  - Record for later client pass: wire authenticated service URL construction against the actual API root, editor theme sync, iframe load/error state, refresh keys, and auth token handling.

### [x] `ui/src/lib/components/ui/alert/alert-description.svelte`

- Target: `ui-go/content/lib/components/ui/alert/alert_description.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert/alert.go`, `ui-go/content/lib/components/ui/alert/alert_description.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the pure alert description wrapper with matching slot and typography classes.

### [x] `ui/src/lib/components/ui/alert/alert-title.svelte`

- Target: `ui-go/content/lib/components/ui/alert/alert_title.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert/alert.go`, `ui-go/content/lib/components/ui/alert/alert_title.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the pure alert title wrapper with matching slot and title classes.

### [x] `ui/src/lib/components/ui/alert/alert.svelte`

- Target: `ui-go/content/lib/components/ui/alert/alert.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert/alert.go`, `ui-go/content/lib/components/ui/alert/alert.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the pure alert wrapper with default/destructive variants, alert role, data slot, and class passthrough.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-action.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_action.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_action.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the action button wrapper with default button styling and action data hook.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-cancel.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_cancel.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_cancel.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the cancel button wrapper with outline button styling and cancel data hook.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-content.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_content.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_content.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the content shell with portal, overlay, alertdialog role, modal attributes, and matching placement/animation classes.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-description.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_description.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_description.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the description wrapper with matching slot and muted text classes.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-footer.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_footer.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_footer.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the footer layout wrapper with responsive reversed button order.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-header.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_header.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_header.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the header layout wrapper with responsive text alignment.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-overlay.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_overlay.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_overlay.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the overlay wrapper with matching fixed backdrop and animation classes.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-portal.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_portal.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_portal.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the portal wrapper shell for alert-dialog content.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-title.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_title.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_title.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the title wrapper with matching slot and heading classes.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_trigger.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog_trigger.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the trigger button shell with trigger data hook.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [~] `ui/src/lib/components/ui/alert-dialog/alert-dialog.svelte`

- Target: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.templ`
- Status: Partial ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.go`, `ui-go/content/lib/components/ui/alert-dialog/alert_dialog.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the root shell that conditionally renders open dialog content.
  - Record for later client pass: wire Bits UI alert-dialog focus trapping, escape/outside interactions, trigger-root open state, and portal behavior where needed.

### [x] `ui/src/lib/components/ui/badge/badge.svelte`

- Target: `ui-go/content/lib/components/ui/badge/badge.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/badge/badge.go`, `ui-go/content/lib/components/ui/badge/badge.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the pure badge primitive with default, secondary, destructive, and outline variants plus span/link rendering.

### [x] `ui/src/lib/components/ui/button/button.svelte`

- Target: `ui-go/content/lib/components/ui/button/button.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: `ui-go/content/lib/components/ui/button/button.go`, `ui-go/content/lib/components/ui/button/button.templ`
- Validation: `pnpm --dir /home/discobot/workspace/ui-go check` passed
- Notes:
  - Added the pure button primitive with variant/size class mapping plus button/link rendering and disabled link affordances.

### [x] `ui/src/lib/components/ui/button-group/button-group-separator.svelte`

- Target: `ui-go/content/lib/components/ui/button-group/button_group_separator.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/button-group/button_group.go, ui-go/content/lib/components/ui/button-group/button_group_separator.templ
- Validation: generate/check passed
- Notes:
  - Added the separator wrapper with matching data slot, orientation attributes, and group separator styling.

### [x] `ui/src/lib/components/ui/button-group/button-group-text.svelte`

- Target: `ui-go/content/lib/components/ui/button-group/button_group_text.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/button-group/button_group.go, ui-go/content/lib/components/ui/button-group/button_group_text.templ
- Validation: generate/check passed
- Notes:
  - Added the text wrapper with matching muted/bordered styling and child slot rendering.

### [x] `ui/src/lib/components/ui/button-group/button-group.svelte`

- Target: `ui-go/content/lib/components/ui/button-group/button_group.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/button-group/button_group.go, ui-go/content/lib/components/ui/button-group/button_group.templ
- Validation: generate/check passed
- Notes:
  - Added the group wrapper with horizontal/vertical orientation class mapping and slot rendering.

### [x] `ui/src/lib/components/ui/card/card-action.svelte`

- Target: `ui-go/content/lib/components/ui/card/card_action.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/card/card.go, ui-go/content/lib/components/ui/card/card_action.templ
- Validation: generate/check passed
- Notes:
  - Added the pure card action slot wrapper and class mapping.

### [x] `ui/src/lib/components/ui/card/card-content.svelte`

- Target: `ui-go/content/lib/components/ui/card/card_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/card/card.go, ui-go/content/lib/components/ui/card/card_content.templ
- Validation: generate/check passed
- Notes:
  - Added the pure card content wrapper and class mapping.

### [x] `ui/src/lib/components/ui/card/card-description.svelte`

- Target: `ui-go/content/lib/components/ui/card/card_description.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/card/card.go, ui-go/content/lib/components/ui/card/card_description.templ
- Validation: generate/check passed
- Notes:
  - Added the pure card description paragraph wrapper and class mapping.

### [x] `ui/src/lib/components/ui/card/card-footer.svelte`

- Target: `ui-go/content/lib/components/ui/card/card_footer.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/card/card.go, ui-go/content/lib/components/ui/card/card_footer.templ
- Validation: generate/check passed
- Notes:
  - Added the pure card footer wrapper and class mapping.

### [x] `ui/src/lib/components/ui/card/card-header.svelte`

- Target: `ui-go/content/lib/components/ui/card/card_header.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/card/card.go, ui-go/content/lib/components/ui/card/card_header.templ
- Validation: generate/check passed
- Notes:
  - Added the pure card header wrapper and class mapping.

### [x] `ui/src/lib/components/ui/card/card-title.svelte`

- Target: `ui-go/content/lib/components/ui/card/card_title.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/card/card.go, ui-go/content/lib/components/ui/card/card_title.templ
- Validation: generate/check passed
- Notes:
  - Added the pure card title wrapper and class mapping.

### [x] `ui/src/lib/components/ui/card/card.svelte`

- Target: `ui-go/content/lib/components/ui/card/card.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/card/card.go, ui-go/content/lib/components/ui/card/card.templ
- Validation: generate/check passed
- Notes:
  - Added the root card wrapper with matching data slot, layout, color, border, and shadow classes.

### [x] `ui/src/lib/components/ui/checkbox/checkbox.svelte`

- Target: `ui-go/content/lib/components/ui/checkbox/checkbox.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/checkbox/checkbox.go, ui-go/content/lib/components/ui/checkbox/checkbox.templ
- Validation: generate/check passed
- Notes:
  - Added a static checkbox shell with checked/indeterminate states, matching data slot/classes, aria state, disabled handling, and indicator icons.

### [~] `ui/src/lib/components/ui/collapsible/collapsible-content.svelte`

- Target: `ui-go/content/lib/components/ui/collapsible/collapsible_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/collapsible/collapsible.go, ui-go/content/lib/components/ui/collapsible/collapsible_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static open-state content shell with matching data slot/state; Bits UI mount/animation behavior remains a client concern.

### [~] `ui/src/lib/components/ui/collapsible/collapsible-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/collapsible/collapsible_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/collapsible/collapsible.go, ui-go/content/lib/components/ui/collapsible/collapsible_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a trigger button shell with data slot/state, aria-expanded, disabled handling, and child slot rendering; open/close wiring remains client-side.

### [~] `ui/src/lib/components/ui/collapsible/collapsible.svelte`

- Target: `ui-go/content/lib/components/ui/collapsible/collapsible.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/collapsible/collapsible.go, ui-go/content/lib/components/ui/collapsible/collapsible.templ
- Validation: generate/check passed
- Notes:
  - Added the root wrapper with data slot/state and child slot rendering; Bits UI state binding behavior remains a client concern.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-checkbox-item.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_checkbox_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_checkbox_item.templ
- Validation: generate/check passed
- Notes:
  - Added a static checkbox item wrapper with checked/indeterminate state shell and check indicator; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-content.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static content wrapper with portal shell, state/side attributes, and popover classes; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-group-heading.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_group_heading.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_group_heading.templ
- Validation: generate/check passed
- Notes:
  - Added a static group heading wrapper with inset data attribute and label classes; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-group.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_group.templ
- Validation: generate/check passed
- Notes:
  - Added a static group wrapper with role and child slot rendering; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-item.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_item.templ
- Validation: generate/check passed
- Notes:
  - Added a static item wrapper with inset, variant, disabled state, and item classes; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-label.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_label.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_label.templ
- Validation: generate/check passed
- Notes:
  - Added a static label wrapper with inset data attribute and label classes; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-portal.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_portal.templ
- Validation: generate/check passed
- Notes:
  - Added a static portal placeholder wrapper for static rendering; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-radio-group.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_radio_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_radio_group.templ
- Validation: generate/check passed
- Notes:
  - Added a static radio group wrapper with value metadata and child slot rendering; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-radio-item.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_radio_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_radio_item.templ
- Validation: generate/check passed
- Notes:
  - Added a static radio item wrapper with checked state and circle indicator; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-separator.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_separator.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_separator.templ
- Validation: generate/check passed
- Notes:
  - Added a static separator wrapper with matching slot and border classes; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-shortcut.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_shortcut.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_shortcut.templ
- Validation: generate/check passed
- Notes:
  - Added a static shortcut span wrapper with muted tracking classes; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-sub-content.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_sub_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_sub_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static sub-content wrapper with state/side attributes and popover classes; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-sub-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_sub_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_sub_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a static sub-trigger wrapper with state/inset/disabled metadata and chevron icon; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-sub.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_sub.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_sub.templ
- Validation: generate/check passed
- Notes:
  - Added a static sub-menu root wrapper with open-state metadata; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a static trigger placeholder wrapper with matching data slot; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/context-menu/context-menu.svelte`

- Target: `ui-go/content/lib/components/ui/context-menu/context_menu.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/context-menu/context_menu.go, ui-go/content/lib/components/ui/context-menu/context_menu.templ
- Validation: generate/check passed
- Notes:
  - Added a static root wrapper with open-state metadata and child slot rendering; Bits UI portal, focus, positioning, keyboard, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-close.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_close.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_close.templ
- Validation: generate/check passed
- Notes:
  - Added a static close button wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-content.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static content wrapper with overlay, portal shell, close button, and dialog positioning classes; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-description.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_description.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_description.templ
- Validation: generate/check passed
- Notes:
  - Added a static description wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-footer.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_footer.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_footer.templ
- Validation: generate/check passed
- Notes:
  - Added a static footer wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-header.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_header.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_header.templ
- Validation: generate/check passed
- Notes:
  - Added a static header wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-overlay.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_overlay.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_overlay.templ
- Validation: generate/check passed
- Notes:
  - Added a static overlay wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-portal.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_portal.templ
- Validation: generate/check passed
- Notes:
  - Added a static portal placeholder wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-title.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_title.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_title.templ
- Validation: generate/check passed
- Notes:
  - Added a static title wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a static trigger button wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dialog/dialog.svelte`

- Target: `ui-go/content/lib/components/ui/dialog/dialog.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dialog/dialog.go, ui-go/content/lib/components/ui/dialog/dialog.templ
- Validation: generate/check passed
- Notes:
  - Added a static root open-state wrapper; Bits UI focus trapping, escape/outside close behavior, portal placement, and open-state wiring remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-checkbox-group.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_checkbox_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_checkbox_group.templ
- Validation: generate/check passed
- Notes:
  - Added a static checkbox group wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-checkbox-item.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_checkbox_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_checkbox_item.templ
- Validation: generate/check passed
- Notes:
  - Added a static checkbox item wrapper with check/minus indicator; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-content.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static content wrapper with portal shell, state/side attributes, side offset metadata, and popover classes; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-group-heading.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_group_heading.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_group_heading.templ
- Validation: generate/check passed
- Notes:
  - Added a static group heading wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-group.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_group.templ
- Validation: generate/check passed
- Notes:
  - Added a static group wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-item.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_item.templ
- Validation: generate/check passed
- Notes:
  - Added a static item wrapper with inset, variant, and disabled metadata; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-label.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_label.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_label.templ
- Validation: generate/check passed
- Notes:
  - Added a static label wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-portal.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_portal.templ
- Validation: generate/check passed
- Notes:
  - Added a static portal placeholder wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-radio-group.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_radio_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_radio_group.templ
- Validation: generate/check passed
- Notes:
  - Added a static radio group wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-radio-item.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_radio_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_radio_item.templ
- Validation: generate/check passed
- Notes:
  - Added a static radio item wrapper with circle indicator; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-separator.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_separator.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_separator.templ
- Validation: generate/check passed
- Notes:
  - Added a static separator wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-shortcut.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_shortcut.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_shortcut.templ
- Validation: generate/check passed
- Notes:
  - Added a static shortcut span wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-sub-content.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_sub_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_sub_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static sub-content wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-sub-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_sub_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_sub_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a static sub-trigger wrapper with chevron icon; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-sub.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_sub.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_sub.templ
- Validation: generate/check passed
- Notes:
  - Added a static sub-menu root wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a static trigger wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/dropdown-menu/dropdown-menu.svelte`

- Target: `ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.go, ui-go/content/lib/components/ui/dropdown-menu/dropdown_menu.templ
- Validation: generate/check passed
- Notes:
  - Added a static root open-state wrapper; Bits UI portal, focus, positioning, keyboard, selection, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/hover-card/hover-card-content.svelte`

- Target: `ui-go/content/lib/components/ui/hover-card/hover_card_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/hover-card/hover_card.go, ui-go/content/lib/components/ui/hover-card/hover_card_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static content wrapper with portal shell, state/side/align/offset metadata, and popover classes; Bits UI hover timing, portal placement, positioning, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/hover-card/hover-card-portal.svelte`

- Target: `ui-go/content/lib/components/ui/hover-card/hover_card_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/hover-card/hover_card.go, ui-go/content/lib/components/ui/hover-card/hover_card_portal.templ
- Validation: generate/check passed
- Notes:
  - Added a static portal placeholder wrapper; Bits UI hover timing, portal placement, positioning, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/hover-card/hover-card-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/hover-card/hover_card_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/hover-card/hover_card.go, ui-go/content/lib/components/ui/hover-card/hover_card_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a static trigger wrapper; Bits UI hover timing, portal placement, positioning, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/hover-card/hover-card.svelte`

- Target: `ui-go/content/lib/components/ui/hover-card/hover_card.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/hover-card/hover_card.go, ui-go/content/lib/components/ui/hover-card/hover_card.templ
- Validation: generate/check passed
- Notes:
  - Added a static root open-state wrapper; Bits UI hover timing, portal placement, positioning, and open-state behavior remain client-side concerns.

### [x] `ui/src/lib/components/ui/icons/CursorIcon.svelte`

- Target: `ui-go/content/lib/components/ui/icons/cursor_icon.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/icons/icons.go, ui-go/content/lib/components/ui/icons/cursor_icon.templ
- Validation: generate/check passed
- Notes:
  - Added the inline simple-icons Cursor SVG with currentColor fill, title fallback, aria label, and class merging.

### [x] `ui/src/lib/components/ui/icons/GithubIcon.svelte`

- Target: `ui-go/content/lib/components/ui/icons/github_icon.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/icons/icons.go, ui-go/content/lib/components/ui/icons/github_icon.templ
- Validation: generate/check passed
- Notes:
  - Added the inline simple-icons GitHub SVG with currentColor fill, title fallback, aria label, and class merging.

### [x] `ui/src/lib/components/ui/input/input.svelte`

- Target: `ui-go/content/lib/components/ui/input/input.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/input/input.go, ui-go/content/lib/components/ui/input/input.templ
- Validation: generate/check passed
- Notes:
  - Added the input wrapper with file/non-file class mapping, data-slot defaulting, value/placeholder/name attributes, disabled/required/readonly, and aria-invalid support.

### [x] `ui/src/lib/components/ui/input-group/input-group-addon.svelte`

- Target: `ui-go/content/lib/components/ui/input-group/input_group_addon.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/input-group/input_group.go, ui-go/content/lib/components/ui/input-group/input_group_addon.templ
- Validation: generate/check passed
- Notes:
  - Added the addon wrapper with align variants with matching data slots and class mapping.

### [~] `ui/src/lib/components/ui/input-group/input-group-button.svelte`

- Target: `ui-go/content/lib/components/ui/input-group/input_group_button.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/input-group/input_group.go, ui-go/content/lib/components/ui/input-group/input_group_button.templ
- Validation: generate/check passed
- Notes:
  - Added the button wrapper with size variants with matching data slots and class mapping. Browser value binding and dependency composition remain client-side/integration concerns.

### [~] `ui/src/lib/components/ui/input-group/input-group-input.svelte`

- Target: `ui-go/content/lib/components/ui/input-group/input_group_input.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/input-group/input_group.go, ui-go/content/lib/components/ui/input-group/input_group_input.templ
- Validation: generate/check passed
- Notes:
  - Added the input control wrapper with matching data slots and class mapping. Browser value binding and dependency composition remain client-side/integration concerns.

### [x] `ui/src/lib/components/ui/input-group/input-group-text.svelte`

- Target: `ui-go/content/lib/components/ui/input-group/input_group_text.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/input-group/input_group.go, ui-go/content/lib/components/ui/input-group/input_group_text.templ
- Validation: generate/check passed
- Notes:
  - Added the text span wrapper with matching data slots and class mapping.

### [~] `ui/src/lib/components/ui/input-group/input-group-textarea.svelte`

- Target: `ui-go/content/lib/components/ui/input-group/input_group_textarea.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/input-group/input_group.go, ui-go/content/lib/components/ui/input-group/input_group_textarea.templ
- Validation: generate/check passed
- Notes:
  - Added the textarea control wrapper with matching data slots and class mapping. Browser value binding and dependency composition remain client-side/integration concerns.

### [x] `ui/src/lib/components/ui/input-group/input-group.svelte`

- Target: `ui-go/content/lib/components/ui/input-group/input_group.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/input-group/input_group.go, ui-go/content/lib/components/ui/input-group/input_group.templ
- Validation: generate/check passed
- Notes:
  - Added the root group wrapper with focus/error/alignment classes with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-actions.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_actions.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_actions.templ
- Validation: generate/check passed
- Notes:
  - Added the pure actions wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-content.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_content.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_content.templ
- Validation: generate/check passed
- Notes:
  - Added the pure content wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-description.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_description.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_description.templ
- Validation: generate/check passed
- Notes:
  - Added the pure description wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-footer.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_footer.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_footer.templ
- Validation: generate/check passed
- Notes:
  - Added the pure footer wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-group.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_group.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_group.templ
- Validation: generate/check passed
- Notes:
  - Added the pure group wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-header.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_header.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_header.templ
- Validation: generate/check passed
- Notes:
  - Added the pure header wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-media.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_media.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_media.templ
- Validation: generate/check passed
- Notes:
  - Added the pure media wrapper with variant mapping with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-separator.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_separator.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_separator.templ
- Validation: generate/check passed
- Notes:
  - Added the pure separator wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item-title.svelte`

- Target: `ui-go/content/lib/components/ui/item/item_title.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item_title.templ
- Validation: generate/check passed
- Notes:
  - Added the pure title wrapper with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/item/item.svelte`

- Target: `ui-go/content/lib/components/ui/item/item.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/item/item.go, ui-go/content/lib/components/ui/item/item.templ
- Validation: generate/check passed
- Notes:
  - Added the pure root item wrapper with variant/size mapping with matching data slots and class mapping.

### [x] `ui/src/lib/components/ui/kbd/kbd-group.svelte`

- Target: `ui-go/content/lib/components/ui/kbd/kbd_group.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/kbd/kbd.go, ui-go/content/lib/components/ui/kbd/kbd_group.templ
- Validation: generate/check passed
- Notes:
  - Added the pure kbd group wrapper with matching data slot and class mapping.

### [x] `ui/src/lib/components/ui/kbd/kbd.svelte`

- Target: `ui-go/content/lib/components/ui/kbd/kbd.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/kbd/kbd.go, ui-go/content/lib/components/ui/kbd/kbd.templ
- Validation: generate/check passed
- Notes:
  - Added the pure kbd wrapper with tooltip-aware classes, SVG sizing, and child slot rendering.

### [x] `ui/src/lib/components/ui/label/label.svelte`

- Target: `ui-go/content/lib/components/ui/label/label.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/label/label.go, ui-go/content/lib/components/ui/label/label.templ
- Validation: generate/check passed
- Notes:
  - Added the label wrapper with matching data slot, class mapping, `for` support, and child slot rendering.

### [x] `ui/src/lib/components/ui/native-select/native-select-opt-group.svelte`

- Target: `ui-go/content/lib/components/ui/native-select/native_select_opt_group.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/native-select/native_select_opt_group.templ
- Validation: generate/check passed
- Notes:
  - Added the native optgroup wrapper with data slot, label/disabled attributes, and child slot rendering.

### [x] `ui/src/lib/components/ui/native-select/native-select-option.svelte`

- Target: `ui-go/content/lib/components/ui/native-select/native_select_option.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/native-select/native_select_option.templ
- Validation: generate/check passed
- Notes:
  - Added the native option wrapper with data slot, value/selected/disabled attributes, and child slot rendering.

### [x] `ui/src/lib/components/ui/native-select/native-select.svelte`

- Target: `ui-go/content/lib/components/ui/native-select/native_select.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/native-select/native_select.go, ui-go/content/lib/components/ui/native-select/native_select.templ
- Validation: generate/check passed
- Notes:
  - Added the native select wrapper with matching wrapper/select/icon slots, class mapping, value/name/disabled/required/invalid attributes, and chevron icon.

### [~] `ui/src/lib/components/ui/popover/popover-close.svelte`

- Target: `ui-go/content/lib/components/ui/popover/popover_close.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/popover/popover.go, ui-go/content/lib/components/ui/popover/popover_close.templ
- Validation: generate/check passed
- Notes:
  - Added a static close button wrapper; Bits UI portal, focus, positioning, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/popover/popover-content.svelte`

- Target: `ui-go/content/lib/components/ui/popover/popover_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/popover/popover.go, ui-go/content/lib/components/ui/popover/popover_content.templ
- Validation: generate/check passed
- Notes:
  - Added a static content wrapper with portal shell, state/side/align/offset metadata, and popover classes; Bits UI portal, focus, positioning, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/popover/popover-portal.svelte`

- Target: `ui-go/content/lib/components/ui/popover/popover_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/popover/popover.go, ui-go/content/lib/components/ui/popover/popover_portal.templ
- Validation: generate/check passed
- Notes:
  - Added a static portal placeholder wrapper; Bits UI portal, focus, positioning, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/popover/popover-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/popover/popover_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/popover/popover.go, ui-go/content/lib/components/ui/popover/popover_trigger.templ
- Validation: generate/check passed
- Notes:
  - Added a static trigger button wrapper; Bits UI portal, focus, positioning, and open-state behavior remain client-side concerns.

### [~] `ui/src/lib/components/ui/popover/popover.svelte`

- Target: `ui-go/content/lib/components/ui/popover/popover.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/popover/popover.go, ui-go/content/lib/components/ui/popover/popover.templ
- Validation: generate/check passed
- Notes:
  - Added a static root open-state wrapper; Bits UI portal, focus, positioning, and open-state behavior remain client-side concerns.

### [x] `ui/src/lib/components/ui/progress/progress.svelte`

- Target: `ui-go/content/lib/components/ui/progress/progress.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/progress/progress.go, ui-go/content/lib/components/ui/progress/progress.templ
- Validation: generate/check passed
- Notes:
  - Added the progress wrapper with matching slots/classes, aria progress attributes, max/value clamping, and translated indicator style.

### [~] `ui/src/lib/components/ui/resizable/resizable-handle.svelte`

- Target: `ui-go/content/lib/components/ui/resizable/resizable_handle.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/resizable/resizable.go, ui-go/content/lib/components/ui/resizable/resizable_handle.templ
- Validation: generate/check passed
- Notes:
  - Added the resizable handle shell with direction metadata, classes, optional grip handle, and separator role; Paneforge drag/resize behavior remains client-side.

### [~] `ui/src/lib/components/ui/resizable/resizable-pane-group.svelte`

- Target: `ui-go/content/lib/components/ui/resizable/resizable_pane_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/resizable/resizable.go, ui-go/content/lib/components/ui/resizable/resizable_pane_group.templ
- Validation: generate/check passed
- Notes:
  - Added the pane group shell with direction metadata and class mapping; Paneforge layout state and resizing remain client-side.

### [~] `ui/src/lib/components/ui/select/select-content.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-group-heading.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_group_heading.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-group.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-item.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-label.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_label.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-portal.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-scroll-down-button.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_scroll_down_button.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-scroll-up-button.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_scroll_up_button.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-separator.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_separator.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/select/select_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [~] `ui/src/lib/components/ui/select/select.svelte`

- Target: `ui-go/content/lib/components/ui/select/select.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/select/select.go, ui-go/content/lib/components/ui/select/*.templ
- Validation: generate/check passed
- Notes:
  - Added static select shell preserving classes, slots, icons, and state metadata; Bits UI selection, keyboard, focus, and positioning behavior remain client-side.

### [x] `ui/src/lib/components/ui/separator/separator.svelte`

- Target: `ui-go/content/lib/components/ui/separator/separator.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/separator/{separator.go,separator.templ}
- Validation: generate/check passed
- Notes:
  - Added static separator wrapper with orientation metadata, default data-slot, and Svelte class mapping.

### [~] `ui/src/lib/components/ui/sheet/sheet-close.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_close.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-content.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-description.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_description.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-footer.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_footer.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-header.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_header.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-overlay.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_overlay.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-portal.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-title.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_title.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [~] `ui/src/lib/components/ui/sheet/sheet.svelte`

- Target: `ui-go/content/lib/components/ui/sheet/sheet.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sheet/sheet.go, ui-go/content/lib/components/ui/sheet/*.templ
- Validation: generate/check passed
- Notes:
  - Added static sheet wrapper preserving side variants, overlay/content/title/description/header/footer classes, close icon, and state metadata; Bits UI portal, focus trap, and open/close behavior remain client-side.

### [x] `ui/src/lib/components/ui/skeleton/skeleton.svelte`

- Target: `ui-go/content/lib/components/ui/skeleton/skeleton.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/skeleton/{skeleton.go,skeleton.templ}
- Validation: generate/check passed
- Notes:
  - Added static skeleton div with pulse/background/rounded class mapping.

### [~] `ui/src/lib/components/ui/sonner/sonner.svelte`

- Target: `ui-go/content/lib/components/ui/sonner/sonner.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/sonner/{sonner.go,sonner.templ}
- Validation: generate/check passed
- Notes:
  - Added static toaster mount preserving class, CSS variables, and theme metadata; svelte-sonner runtime toast behavior remains client-side.

### [x] `ui/src/lib/components/ui/spinner/spinner.svelte`

- Target: `ui-go/content/lib/components/ui/spinner/spinner.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/spinner/{spinner.go,spinner.templ}
- Validation: generate/check passed
- Notes:
  - Added LoaderCircle status icon with loading aria-label and spin class mapping.

### [~] `ui/src/lib/components/ui/split-dropdown-button/split-dropdown-button.svelte`

- Target: `ui-go/content/lib/components/ui/split-dropdown-button/split_dropdown_button.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/split-dropdown-button/{split_dropdown_button.go,split_dropdown_button.templ}
- Validation: generate/check passed
- Notes:
  - Added static split button layout with primary action, dropdown trigger, chevron sizing, and menu content shell; full dropdown behavior follows the shared dropdown-menu client runtime.

### [~] `ui/src/lib/components/ui/switch/switch.svelte`

- Target: `ui-go/content/lib/components/ui/switch/switch.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/switch/{switch.go,switch.templ}
- Validation: generate/check passed
- Notes:
  - Added static switch button and thumb with checked state, aria metadata, disabled support, and class mapping; Bits UI checked binding remains client-side.

### [~] `ui/src/lib/components/ui/tabs/tabs-content.svelte`

- Target: `ui-go/content/lib/components/ui/tabs/tabs_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tabs/tabs.go, ui-go/content/lib/components/ui/tabs/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tabs content panel with value/state metadata and hidden inactive state; Bits UI tab selection behavior remains client-side.

### [~] `ui/src/lib/components/ui/tabs/tabs-list.svelte`

- Target: `ui-go/content/lib/components/ui/tabs/tabs_list.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tabs/tabs.go, ui-go/content/lib/components/ui/tabs/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tablist shell with Svelte class mapping; Bits UI roving focus remains client-side.

### [~] `ui/src/lib/components/ui/tabs/tabs-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/tabs/tabs_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tabs/tabs.go, ui-go/content/lib/components/ui/tabs/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tab trigger button with value/state/aria metadata and class mapping; Bits UI selection and keyboard behavior remain client-side.

### [~] `ui/src/lib/components/ui/tabs/tabs.svelte`

- Target: `ui-go/content/lib/components/ui/tabs/tabs.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tabs/tabs.go, ui-go/content/lib/components/ui/tabs/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tabs root with value metadata and layout classes; Bits UI value binding remains client-side.

### [x] `ui/src/lib/components/ui/textarea/textarea.svelte`

- Target: `ui-go/content/lib/components/ui/textarea/textarea.templ`
- Status: Ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/textarea/{textarea.go,textarea.templ}
- Validation: generate/check passed
- Notes:
  - Added textarea primitive with default data-slot, value, placeholder/name, disabled/required/readonly/invalid attributes, and Svelte class mapping.

### [~] `ui/src/lib/components/ui/toggle/toggle.svelte`

- Target: `ui-go/content/lib/components/ui/toggle/toggle.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/toggle/{toggle.go,toggle.templ}
- Validation: generate/check passed
- Notes:
  - Added static toggle button with variant/size classes, pressed state, aria metadata, and disabled support; Bits UI pressed binding remains client-side.

### [~] `ui/src/lib/components/ui/toggle-group/toggle-group-item.svelte`

- Target: `ui-go/content/lib/components/ui/toggle-group/toggle_group_item.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/toggle-group/toggle_group.go, ui-go/content/lib/components/ui/toggle-group/*.templ
- Validation: generate/check passed
- Notes:
  - Added static toggle group item with value/state/variant/size/spacing metadata and toggle classes; Bits UI group selection remains client-side.

### [~] `ui/src/lib/components/ui/toggle-group/toggle-group.svelte`

- Target: `ui-go/content/lib/components/ui/toggle-group/toggle_group.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/toggle-group/toggle_group.go, ui-go/content/lib/components/ui/toggle-group/*.templ
- Validation: generate/check passed
- Notes:
  - Added static toggle group root with value, variant, size, spacing metadata, gap style, and group classes; Bits UI value binding remains client-side.

### [~] `ui/src/lib/components/ui/tooltip/tooltip-content.svelte`

- Target: `ui-go/content/lib/components/ui/tooltip/tooltip_content.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tooltip/tooltip.go, ui-go/content/lib/components/ui/tooltip/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tooltip content portal shell with side/offset metadata, content classes, and arrow classes; Bits UI positioning/visibility remains client-side.

### [~] `ui/src/lib/components/ui/tooltip/tooltip-portal.svelte`

- Target: `ui-go/content/lib/components/ui/tooltip/tooltip_portal.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tooltip/tooltip.go, ui-go/content/lib/components/ui/tooltip/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tooltip portal wrapper; real portal behavior remains client-side.

### [~] `ui/src/lib/components/ui/tooltip/tooltip-provider.svelte`

- Target: `ui-go/content/lib/components/ui/tooltip/tooltip_provider.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tooltip/tooltip.go, ui-go/content/lib/components/ui/tooltip/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tooltip provider wrapper; provider timing behavior remains client-side.

### [~] `ui/src/lib/components/ui/tooltip/tooltip-trigger.svelte`

- Target: `ui-go/content/lib/components/ui/tooltip/tooltip_trigger.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tooltip/tooltip.go, ui-go/content/lib/components/ui/tooltip/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tooltip trigger wrapper; hover/focus trigger behavior remains client-side.

### [~] `ui/src/lib/components/ui/tooltip/tooltip.svelte`

- Target: `ui-go/content/lib/components/ui/tooltip/tooltip.templ`
- Status: Partial shell ported
- Owner/task: Port each file sequentially
- Shared files touched: ui-go/content/lib/components/ui/tooltip/tooltip.go, ui-go/content/lib/components/ui/tooltip/*.templ
- Validation: generate/check passed
- Notes:
  - Added static tooltip root with open state metadata; Bits UI open binding remains client-side.
