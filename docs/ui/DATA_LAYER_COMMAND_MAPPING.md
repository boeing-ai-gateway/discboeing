# UI Data Layer Command Mapping

This register maps the existing command exports in
`ui/src/lib/context/commands/*.ts` to the next-generation `$lib/context` command
surface.

The purpose is inventory, not final API approval. The new command names may
change while `$lib/context` is still isolated. Each existing command should eventually
land in exactly one category:

- **Existing ng command** — already represented in `NgCommands`.
- **Planned ng command** — should be added to the global command surface.
- **Component-local state** — should move into the owning component instead of
  global commands.
- **Domain internal** — should become implementation detail inside `domains/*`,
  not public component-facing command API.
- **Delete** — old bridge/compatibility behavior that should not exist in the
  final architecture.

## Summary

Current old command exports: 127.

Current exact-name coverage in `NgCommands`: 17.

The current `$lib/context` command surface is therefore not comprehensive. This file
is the working register for closing that gap deliberately.

## Agent commands

Source: `ui/src/lib/context/commands/agent-command.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `refreshAgentCommands` | `refreshAgentCommands(sessionId, options?)` | Planned ng command | Session-scoped global resource. |
| `runAgentCommand` | `runAgentCommand(sessionId, command, options?)` | Planned ng command | Backend mutation command. |
| `closeAgentCommandCredentialDialog` | `closeAgentCommandCredentialDialog(sessionId)` | Planned ng command | Global view state if dialog remains shared. |
| `confirmAgentCommandCredentialDialog` | `confirmAgentCommandCredentialDialog(sessionId, options?)` | Planned ng command | Dialog submit with API side effects. |
| `selectAgentCommandCredentialOption` | `selectAgentCommandCredentialOption(sessionId, envVar, value)` | Planned ng command | Global dialog view state. |
| `setAgentCommandCredentialCreateName` | `setAgentCommandCredentialCreateName(sessionId, envVar, value)` | Planned ng command | Global dialog view state. |
| `setAgentCommandCredentialCreateSecret` | `setAgentCommandCredentialCreateSecret(sessionId, envVar, value)` | Planned ng command | Global dialog view state. |
| `setAgentCommandCredentialValidityPreset` | `setAgentCommandCredentialValidityPreset(sessionId, envVar, value)` | Planned ng command | Global dialog view state. |
| `setAgentCommandCredentialValidityValue` | `setAgentCommandCredentialValidityValue(sessionId, envVar, value)` | Planned ng command | Global dialog view state. |
| `setAgentCommandCredentialValidityUnit` | `setAgentCommandCredentialValidityUnit(sessionId, envVar, value)` | Planned ng command | Global dialog view state. |
| `launchAgentCommandCredentialOAuthWizard` | `launchAgentCommandCredentialOAuthWizard(sessionId, envVar, options?)` | Planned ng command | API/OAuth side effects. |
| `refreshAgentCommandCredentialDialogCredentials` | `refreshAgentCommandCredentialDialogCredentials(sessionId, options?)` | Planned ng command | Could delegate to credential domain loader. |

## Bootstrap

Source: `ui/src/lib/context/commands/bootstrap.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `initializeAppCommands` | `createCommands(context)` / context initialization | Delete | Old bootstrap seam. New context owns command creation directly. |

## Dialog commands

