import { getCommandContext } from "$lib/context/commands";
import type { SwitcherCommitModifier } from "$lib/app/global-shortcuts";
import type {
	ChatWidthMode,
	SettingsDialogTab,
} from "$lib/app/app-context.types";
import { api } from "$lib/api-client";
import type {
	AgentCommand,
	ChatMessage,
	WorkspaceValidationResult,
} from "$lib/api-types";
import type { ServiceOutputSubscription } from "$lib/thread/chat-stream-manager";
import type {
	SessionContextValue,
	ThreadContextValue,
} from "$lib/session/session-context.types";
import type {
	CredentialValidityPreset,
	CredentialValidityUnit,
} from "$lib/context/context.types";
import type { PreferredIde } from "$lib/app/ide-options";
import type { ThemeColorScheme } from "$lib/api-types";
import type { ThemeMode } from "$lib/theme";
import {
	applyColorScheme,
	applyTheme,
	getAvailableThemes,
	resolveThemeMode,
} from "$lib/theme";
import { UIStateStore } from "$lib/store/ui-state.store.svelte";
import {
	checkForAppUpdate,
	closeAppUpdate,
	downloadAppUpdate,
	installAppUpdate,
	relaunchApp,
	type DesktopDownloadEvent,
} from "$lib/shell";
import {
	connectRuntimeProjectEvents,
	connectRuntimeThread,
	createRuntimeThread,
	acceptRuntimeFileConflict,
	addRuntimeThreadPendingComment,
	bindRuntimeServiceLocalhost,
	cancelRuntimeThread,
	clearRuntimeComposerDraft,
	clearRuntimeThreadNextComposerValues,
	clearRuntimeThreadPendingComments,
	closeRuntimeFile,
	collapseRuntimeFileTree,
	deleteRuntimeQueuedPrompt,
	deleteRuntimeSession,
	deleteRuntimeThread,
	discardRuntimeFileBuffer,
	ensureRuntimeSessionState,
	ensureRuntimeThreadState,
	expandRuntimeFileTree,
	forceSaveRuntimeFile,
	getRuntimeFileEditorModel,
	getRuntimeFileEditorViewState,
	initializeAppRuntime,
	openRuntimeFile,
	openRuntimeService,
	openRuntimeThread,
	refreshCredentialsData,
	refreshRuntimeCommands,
	refreshRuntimeData,
	refreshRuntimeFiles,
	refreshRuntimeHooks,
	refreshRuntimeServices,
	refreshRuntimeThread,
	refreshWorkspacesData,
	releaseRuntimeThreadState,
	releaseRuntimeSessionState,
	renameRuntimeSession,
	renameRuntimeThread,
	renameRuntimeFile,
	removeRuntimeFile,
	rerunRuntimeHook,
	runtime,
	runRuntimeCommand,
	selectRuntimeSession,
	saveRuntimeFile,
	setRuntimeComposerDraft,
	setRuntimeConversationScrollTop,
	setRuntimeFileDiffTarget,
	setRuntimeFileEditorModel,
	setRuntimeFileEditorViewState,
	setRuntimeHookPaused,
	setRuntimeHooksPaused,
	setRuntimeThreadNextModelId,
	setRuntimeThreadNextReasoning,
	setRuntimeThreadNextServiceTier,
	shouldLoadRuntimeSession,
	startRuntimeService,
	startNewRuntimeSession,
	stopRuntimeService,
	stopRuntimeSession,
	submitRuntimeThread,
	syncRuntimeProjections,
	toggleRuntimeFileDirectory,
	toggleRuntimeFilesChangedOnly,
	unbindRuntimeServiceLocalhost,
	updateRuntimeFileBuffer,
	removeRuntimeThreadPendingComment,
	updateRuntimeQueuedPrompt,
} from "$lib/app/app-runtime.svelte";
import {
	DESKTOP_SERVICE_ID,
	VSCODE_SERVICE_ID,
} from "$lib/session/service-ids";

