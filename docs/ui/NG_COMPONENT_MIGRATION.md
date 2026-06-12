# Ng Component Migration Tracker

This tracks migration of `ui/src/lib/components/app/**/*.svelte` from the
legacy `$lib/context` / `$lib/store` runtime to `NgContext`.

For the remaining cleanup of the newer `ui/src/lib/context/runtime` bridge, see
`docs/ui/NG_RUNTIME_BRIDGE_DELETION_PLAN.md`.

Status meanings:

- **Migrated**: component uses `NgContext` or no longer depends on legacy
  context/store APIs.
- **Legacy**: component still imports or consumes the legacy context/store or
  legacy context command modules.
- **Pure / no context**: component is props-only for this migration and does not
  need a context migration unless its parent API changes.
- **Mixed**: component uses both runtimes and still needs follow-up cleanup.

Testing expectations:

- Prefer Playwright coverage in `./e2e` for visible migrated behavior.
- Use Vitest/unit tests for pure helper behavior or command/domain behavior.
- Do not mark a legacy consumer as migrated until it has passing targeted
  coverage or is covered by an existing e2e flow listed in this table.

## Current coverage

- `e2e/ng-context-bootstrap.spec.ts`
  - root `NgContext` bootstrap
  - settings dialog smoke behavior while ng is mounted
  - new-session composer/navigation smoke behavior while ng is mounted
- `e2e/ng-component-migration.spec.ts`
  - `AppMacWindowSpacer.svelte`
  - `SessionHeaderDropdown.svelte`
  - `AppHeader.svelte`
  - `AppShell.svelte`
  - `AppSidebar.svelte`
  - `AppKeyboardShortcuts.svelte`
  - `SupportInfoDialog.svelte`
- `ui/src/lib/context/view-commands.vitest.ts`
  - ng view/dialog/navigation/preference commands
  - ng pending-session and sidebar preference commands
  - session/thread view record helpers
- `ui/src/lib/components/test/session-workspace.vitest.ts`
  - `SessionWorkspace.svelte` runtime-backed session ensure/release wiring
- `ui/src/lib/components/test/thread-workspace.vitest.ts`
  - `ThreadWorkspace.svelte` and `ThreadWorkspaceActive.svelte`
    runtime-backed thread ensure/connect/release wiring
- `ui/src/lib/components/test/startup-gate.vitest.ts`
  - `StartupGate.svelte` ng context/runtime startup wiring
- `ui/src/lib/components/test/conversation-credentials-control.vitest.ts`
  - `ConversationCredentialsControl.svelte` ng session credential commands
- `ui/src/lib/context/test/session-credentials.vitest.ts`
  - session credential cache load/save helpers and request filtering

## Root app components