Source: `ui/src/lib/context/commands/dialog.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `closeSettingsDialog` | `closeSettingsDialog()` | Planned ng command | Global app view state. |
| `setSettingsDialogOpen` | `setSettingsDialogOpen(open)` | Planned ng command | Global app view state. |
| `setSettingsDialogTab` | `setSettingsDialogTab(tab)` | Planned ng command | Global app view state. |
| `openSettingsDialog` | `openSettingsDialog(tab?)` | Planned ng command | Global app view state. |
| `openGitHubCredentialFlow` | `openGitHubCredentialFlow()` | Planned ng command | Global credential dialog flow state. |
| `clearCredentialFlowIntent` | `clearCredentialFlowIntent()` | Planned ng command | Global credential dialog flow state. |
| `openCredentialsDialog` | `openCredentialsDialog(credentialId?)` | Planned ng command | Global app view state. |
| `closeCredentialsDialog` | `closeCredentialsDialog()` | Planned ng command | Global app view state. |
| `clearCredentialsDialogTarget` | `clearCredentialsDialogTarget()` | Planned ng command | Global app view state. |
| `closeSupportInfoDialog` | `closeSupportInfoDialog()` | Planned ng command | Global app view state. |
| `openSupportInfoDialog` | `openSupportInfoDialog()` | Planned ng command | Global app view state. |
| `setKeyboardShortcutsOpen` | `setKeyboardShortcutsOpen(open)` | Planned ng command | Global overlay state. |
| `toggleKeyboardShortcutsOpen` | `toggleKeyboardShortcutsOpen()` | Planned ng command | Global overlay state. |
| `setRecentThreadSwitcherOpen` | `setRecentThreadSwitcherOpen(open)` | Planned ng command | Global overlay state. |
| `setRecentThreadSwitcherSelectedKey` | `setRecentThreadSwitcherSelectedKey(key)` | Planned ng command | Global overlay state. |
| `setRecentThreadSwitcherCommitModifier` | `setRecentThreadSwitcherCommitModifier(modifier)` | Planned ng command | Global overlay state. |
| `closeKeyboardShortcutOverlays` | `closeKeyboardShortcutOverlays()` | Planned ng command | Global overlay state. |

## File commands

Source: `ui/src/lib/context/commands/file.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `openFile` | `openFile(sessionId, path, options?)` | Existing ng command | Exact name exists. May need richer editor/buffer behavior. |
| `refreshFiles` | `refreshFileSubtree(sessionId, path, options?)` | Existing ng command | Rename/scope change. Root path refresh covers old full refresh. |
| `setFileDiffTarget` | `setFileDiffTarget(sessionId, path, options?)` | Planned ng command | Global file/diff state if shared. |
| `saveFile` | `saveFile(sessionId, path, content, options?)` | Existing ng command | Exact name exists. |
| `renameFile` | `renameFile(sessionId, from, to, options?)` | Existing ng command | Exact name exists. |
| `deleteFile` | `deleteFile(sessionId, path, options?)` | Existing ng command | Exact name exists. |
| `closeFile` | `closeFile(sessionId, path)` | Planned ng command | Global editor/view state if open files remain shared. |
| `toggleFilesChangedOnly` | `toggleFilesChangedOnly(sessionId)` | Planned ng command | View state. Could be component-local if only one panel owns it. |
| `toggleFileDirectory` | `activateFileSubtree(sessionId, path, options?)` plus view toggle | Existing + planned | Loading maps to existing activation; expanded/collapsed view needs command or local state. |
| `expandFileTree` | `expandFileTree(sessionId, options?)` | Planned ng command | Likely orchestrates activated subtrees. |
| `collapseFileTree` | `collapseFileTree(sessionId)` | Planned ng command | View state. |
| `updateFileBuffer` | `updateFileBuffer(sessionId, path, patch)` | Planned ng command | Shared editor buffer state. |
| `discardFileBuffer` | `discardFileBuffer(sessionId, path)` | Planned ng command | Shared editor buffer state. |
| `acceptFileConflict` | `acceptFileConflict(sessionId, path)` | Planned ng command | Shared editor buffer state. |
| `forceSaveFile` | `forceSaveFile(sessionId, path, options?)` | Planned ng command | File mutation with conflict override. |
| `getFileEditorModel` | none | Component-local state | Monaco model object should likely stay component-local. |
| `setFileEditorModel` | none | Component-local state | Monaco model object should likely stay component-local. |
| `getFileEditorViewState` | maybe `get` from view state directly | Component-local or global view | Prefer component-local unless it must survive panel lifecycle. |
| `setFileEditorViewState` | maybe `setFileEditorViewState(sessionId, path, state)` | Component-local or planned ng command | Only global if we intentionally preserve editor view state. |

## Hook commands

Source: `ui/src/lib/context/commands/hook.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `refreshHooks` | `refreshHooks(sessionId, options?)` | Existing ng command | Exact name exists. |
| `rerunHook` | `rerunHook(sessionId, hookId, options?)` | Existing ng command | Exact name exists. |
| `setHooksPaused` | `pauseHooks(sessionId, paused, options?)` | Planned ng command | Session-wide pause. |
| `setHookPaused` | `pauseHook(sessionId, hookId, paused, options?)` | Existing ng command | Rename to command verb. |

## Navigation commands

Source: `ui/src/lib/context/commands/navigation.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `setDesktopSidebarOpen` | `setDesktopSidebarOpen(open)` | Planned ng command | Global app view state. |
| `setMobileSidebarOpen` | `setMobileSidebarOpen(open)` | Planned ng command | Global app view state. |
| `toggleDesktopSidebarOpen` | `toggleDesktopSidebarOpen()` | Planned ng command | Global app view state. |
| `toggleMobileSidebarOpen` | `toggleMobileSidebarOpen()` | Planned ng command | Global app view state. |
| `toggleSelectedSessionView` | `toggleSelectedSessionView(view)` | Planned ng command | Global session workspace view state. |

## Preference commands

