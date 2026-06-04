import type { ChatWidthMode } from "$lib/app/app-context.types";
import type { PreferredIde } from "$lib/app/ide-options";
import type { ThemeColorScheme } from "$lib/api-types";
import type { ThemeMode } from "$lib/theme";
import { getCommandContext } from "$lib/context/commands";
import {
	applyColorScheme,
	applyTheme,
	getAvailableThemes,
	resolveThemeMode,
} from "$lib/theme";

import { uiStateStore } from "./shared";

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
