import { getCommandContext } from "$lib/context/commands";
import type { SwitcherCommitModifier } from "$lib/app/global-shortcuts";
import type {
	ChatWidthMode,
	SettingsDialogTab,
} from "$lib/app/app-context.types";
import { api } from "$lib/api-client";
import type { WorkspaceValidationResult } from "$lib/api-types";
import type { ServiceOutputSubscription } from "$lib/thread/chat-stream-manager";
import type { SessionContextValue } from "$lib/session/session-context.types";
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
	createRuntimeThread,
	deleteRuntimeSession,
	deleteRuntimeThread,
	ensureRuntimeSessionState,
	initializeAppRuntime,
	openRuntimeThread,
	refreshCredentialsData,
	refreshRuntimeData,
	refreshWorkspacesData,
	releaseRuntimeSessionState,
	renameRuntimeSession,
	renameRuntimeThread,
	runtime,
	selectRuntimeSession,
	shouldLoadRuntimeSession,
	startNewRuntimeSession,
	stopRuntimeSession,
	syncRuntimeProjections,
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