const uiStateStore = new UIStateStore();
let pendingUpdate: {
	updateRid: number;
	bytesRid: number | null;
} | null = null;

export function initializeAppCommands(bootstrap: {
	selectedSessionId?: string;
	selectedThreadId?: string;
}): void {
	initializeAppRuntime(bootstrap);
	const context = getCommandContext();
	context.view.app.preferences.preferredIde = uiStateStore.preferredIde;
	context.view.app.preferences.chatWidthMode = uiStateStore.chatWidthMode;
	context.view.app.preferences.defaultModel = uiStateStore.defaultModel;
	context.view.app.preferences.defaultReasoning = uiStateStore.defaultReasoning;
	context.view.app.preferences.defaultServiceTier =
		uiStateStore.defaultServiceTier;
	context.view.app.preferences.recentThreadsVisibleLimit =
		uiStateStore.recentThreadsVisibleLimit;
	context.view.app.preferences.sidebarRecentOpen =
		uiStateStore.sidebarRecentOpen;
	context.view.app.preferences.sidebarAllOpen = uiStateStore.sidebarAllOpen;
	context.view.app.preferences.sidebarAllGroupedByWorkspace =
		uiStateStore.sidebarAllGroupedByWorkspace;
	context.view.app.preferences.showRefreshButton =
		uiStateStore.showRefreshButton;
	context.view.app.preferences.topBarIconOnly = uiStateStore.topBarIconOnly;
	context.view.app.preferences.autoScrollOnStream =
		uiStateStore.autoScrollOnStream;
	context.view.app.preferences.promptHistory = uiStateStore.promptHistory;
	context.view.app.preferences.pinnedPrompts = uiStateStore.pinnedPrompts;
	context.view.app.preferences.ignoredUpdateVersion =
		uiStateStore.ignoredUpdateVersion;
	context.view.app.preferences.trackPrereleases = uiStateStore.trackPrereleases;
}

export function syncAppNavigationFromBridge(): void {
	syncRuntimeProjections();
}

export function shouldLoadSessionWorkspace(
	sessionId: string,
	options?: { includePending?: boolean },
): boolean {
	return shouldLoadRuntimeSession(sessionId, options);
}

export function shouldLoadSessionToolbar(sessionId: string): boolean {
	return shouldLoadRuntimeSession(sessionId);
}

export function selectSession(sessionId: string): void {
	selectRuntimeSession(sessionId);
	syncAppNavigationFromBridge();
}

export function openThread(sessionId: string, threadId: string): void {
	openRuntimeThread(sessionId, threadId);
	syncAppNavigationFromBridge();
}

export function startNewSession(): void {
	startNewRuntimeSession();
	syncAppNavigationFromBridge();
}

export async function createThread(sessionId: string): Promise<string | null> {
	const threadId = await createRuntimeThread(sessionId);
	syncAppNavigationFromBridge();
	return threadId;
}

export async function renameSession(
	sessionId: string,
	nextName: string,
): Promise<boolean> {
	const renamed = await renameRuntimeSession(sessionId, nextName);
	syncAppNavigationFromBridge();
	return renamed;
}

export async function stopSession(sessionId: string): Promise<boolean> {
	const stopped = await stopRuntimeSession(sessionId);
	syncAppNavigationFromBridge();
	return stopped;
}

export async function deleteSession(sessionId: string): Promise<boolean> {
	const deleted = await deleteRuntimeSession(sessionId);
	syncAppNavigationFromBridge();
	return deleted;
}

export async function renameThread(
	sessionId: string,
	threadId: string,
	nextName: string,
): Promise<boolean> {
	const renamed = await renameRuntimeThread(sessionId, threadId, nextName);
	syncAppNavigationFromBridge();
	return renamed;
}

export async function deleteThread(
	sessionId: string,
	threadId: string,
): Promise<boolean> {
	const deleted = await deleteRuntimeThread(sessionId, threadId);
	syncAppNavigationFromBridge();
	return deleted;
}