| Component | Status | Targeted coverage | Notes / next action |
| --- | --- | --- | --- |
| `AppHeader.svelte` | Migrated | `e2e/ng-context-bootstrap.spec.ts`, `e2e/ng-component-migration.spec.ts` | Uses `NgContext` environment/preferences/update badge plus ng settings and pending-session commands. |
| `AppKeyboardShortcuts.svelte` | Migrated | `e2e/ng-component-migration.spec.ts`, `ui/src/lib/context/view-commands.vitest.ts` | Uses `NgContext` dialog/navigation/session commands. Recent-thread switcher currently uses ng session cache plus ng recent-thread visible items. |
| `AppMacWindowSpacer.svelte` | Migrated | `e2e/ng-component-migration.spec.ts` | Uses `NgContext` environment. |
| `AppSessionStatus.svelte` | Migrated | `ui/src/lib/context/session-status.vitest.ts` | Uses `NgContext` session record value via `resolveNgSessionDisplayStatus`. |
| `AppShell.svelte` | Migrated | `e2e/ng-context-bootstrap.spec.ts`, `e2e/ng-component-migration.spec.ts` | Uses `NgContext` navigation/selection/startup projections and ng sidebar commands. Child session workspace remains separate legacy target. |
| `AppSidebar.svelte` | Migrated | `e2e/ng-context-bootstrap.spec.ts`, `e2e/ng-component-migration.spec.ts` | Uses `NgContext` session/workspace/thread projections and ng preference/session/workspace commands. Recent-thread list remains empty until ng recent-thread projection parity is added. |
| `AppThreadStatus.svelte` | Migrated | `ui/src/lib/context/session-status.vitest.ts` | Uses `NgContext` session record/thread content via `resolveNgThreadDisplayStatus`. |
| `ConversationComposer.svelte` | Legacy | Existing composer e2e smoke only | Large component; migrate during conversation/thread phase. |
| `ConversationComposerSessionSetupStatus.svelte` | Migrated | `ui/src/lib/components/test/session-status.vitest.ts`, `pnpm typecheck` | Reads session/thread setup status and pending workspace view directly from `NgContext`; no runtime session or thread snapshot dependency. |
| `ConversationCredentialsControl.svelte` | Migrated | `ui/src/lib/components/test/conversation-credentials-control.vitest.ts`, `ui/src/lib/context/test/session-credentials.vitest.ts` | Uses `NgContext` session credential cache/commands and ng credentials dialog command. |
| `ConversationHooksPanel.svelte` | Pure / no context | Parent/component e2e pending | Props-only for current migration. |
| `ConversationPane.svelte` | Legacy | Conversation e2e pending | Migrate during thread/conversation phase. |
| `ConversationPromptHistoryDropdown.svelte` | Pure / no context | Parent/component e2e pending | Props-only for current migration. |
| `ConversationWorkspaceSelector.svelte` | Migrated | `pnpm typecheck` | Uses `NgContext` workspace cache and ng workspace loader; validation remains direct API until a command surface is added. |
| `CredentialsManager.svelte` | Migrated | `ui/src/lib/context/credentials-commands.vitest.ts`, `e2e/ng-component-migration.spec.ts` | Uses `NgContext` credential cache/dialog flow and credential/OAuth commands. |
| `DockPanel.svelte` | Migrated | `pnpm typecheck` | Uses `NgContext` session file/service/diff state and ng file/service commands where available; service output/bind/open still use runtime APIs pending ng command parity. |
| `RecentThreadSwitcherDialog.svelte` | Pure / no context | Shortcut e2e pending | Props-only; parent controller still legacy. |
| `SandboxProvidersManager.svelte` | Migrated | `ui/src/lib/context/sandbox-providers-commands.vitest.ts`, `e2e/ng-component-migration.spec.ts` | Uses `NgContext` sandbox provider cache/commands and credential commands for inline credential creation. |
| `SessionHeaderDropdown.svelte` | Migrated | `e2e/ng-component-migration.spec.ts` | Uses `NgContext` selection/session record and sidebar command. |
| `SessionToolbar.svelte` | Legacy | Session toolbar e2e pending | Migrate after session commands/services/files commands are in ng. |
| `SessionToolbarStack.svelte` | Migrated | `ui/src/lib/context/shell-selectors.vitest.ts` | Uses `NgContext` navigation/selection and `shouldLoadNgSessionToolbar`. Child toolbar remains a separate legacy target. |
| `SessionWorkspace.svelte` | Migrated | `ui/src/lib/components/test/session-workspace.vitest.ts` | Uses runtime-backed session ensure/release APIs directly without legacy context imports. |
| `SettingsDialog.svelte` | Mixed | `e2e/ng-context-bootstrap.spec.ts`, `e2e/ng-component-migration.spec.ts` support-info/credentials/providers flows | Support-info, credentials, and sandbox provider tabs now render ng-backed components; settings/preferences/update shell remains mixed. |
| `StartupGate.svelte` | Migrated | `ui/src/lib/components/test/startup-gate.vitest.ts` | Uses `NgContext` startup task projection and runtime startup/project-event APIs directly. |
| `SupportInfoDialog.svelte` | Migrated | `e2e/ng-component-migration.spec.ts` | Uses `NgContext` support-info data and commands; settings trigger/render condition now uses ng support dialog state. |
| `ThreadWorkspace.svelte` | Migrated | `ui/src/lib/components/test/thread-workspace.vitest.ts` | Uses runtime-backed thread ensure API directly without legacy context imports. |
| `ThreadWorkspaceActive.svelte` | Migrated | `ui/src/lib/components/test/thread-workspace.vitest.ts` | Uses runtime-backed thread connect/release APIs directly without legacy context imports. |

## App parts

