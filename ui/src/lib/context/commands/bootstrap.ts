import { getCommandContext } from "$lib/context/commands";
import { initializeAppRuntime } from "$lib/app/app-runtime.svelte";

import { uiStateStore } from "./shared";

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