export async function submitThread(
	sessionId: string,
	threadId: string,
	payload: {
		parts: ChatMessage["parts"];
		workspaceId?: string;
		providerId?: string;
		workspaceType?: "local" | "git" | null;
		workspacePath?: string | null;
		allowEmptyPendingMessage?: boolean;
		runAfter?: string;
	},
): ReturnType<typeof submitRuntimeThread> {
	return submitRuntimeThread(sessionId, threadId, payload);
}

export async function cancelThread(
	sessionId: string,
	threadId: string,
): Promise<void> {
	await cancelRuntimeThread(sessionId, threadId);
}

export async function refreshThread(
	sessionId: string,
	threadId: string,
): Promise<void> {
	await refreshRuntimeThread(sessionId, threadId);
}

export function setComposerDraft(sessionId: string, value: string): void {
	setRuntimeComposerDraft(sessionId, value);
}

export function clearComposerDraft(
	sessionId: string,
	threadId: string,
	storageKey?: string,
): void {
	clearRuntimeComposerDraft(sessionId, threadId, storageKey);
}

export function setThreadNextModelId(
	sessionId: string,
	threadId: string,
	modelId: string | null | undefined,
): void {
	setRuntimeThreadNextModelId(sessionId, threadId, modelId);
}

export function setThreadNextReasoning(
	sessionId: string,
	threadId: string,
	reasoning: string | undefined,
): void {
	setRuntimeThreadNextReasoning(sessionId, threadId, reasoning);
}

export function setThreadNextServiceTier(
	sessionId: string,
	threadId: string,
	serviceTier: string | null | undefined,
): void {
	setRuntimeThreadNextServiceTier(sessionId, threadId, serviceTier);
}

export function clearThreadNextComposerValues(
	sessionId: string,
	threadId: string,
): void {
	clearRuntimeThreadNextComposerValues(sessionId, threadId);
}

export function addThreadPendingComment(
	sessionId: string,
	threadId: string,
	comment: Parameters<ThreadContextValue["addPendingComment"]>[0],
): void {
	addRuntimeThreadPendingComment(sessionId, threadId, comment);
}

export function removeThreadPendingComment(
	sessionId: string,
	threadId: string,
	commentId: string,
): void {
	removeRuntimeThreadPendingComment(sessionId, threadId, commentId);
}

export function clearThreadPendingComments(
	sessionId: string,
	threadId: string,
): void {
	clearRuntimeThreadPendingComments(sessionId, threadId);
}

export async function deleteQueuedPrompt(
	sessionId: string,
	threadId: string,
	queueId: string,
): Promise<void> {
	await deleteRuntimeQueuedPrompt(sessionId, threadId, queueId);
}

export async function updateQueuedPrompt(
	sessionId: string,
	threadId: string,
	queueId: string,
	payload: Parameters<ThreadContextValue["updateQueuedPrompt"]>[1],
): Promise<void> {
	await updateRuntimeQueuedPrompt(sessionId, threadId, queueId, payload);
}

export function setConversationScrollTop(
	sessionId: string,
	threadId: string,
	scrollTop: number,
): void {
	setRuntimeConversationScrollTop(sessionId, threadId, scrollTop);
}

export async function openFile(
	sessionId: string,
	path?: string,
): Promise<void> {
	await openRuntimeFile(sessionId, path);
}

export async function refreshFiles(sessionId: string): Promise<void> {
	await refreshRuntimeFiles(sessionId);
}

export async function setFileDiffTarget(
	sessionId: string,
	target: string,
): Promise<void> {
	await setRuntimeFileDiffTarget(sessionId, target);
}

export async function saveFile(
	sessionId: string,
	path: string,
): Promise<boolean> {
	return saveRuntimeFile(sessionId, path);
}

export async function renameFile(
	sessionId: string,
	path: string,
	nextName: string,
): Promise<boolean> {
	return renameRuntimeFile(sessionId, path, nextName);
}

export async function removeFile(
	sessionId: string,
	path: string,
): Promise<boolean> {
	return removeRuntimeFile(sessionId, path);
}