| Component | Status | Targeted coverage | Notes / next action |
| --- | --- | --- | --- |
| `parts/AppSidebarDeleteDialog.svelte` | Pure / no context | Sidebar e2e pending | Props-only. |
| `parts/AppSidebarRenameDialog.svelte` | Pure / no context | Sidebar e2e pending | Props-only. |
| `parts/AssistantMessageCopyActions.svelte` | Pure / no context | Conversation e2e pending | Props-only. |
| `parts/BrowserScreenshotPreviewDialog.svelte` | Pure / no context | Conversation e2e pending | Props-only. |
| `parts/ConversationComposerAttachmentButton.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerAttachments.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerHooksControl.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerModelControl.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerProvidersControl.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerQueueControl.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerReasoningControl.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerServiceTierControl.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerSubmitButton.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerTextarea.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationComposerTokenUsage.svelte` | Pure / no context | Conversation e2e pending | Props-only. |
| `parts/ConversationFileMentionDropdown.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationPromptQueuePanel.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationPromptSchedulePicker.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationQueuePanel.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/ConversationSelectionComment.svelte` | Pure / no context | Conversation e2e pending | Props-only. |
| `parts/ConversationSlashCommandDropdown.svelte` | Pure / no context | Composer e2e pending | Props-only. |
| `parts/CredentialEnvVarEditor.svelte` | Pure / no context | Credentials e2e pending | Props-only. |
| `parts/CredentialListItem.svelte` | Pure / no context | Credentials e2e pending | Props-only. |
| `parts/CredentialOAuthScopePicker.svelte` | Pure / no context | Credentials e2e pending | Props-only. |
| `parts/CredentialOAuthWizardDialog.svelte` | Pure / no context | Credentials e2e pending | Props-only. |
| `parts/CredentialTypePicker.svelte` | Pure / no context | Credentials e2e pending | Props-only. |
| `parts/DesktopPanel.svelte` | Pure / no context | Dock e2e pending | Props-only. |
| `parts/DevErrorOverlay.svelte` | Pure / no context | App boot e2e smoke | Props-only. |
| `parts/DiffReviewFileRenderer.svelte` | Pure / no context | Diff e2e pending | Props-only. |
| `parts/DiffReviewPanel.svelte` | Pure / no context | Diff e2e pending | Props-only. |
| `parts/DiffReviewSelectionCommentPopover.svelte` | Pure / no context | Diff e2e pending | Props-only. |
| `parts/DiscobotBrand.svelte` | Pure / no context | App boot e2e smoke | Props-only. |
| `parts/DiscobotLogo.svelte` | Pure / no context | App boot e2e smoke | Props-only. |
| `parts/DiscobotWordmark.svelte` | Pure / no context | App boot e2e smoke | Props-only. |
| `parts/DockWindowChrome.svelte` | Pure / no context | Dock e2e pending | Props-only. |
| `parts/FilesPanel.svelte` | Migrated | `pnpm typecheck` | Props-only file panel with local exported panel-shape types backed by API/ng data. |
| `parts/FilesPanelTabs.svelte` | Pure / no context | Files e2e pending | Props-only. |
| `parts/GlobalFindBar.svelte` | Pure / no context | Shortcut/find e2e pending | Props-only. |
| `parts/HookPreviewDialog.svelte` | Pure / no context | Hooks e2e pending | Props-only. |
| `parts/KeyboardShortcutHelpDialog.svelte` | Pure / no context | Shortcut e2e pending | Props-only. |
| `parts/MessageResponseWithCommand.svelte` | Pure / no context | Conversation e2e pending | Props-only. |
| `parts/ProjectInspectionTerminalDialog.svelte` | Pure / no context | Project settings e2e pending | Props-only. |
| `parts/ProjectSettingsTabContent.svelte` | Pure / no context | Settings e2e pending | Props-only. |
| `parts/ProviderIcon.svelte` | Pure / no context | Credentials/settings e2e pending | Props-only. |
| `parts/RightWindowControls.svelte` | Pure / no context | App boot e2e smoke | Props-only. |
| `parts/SandboxProviderConfigField.svelte` | Pure / no context | Provider settings e2e pending | Props-only. |
| `parts/ServicePanel.svelte` | Pure / no context | Service e2e pending | Props-only. |
| `parts/SessionCommandCredentialsDialog.svelte` | Migrated | Typecheck; parent behavior remains covered by future agent-command credential e2e | Props-only component now imports ng credential dialog types. Parent command/state wiring remains a separate `SessionToolbar` target. |
| `parts/SessionStatus.svelte` | Pure / no context | Status e2e pending | Props-only. |
| `parts/StartupScreen.svelte` | Pure / no context | Startup gate e2e pending | Props-only. |
| `parts/StartupTasksBanner.svelte` | Pure / no context | Startup tasks e2e pending | Props-only. |
| `parts/TerminalPanel.svelte` | Pure / no context | Dock e2e pending | Props-only. |
| `parts/ThreadStateBadge.svelte` | Pure / no context | Thread e2e pending | Props-only. |
| `parts/ThreadWorkspaceHeader.svelte` | Pure / no context | Thread e2e pending | Props-only. |
| `parts/VSCodePanel.svelte` | Pure / no context | Dock e2e pending | Props-only. |

## Working order

1. Small app-shell/status components.
2. Dialog and settings shell components.
3. Sidebar/navigation/session selection components.
4. Startup/session workspace shell components.
5. Session toolbar/dock/files/services components.
6. Thread/conversation/composer components.
7. Credentials/provider/support components.
8. Delete legacy `lib/context` and `lib/store` after all production imports are
   gone and a guard test is added.