Source: `ui/src/lib/context/commands/preference.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `setSidebarRecentOpen` | `setSidebarRecentOpen(open)` | Planned ng command | Preference/global view state. |
| `setSidebarAllOpen` | `setSidebarAllOpen(open)` | Planned ng command | Preference/global view state. |
| `setSidebarAllGroupedByWorkspace` | `setSidebarAllGroupedByWorkspace(grouped)` | Planned ng command | Preference/global view state. |
| `setPreferredIde` | `setPreferredIde(preferredIde)` | Planned ng command | Preference. |
| `setTheme` | `setTheme(theme)` | Planned ng command | Preference and derived resolved theme handling. |
| `setColorScheme` | `setColorScheme(colorScheme)` | Planned ng command | Preference. |
| `setRecentThreadsVisibleLimit` | `setRecentThreadsVisibleLimit(value)` | Planned ng command | Preference. |
| `setShowRefreshButton` | `setShowRefreshButton(show)` | Planned ng command | Preference. |
| `setTopBarIconOnly` | `setTopBarIconOnly(iconOnly)` | Planned ng command | Preference. |
| `setDefaultModel` | `setDefaultModel(modelId)` | Planned ng command | Preference. |
| `setDefaultReasoning` | `setDefaultReasoning(reasoning)` | Planned ng command | Preference. |
| `setDefaultServiceTier` | `setDefaultServiceTier(serviceTier)` | Planned ng command | Preference. |
| `setChatWidthMode` | `setChatWidthMode(mode)` | Planned ng command | Preference. |
| `setAutoScrollOnStream` | `setAutoScrollOnStream(enabled)` | Planned ng command | Preference. |
| `addPromptToHistory` | `addPromptToHistory(prompt)` | Planned ng command | Preference/local persistence. |
| `removePromptFromHistory` | `removePromptFromHistory(prompt)` | Planned ng command | Preference/local persistence. |
| `pinPrompt` | `pinPrompt(prompt)` | Planned ng command | Preference/local persistence. |
| `unpinPrompt` | `unpinPrompt(prompt)` | Planned ng command | Preference/local persistence. |

## Service commands

Source: `ui/src/lib/context/commands/service.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `refreshServices` | `refreshServices(sessionId, options?)` | Existing ng command | Exact name exists. |
| `openService` | `openService(sessionId, serviceId)` | Planned ng command | Session view state. |
| `startService` | `startService(sessionId, serviceId, options?)` | Existing ng command | Exact name exists. |
| `stopService` | `stopService(sessionId, serviceId, options?)` | Existing ng command | Exact name exists. |
| `bindServiceLocalhost` | `bindServiceLocalhost(sessionId, serviceId, request?, options?)` | Planned ng command | Service mutation. |
| `unbindServiceLocalhost` | `unbindServiceLocalhost(sessionId, serviceId, options?)` | Planned ng command | Service mutation. |
| `subscribeServiceOutput` | `activateServiceOutput(sessionId, serviceId)` | Planned ng command or component-local | If logs are shared/cached, global activation; otherwise component-local stream. |

## Session commands

Source: `ui/src/lib/context/commands/session.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `syncAppNavigationFromBridge` | none | Delete | Bridge compatibility behavior. |
| `shouldLoadSessionWorkspace` | direct `context.data`/status read | Delete | Query/helper, not command. |
| `shouldLoadSessionToolbar` | direct `context.data`/status read | Delete | Query/helper, not command. |
| `selectSession` | `selectSession(sessionId, options?)` / `activateSession` | Planned ng command | Selection plus activation. |
| `openThread` | `openThread(sessionId, threadId, options?)` / `activateThread` | Planned ng command | Selection plus activation. |
| `createSession` | `createSession(input, options?)` | Existing ng command | Exact name exists, but old no-arg create-session view flow may need separate command. |
| `createThread` | `createThread(sessionId, input, options?)` | Existing ng command | Exact name exists with explicit input. |
| `renameSession` | `renameSession(sessionId, name, options?)` | Existing ng command | Exact name exists. |
| `stopSession` | `stopSession(sessionId, options?)` | Existing ng command | Exact name exists. |
| `deleteSession` | `deleteSession(sessionId, options?)` | Existing ng command | Exact name exists. |
| `renameThread` | `renameThread(sessionId, threadId, name, options?)` | Existing ng command | Exact name exists. |
| `deleteThread` | `deleteThread(sessionId, threadId, options?)` | Existing ng command | Exact name exists. |
| `ensureSessionState` | `activateSession(sessionId, options?)` | Domain internal / existing ng command | Old runtime lifecycle should disappear. |
| `releaseSessionState` | maybe `releaseSession(sessionId)` | Planned or delete | Only needed if we keep mounted-session lifecycle. |
| `ensureThreadState` | `activateThread(sessionId, threadId, options?)` | Domain internal / existing ng command | Old runtime lifecycle should disappear. |
| `connectThread` | `activateThread` / project event stream | Delete or planned | Separate chat stream connection should not survive if project stream handles events. |
| `releaseThreadState` | maybe `releaseThread(sessionId, threadId)` | Planned or delete | Only needed if we keep mounted-thread lifecycle. |
| `refreshAppData` | `startup` / `refreshAppData(options?)` | Planned ng command | Should refresh app-global collections explicitly. |
| `connectProjectEvents` | project watch startup | Domain internal | Page load should start project watch; not component-facing command. |

## Support commands

Source: `ui/src/lib/context/commands/support.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `fetchSupportInfo` | `fetchSupportInfo(options?)` | Planned ng command | Shared app data. |