export async function refreshHooks(sessionId: string): Promise<void> {
	await refreshRuntimeHooks(sessionId);
}

export function rerunHook(sessionId: string, hookId: string): void {
	rerunRuntimeHook(sessionId, hookId);
}

export async function setHooksPaused(
	sessionId: string,
	paused: boolean,
): Promise<void> {
	await setRuntimeHooksPaused(sessionId, paused);
}

export async function setHookPaused(
	sessionId: string,
	hookId: string,
	paused: boolean,
): Promise<void> {
	await setRuntimeHookPaused(sessionId, hookId, paused);
}

export async function refreshServices(sessionId: string): Promise<void> {
	await refreshRuntimeServices(sessionId);
}

export function openService(sessionId: string, serviceId: string): void {
	openRuntimeService(sessionId, serviceId);
}

export async function startService(
	sessionId: string,
	serviceId: string,
): Promise<void> {
	await startRuntimeService(sessionId, serviceId);
}

export async function stopService(
	sessionId: string,
	serviceId: string,
): Promise<void> {
	await stopRuntimeService(sessionId, serviceId);
}

export async function bindServiceLocalhost(
	sessionId: string,
	serviceId: string,
	port: number,
): Promise<void> {
	await bindRuntimeServiceLocalhost(sessionId, serviceId, port);
}

export async function unbindServiceLocalhost(
	sessionId: string,
	serviceId: string,
): Promise<void> {
	await unbindRuntimeServiceLocalhost(sessionId, serviceId);
}

export async function refreshAgentCommands(sessionId: string): Promise<void> {
	await refreshRuntimeCommands(sessionId);
}

export async function runAgentCommand(
	sessionId: string,
	command: AgentCommand,
): Promise<void> {
	await runRuntimeCommand(sessionId, command);
}

export function closeAgentCommandCredentialDialog(sessionId: string): void {
	ensureRuntimeSessionState(sessionId).commands.credentialDialog.close();
	syncRuntimeProjections();
}

export async function confirmAgentCommandCredentialDialog(
	sessionId: string,
): Promise<void> {
	await ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.confirm();
	syncRuntimeProjections();
}

export function selectAgentCommandCredentialOption(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(sessionId).commands.credentialDialog.selectOption(
		envVar,
		value,
	);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialCreateName(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setCreateCredentialName(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialCreateSecret(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setCreateCredentialSecret(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialValidityPreset(
	sessionId: string,
	envVar: string,
	value: CredentialValidityPreset,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setValidityPreset(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialValidityValue(
	sessionId: string,
	envVar: string,
	value: string,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setValidityValue(envVar, value);
	syncRuntimeProjections();
}

export function setAgentCommandCredentialValidityUnit(
	sessionId: string,
	envVar: string,
	value: CredentialValidityUnit,
): void {
	ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.setValidityUnit(envVar, value);
	syncRuntimeProjections();
}

export async function launchAgentCommandCredentialOAuthWizard(
	sessionId: string,
	envVar: string,
): Promise<void> {
	await ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.launchOAuthWizard(envVar);
	syncRuntimeProjections();
}

export async function refreshAgentCommandCredentialDialogCredentials(
	sessionId: string,
): Promise<void> {
	await ensureRuntimeSessionState(
		sessionId,
	).commands.credentialDialog.refreshAvailableCredentials();
	syncRuntimeProjections();
}

export function closeFile(sessionId: string, path: string): void {
	closeRuntimeFile(sessionId, path);
}

export async function toggleFilesChangedOnly(sessionId: string): Promise<void> {
	await toggleRuntimeFilesChangedOnly(sessionId);
}

export async function toggleFileDirectory(
	sessionId: string,
	path: string,
): Promise<void> {
	await toggleRuntimeFileDirectory(sessionId, path);
}

export async function expandFileTree(sessionId: string): Promise<void> {
	await expandRuntimeFileTree(sessionId);
}

export function collapseFileTree(sessionId: string): void {
	collapseRuntimeFileTree(sessionId);
}

export function updateFileBuffer(
	sessionId: string,
	path: string,
	content: string,
): void {
	updateRuntimeFileBuffer(sessionId, path, content);
}

export function discardFileBuffer(sessionId: string, path: string): void {
	discardRuntimeFileBuffer(sessionId, path);
}

export function acceptFileConflict(sessionId: string, path: string): void {
	acceptRuntimeFileConflict(sessionId, path);
}

export async function forceSaveFile(
	sessionId: string,
	path: string,
): Promise<boolean> {
	return forceSaveRuntimeFile(sessionId, path);
}

export function getFileEditorModel(
	sessionId: string,
	path: string,
): unknown | null {
	return getRuntimeFileEditorModel(sessionId, path);
}

export function setFileEditorModel(
	sessionId: string,
	path: string,
	model: unknown | null,
): void {
	setRuntimeFileEditorModel(sessionId, path, model);
}

export function getFileEditorViewState(
	sessionId: string,
	path: string,
): unknown | null {
	return getRuntimeFileEditorViewState(sessionId, path);
}

export function setFileEditorViewState(
	sessionId: string,
	path: string,
	viewState: unknown | null,
): void {
	setRuntimeFileEditorViewState(sessionId, path, viewState);
}

export async function updateWorkspaceDisplayName(
	workspaceId: string,
	displayName: string,
): Promise<void> {
	const workspace = await api.updateWorkspace(workspaceId, {
		displayName: displayName.trim() || null,
	});
	const context = getCommandContext();
	context.data.workspaces.byId[workspace.id] = workspace;
	context.data.workspaces.items = context.data.workspaces.items.map((item) =>
		item.id === workspace.id ? workspace : item,
	);
}

export async function deleteWorkspace(workspaceId: string): Promise<void> {
	await api.deleteWorkspace(workspaceId);
	const context = getCommandContext();
	context.data.workspaces.items = context.data.workspaces.items.filter(
		(workspace) => workspace.id !== workspaceId,
	);
	delete context.data.workspaces.byId[workspaceId];
}

export async function refreshWorkspaces(): Promise<void> {
	await refreshWorkspacesData();
}

export async function validateWorkspace(
	path: string,
	sourceType: "local" | "git",
): Promise<WorkspaceValidationResult> {
	return api.validateWorkspace({ path, sourceType });
}

export async function refreshCredentials(): Promise<void> {
	await refreshCredentialsData();
}

export function setDesktopSidebarOpen(open: boolean): void {
	getCommandContext().view.app.navigation.desktopSidebarOpen = open;
}

export function setMobileSidebarOpen(open: boolean): void {
	getCommandContext().view.app.navigation.mobileSidebarOpen = open;
}

export function toggleDesktopSidebarOpen(): void {
	const context = getCommandContext();
	context.view.app.navigation.desktopSidebarOpen =
		!context.view.app.navigation.desktopSidebarOpen;
}

export function toggleMobileSidebarOpen(): void {
	const context = getCommandContext();
	context.view.app.navigation.mobileSidebarOpen =
		!context.view.app.navigation.mobileSidebarOpen;
}

export function setSidebarRecentOpen(open: boolean): void {
	const context = getCommandContext();
	context.view.app.preferences.sidebarRecentOpen = open;
	uiStateStore.setSidebarRecentOpen(open);
}

export function setSidebarAllOpen(open: boolean): void {
	const context = getCommandContext();
	context.view.app.preferences.sidebarAllOpen = open;
	uiStateStore.setSidebarAllOpen(open);
}

export function setSidebarAllGroupedByWorkspace(grouped: boolean): void {
	const context = getCommandContext();
	context.view.app.preferences.sidebarAllGroupedByWorkspace = grouped;
	uiStateStore.setSidebarAllGroupedByWorkspace(grouped);
}

export function setPreferredIde(preferredIde: PreferredIde): void {
	const context = getCommandContext();
	context.view.app.preferences.preferredIde = preferredIde;
	uiStateStore.setPreferredIde(preferredIde);
}

export function closeSettingsDialog(): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.open = false;
}

export function setSettingsDialogOpen(open: boolean): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.open = open;
}

export function setSettingsDialogTab(tab: SettingsDialogTab): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.tab = tab;
}

export function setTheme(theme: ThemeMode): void {
	const context = getCommandContext();
	const nextTheme = applyTheme(theme);
	const resolvedTheme = resolveThemeMode(nextTheme);
	context.view.app.preferences.theme = nextTheme;
	context.view.app.preferences.resolvedTheme = resolvedTheme;
	context.view.app.preferences.availableThemes =
		getAvailableThemes(resolvedTheme);
	context.view.app.preferences.colorScheme = applyColorScheme(
		context.view.app.preferences.colorScheme,
	);
}

export function setColorScheme(colorScheme: ThemeColorScheme): void {
	const context = getCommandContext();
	context.view.app.preferences.colorScheme = applyColorScheme(colorScheme);
}

export function setRecentThreadsVisibleLimit(value: number): void {
	getCommandContext().view.app.preferences.recentThreadsVisibleLimit = value;
	uiStateStore.setRecentThreadsVisibleLimit(value);
}

export function setShowRefreshButton(show: boolean): void {
	getCommandContext().view.app.preferences.showRefreshButton = show;
	uiStateStore.setShowRefreshButton(show);
}

export function setTopBarIconOnly(iconOnly: boolean): void {
	getCommandContext().view.app.preferences.topBarIconOnly = iconOnly;
	uiStateStore.setTopBarIconOnly(iconOnly);
}

export function setDefaultModel(modelId: string): void {
	getCommandContext().view.app.preferences.defaultModel = modelId;
	uiStateStore.setDefaultModel(modelId);
}

export function setDefaultReasoning(reasoning: string): void {
	getCommandContext().view.app.preferences.defaultReasoning = reasoning;
	uiStateStore.setDefaultReasoning(reasoning);
}

export function setDefaultServiceTier(serviceTier: string): void {
	getCommandContext().view.app.preferences.defaultServiceTier = serviceTier;
	uiStateStore.setDefaultServiceTier(serviceTier);
}

export function setChatWidthMode(mode: ChatWidthMode): void {
	getCommandContext().view.app.preferences.chatWidthMode = mode;
	uiStateStore.setChatWidthMode(mode);
}

export function setAutoScrollOnStream(enabled: boolean): void {
	getCommandContext().view.app.preferences.autoScrollOnStream = enabled;
	uiStateStore.setAutoScrollOnStream(enabled);
}

export function addPromptToHistory(prompt: string): void {
	uiStateStore.addPromptToHistory(prompt);
	getCommandContext().view.app.preferences.promptHistory =
		uiStateStore.promptHistory;
}

export function removePromptFromHistory(prompt: string): void {
	uiStateStore.removePromptFromHistory(prompt);
	getCommandContext().view.app.preferences.promptHistory =
		uiStateStore.promptHistory;
}

export function pinPrompt(prompt: string): void {
	uiStateStore.pinPrompt(prompt);
	getCommandContext().view.app.preferences.pinnedPrompts =
		uiStateStore.pinnedPrompts;
}

export function unpinPrompt(prompt: string): void {
	uiStateStore.unpinPrompt(prompt);
	getCommandContext().view.app.preferences.pinnedPrompts =
		uiStateStore.pinnedPrompts;
}

export async function checkForUpdates(): Promise<void> {
	const context = getCommandContext();
	context.data.updates.status = "checking";
	context.data.updates.error = null;
	context.data.updates.downloadedBytes = 0;
	context.data.updates.totalBytes = null;
	try {
		if (pendingUpdate) {
			await closeAppUpdate(pendingUpdate.updateRid, pendingUpdate.bytesRid);
			pendingUpdate = null;
		}
		const nextUpdate = await checkForAppUpdate(null);
		if (!nextUpdate) {
			context.data.updates.availableVersion = null;
			context.data.updates.status = "idle";
			context.view.app.updates.showBadge = false;
			return;
		}
		context.data.updates.availableVersion = nextUpdate.version;
		if (uiStateStore.ignoredUpdateVersion === nextUpdate.version) {
			await closeAppUpdate(nextUpdate.rid, null);
			context.data.updates.isIgnored = true;
			context.data.updates.status = "ready";
			context.view.app.updates.showBadge = false;
			return;
		}
		pendingUpdate = { updateRid: nextUpdate.rid, bytesRid: null };
		context.data.updates.status = "downloading";
		const bytesRid = await downloadAppUpdate(
			nextUpdate.rid,
			(event: DesktopDownloadEvent) => {
				if (event.event === "Started") {
					context.data.updates.totalBytes = event.data?.contentLength ?? null;
					context.data.updates.downloadedBytes = 0;
				}
				if (event.event === "Progress") {
					context.data.updates.downloadedBytes += event.data?.chunkLength ?? 0;
				}
				if (
					event.event === "Finished" &&
					context.data.updates.totalBytes !== null
				) {
					context.data.updates.downloadedBytes =
						context.data.updates.totalBytes;
				}
			},
		);
		pendingUpdate.bytesRid = bytesRid;
		context.data.updates.isIgnored = false;
		context.data.updates.status = "ready";
		context.view.app.updates.showBadge = true;
	} catch (error) {
		context.data.updates.status = "error";
		context.data.updates.error =
			error instanceof Error ? error.message : "Failed to check for updates";
		context.view.app.updates.showBadge = false;
	}
}

export async function setTrackPrereleases(track: boolean): Promise<void> {
	getCommandContext().view.app.preferences.trackPrereleases = track;
	uiStateStore.setTrackPrereleases(track);
	await checkForUpdates();
}

export async function installAndRelaunch(): Promise<void> {
	if (!pendingUpdate || pendingUpdate.bytesRid === null) {
		return;
	}
	getCommandContext().data.updates.status = "installing";
	await installAppUpdate(pendingUpdate.updateRid, pendingUpdate.bytesRid);
	await relaunchApp();
}

export function ignoreUpdate(): void {
	const context = getCommandContext();
	uiStateStore.ignoreUpdateVersion(context.data.updates.availableVersion);
	context.data.updates.isIgnored = true;
	context.view.app.updates.showBadge = false;
}

export function openSettingsDialog(tab?: SettingsDialogTab): void {
	const context = getCommandContext();
	context.view.app.dialogs.settings.open = true;
	if (tab) {
		context.view.app.dialogs.settings.tab = tab;
	}
}

export function openGitHubCredentialFlow(): void {
	const context = getCommandContext();
	context.view.app.dialogs.credentials.flowIntent = "github-git";
	context.view.app.dialogs.credentials.open = true;
}

export function clearCredentialFlowIntent(): void {
	getCommandContext().view.app.dialogs.credentials.flowIntent = null;
}

export function openCredentialsDialog(credentialId?: string): void {
	const context = getCommandContext();
	context.view.app.dialogs.credentials.open = true;
	context.view.app.dialogs.credentials.targetId = credentialId ?? null;
}

export function closeCredentialsDialog(): void {
	const context = getCommandContext();
	context.view.app.dialogs.credentials.open = false;
	context.view.app.dialogs.credentials.targetId = null;
}

export function clearCredentialsDialogTarget(): void {
	getCommandContext().view.app.dialogs.credentials.targetId = null;
}

export function closeSupportInfoDialog(): void {
	getCommandContext().view.app.dialogs.supportInfo.open = false;
}

export function openSupportInfoDialog(): void {
	getCommandContext().view.app.dialogs.supportInfo.open = true;
}

export async function fetchSupportInfo(): Promise<void> {
	const context = getCommandContext();
	context.data.supportInfo.status = "loading";
	context.data.supportInfo.error = null;
	try {
		context.data.supportInfo.value = await api.getSupportInfo();
		context.data.supportInfo.status = "ready";
	} catch (error) {
		context.data.supportInfo.error =
			error instanceof Error
				? error.message
				: "Failed to load support information.";
		context.data.supportInfo.status = "error";
	}
}

export function subscribeServiceOutput(args: {
	sessionId: string;
	serviceId: string;
	onOpen?: () => void;
	onError?: (error: unknown) => void;
}): ServiceOutputSubscription {
	return runtime.chatStreams.subscribeServiceOutput(args);
}

export function ensureSessionState(sessionId: string): SessionContextValue {
	return ensureRuntimeSessionState(sessionId);
}

export function releaseSessionState(session: SessionContextValue): void {
	releaseRuntimeSessionState(session);
}

export function ensureThreadState(
	sessionId: string,
	threadId: string,
): ThreadContextValue {
	return ensureRuntimeThreadState(sessionId, threadId);
}

export function connectThread(sessionId: string, threadId: string): void {
	connectRuntimeThread(sessionId, threadId);
}

export function releaseThreadState(
	sessionId: string,
	thread: ThreadContextValue,
): void {
	releaseRuntimeThreadState(sessionId, thread);
}

export function toggleSelectedSessionView(
	viewKind:
		| "terminal"
		| "desktop"
		| "vscode"
		| "file"
		| "diff-review"
		| "services",
): void {
	const context = getCommandContext();
	const sessionId = context.view.app.selection.sessionId;
	if (!sessionId) {
		return;
	}
	const sessionContext = runtime.sessionContexts.get(sessionId);
	if (!sessionContext) {
		return;
	}

	const sessionView = sessionContext.ui;
	if (sessionView.activeView.kind === viewKind) {
		sessionView.openChat();
		return;
	}

	if (viewKind === "terminal") {
		sessionView.openTerminal();
		return;
	}
	if (viewKind === "desktop") {
		sessionView.openDesktop();
		return;
	}
	if (viewKind === "vscode") {
		if (
			sessionContext.services.list.some(
				(service) => service.id === VSCODE_SERVICE_ID,
			)
		) {
			sessionView.openVSCode();
		}
		return;
	}
	if (viewKind === "file") {
		void sessionContext.files.open();
		return;
	}
	if (viewKind === "diff-review") {
		sessionView.openDiffReview();
		return;
	}

	const sessionServices = sessionContext.services.list.filter(
		(service) =>
			service.id !== DESKTOP_SERVICE_ID && service.id !== VSCODE_SERVICE_ID,
	);
	if (sessionServices.length > 0) {
		sessionView.openServices();
	}
}

export async function refreshAppData(): Promise<void> {
	await refreshRuntimeData();
	syncAppNavigationFromBridge();
}

export function connectProjectEvents(): () => void {
	return connectRuntimeProjectEvents();
}

export function setKeyboardShortcutsOpen(open: boolean): void {
	getCommandContext().view.app.dialogs.keyboardShortcuts.open = open;
}

export function toggleKeyboardShortcutsOpen(): void {
	const context = getCommandContext();
	context.view.app.dialogs.keyboardShortcuts.open =
		!context.view.app.dialogs.keyboardShortcuts.open;
}

export function setRecentThreadSwitcherOpen(open: boolean): void {
	getCommandContext().view.app.dialogs.recentThreadSwitcher.open = open;
}

export function setRecentThreadSwitcherSelectedKey(
	selectedKey: string | null,
): void {
	getCommandContext().view.app.dialogs.recentThreadSwitcher.selectedKey =
		selectedKey;
}

export function setRecentThreadSwitcherCommitModifier(
	commitModifier: SwitcherCommitModifier | null,
): void {
	getCommandContext().view.app.dialogs.recentThreadSwitcher.commitModifier =
		commitModifier;
}

export function closeKeyboardShortcutOverlays(): void {
	const context = getCommandContext();
	context.view.app.dialogs.keyboardShortcuts.open = false;
	context.view.app.dialogs.recentThreadSwitcher.open = false;
	context.view.app.dialogs.recentThreadSwitcher.selectedKey = null;
	context.view.app.dialogs.recentThreadSwitcher.commitModifier = null;
}