## Thread commands

Source: `ui/src/lib/context/commands/thread.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `submitThread` | `sendMessage(sessionId, threadId, input, options?)` | Existing ng command | Rename. |
| `cancelThread` | `cancelRun(sessionId, threadId, options?)` | Existing ng command | Rename. |
| `refreshThread` | `refreshThread(sessionId, threadId, options?)` | Existing ng command | Exact name exists. |
| `setComposerDraft` | `setComposerDraft(sessionId, value)` | Planned ng command | Global composer state if draft survives component lifecycle. |
| `clearComposerDraft` | `clearComposerDraft(sessionId, threadId?)` | Planned ng command | Global composer state. |
| `setThreadNextModelId` | `setThreadNextModelId(sessionId, threadId, modelId)` | Planned ng command | Composer/thread view state. |
| `setThreadNextReasoning` | `setThreadNextReasoning(sessionId, threadId, reasoning)` | Planned ng command | Composer/thread view state. |
| `setThreadNextServiceTier` | `setThreadNextServiceTier(sessionId, threadId, serviceTier)` | Planned ng command | Composer/thread view state. |
| `clearThreadNextComposerValues` | `clearThreadNextComposerValues(sessionId, threadId)` | Planned ng command | Composer/thread view state. |
| `addThreadPendingComment` | `addThreadPendingComment(sessionId, threadId, comment)` | Planned ng command | Composer/thread view state. |
| `removeThreadPendingComment` | `removeThreadPendingComment(sessionId, threadId, id)` | Planned ng command | Composer/thread view state. |
| `clearThreadPendingComments` | `clearThreadPendingComments(sessionId, threadId)` | Planned ng command | Composer/thread view state. |
| `deleteQueuedPrompt` | `deleteQueuedPrompt(sessionId, threadId, promptId, options?)` | Planned ng command | Backend mutation. |
| `updateQueuedPrompt` | `updateQueuedPrompt(sessionId, threadId, promptId, update, options?)` | Planned ng command | Backend mutation. |
| `setConversationScrollTop` | `setConversationScrollTop(sessionId, threadId, scrollTop)` | Planned ng command or component-local | Global only if scroll position should survive panel lifecycle. |

## Update commands

Source: `ui/src/lib/context/commands/update.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `checkForUpdates` | `checkForUpdates(options?)` | Planned ng command | Desktop app shared state. |
| `setTrackPrereleases` | `setTrackPrereleases(track, options?)` | Planned ng command | Preference/update state. |
| `installUpdateAndRelaunch` | `installUpdateAndRelaunch(options?)` | Planned ng command | Desktop side effect. |
| `ignoreUpdate` | `ignoreUpdate()` | Planned ng command | Update preference/view state. |

## Workspace commands

Source: `ui/src/lib/context/commands/workspace.ts`

| Existing command | Target | Category | Notes |
| --- | --- | --- | --- |
| `renameWorkspace` | `renameWorkspace(workspaceId, name, options?)` | Planned ng command | Backend mutation. |
| `deleteWorkspace` | `deleteWorkspace(workspaceId, options?)` | Planned ng command | Backend mutation. |
| `refreshWorkspaces` | `refreshWorkspaces(options?)` | Planned ng command | Global collection refresh. |
| `validateWorkspace` | `validateWorkspace(input, options?)` | Planned ng command or component-local | Global only if validation result is shared outside the component. |
| `refreshCredentials` | `refreshCredentials(options?)` | Implemented ng command | Loads credential list and types through the credentials domain. |

## Open questions

- Which view-only commands should remain global commands versus component-local
  state?
- Should command names preserve old names for easier migration, or should they be
  renamed to the new activation/refresh terminology?
- Which old runtime lifecycle commands disappear once the single project event
  stream and list-watch activation model are complete?
- Should service output and editor view state be globally cached or owned by the
  panels that render them?
