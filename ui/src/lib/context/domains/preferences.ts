import type { PreferredIde } from "$lib/shell/ide-options";
import type { ThemeColorScheme } from "$lib/api-types";
import type { ChatWidthMode, Context } from "$lib/context/context.types";
import { UIStateStore } from "$lib/context/stores/preferences";
import type { ThemeMode } from "$lib/theme";
import {
	applyColorScheme,
	applyTheme,
	getAvailableThemes,
	resolveThemeMode,
} from "$lib/theme";
import {
	diffReviewPreferencesStore,
	type DiffReviewApprovals,
} from "$lib/context/stores/diff-review-preferences";
import type { DiffStyle } from "$lib/pierre-diff";

export const uiStateStore = new UIStateStore();

export async function setPreferredIde(
	context: Context,
	preferredIde: PreferredIde,
): Promise<void> {
	context.view.app.preferences.preferredIde = preferredIde;
	uiStateStore.setPreferredIde(preferredIde);
}

export async function setTheme(
	context: Context,
	theme: ThemeMode,
): Promise<void> {
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

export async function setColorScheme(
	context: Context,
	colorScheme: ThemeColorScheme,
): Promise<void> {
	context.view.app.preferences.colorScheme = applyColorScheme(colorScheme);
}

export async function setRecentThreadsVisibleLimit(
	context: Context,
	value: number,
): Promise<void> {
	context.view.app.preferences.recentThreadsVisibleLimit = value;
	uiStateStore.setRecentThreadsVisibleLimit(value);
}

export async function setShowRefreshButton(
	context: Context,
	show: boolean,
): Promise<void> {
	context.view.app.preferences.showRefreshButton = show;
	uiStateStore.setShowRefreshButton(show);
}

export async function setShowDebugOverlay(
	context: Context,
	show: boolean,
): Promise<void> {
	context.view.app.preferences.showDebugOverlay = show;
	uiStateStore.setShowDebugOverlay(show);
}

export async function setTopBarIconOnly(
	context: Context,
	iconOnly: boolean,
): Promise<void> {
	context.view.app.preferences.topBarIconOnly = iconOnly;
	uiStateStore.setTopBarIconOnly(iconOnly);
}

export async function setDefaultModel(
	context: Context,
	modelId: string,
): Promise<void> {
	context.view.app.preferences.defaultModel = modelId;
	uiStateStore.setDefaultModel(modelId);
}

export async function setDefaultReasoning(
	context: Context,
	reasoning: string,
): Promise<void> {
	context.view.app.preferences.defaultReasoning = reasoning;
	uiStateStore.setDefaultReasoning(reasoning);
}

export async function setDefaultServiceTier(
	context: Context,
	serviceTier: string,
): Promise<void> {
	context.view.app.preferences.defaultServiceTier = serviceTier;
	uiStateStore.setDefaultServiceTier(serviceTier);
}

export async function setChatWidthMode(
	context: Context,
	mode: ChatWidthMode,
): Promise<void> {
	context.view.app.preferences.chatWidthMode = mode;
	uiStateStore.setChatWidthMode(mode);
}

export async function setAutoScrollOnStream(
	context: Context,
	enabled: boolean,
): Promise<void> {
	context.view.app.preferences.autoScrollOnStream = enabled;
	uiStateStore.setAutoScrollOnStream(enabled);
}

export async function setSidebarRecentOpen(
	context: Context,
	open: boolean,
): Promise<void> {
	context.view.app.preferences.sidebarRecentOpen = open;
	uiStateStore.setSidebarRecentOpen(open);
}

export async function setSidebarAllOpen(
	context: Context,
	open: boolean,
): Promise<void> {
	context.view.app.preferences.sidebarAllOpen = open;
	uiStateStore.setSidebarAllOpen(open);
}

export async function setSidebarAllGroupedByWorkspace(
	context: Context,
	grouped: boolean,
): Promise<void> {
	context.view.app.preferences.sidebarAllGroupedByWorkspace = grouped;
	uiStateStore.setSidebarAllGroupedByWorkspace(grouped);
}

export async function setDiffReviewApprovals(
	context: Context,
	approvals: DiffReviewApprovals,
): Promise<void> {
	context.view.app.diffReview.approvals =
		diffReviewPreferencesStore.setApprovals(approvals);
}

export async function setDiffReviewStyle(
	context: Context,
	style: DiffStyle,
): Promise<void> {
	context.view.app.diffReview.style =
		diffReviewPreferencesStore.setStyle(style);
}

export async function addPromptToHistory(
	context: Context,
	prompt: string,
): Promise<void> {
	uiStateStore.addPromptToHistory(prompt);
	context.view.app.preferences.promptHistory = uiStateStore.promptHistory;
}

export async function removePromptFromHistory(
	context: Context,
	prompt: string,
): Promise<void> {
	uiStateStore.removePromptFromHistory(prompt);
	context.view.app.preferences.promptHistory = uiStateStore.promptHistory;
}

export async function pinPrompt(
	context: Context,
	prompt: string,
): Promise<void> {
	uiStateStore.pinPrompt(prompt);
	context.view.app.preferences.pinnedPrompts = uiStateStore.pinnedPrompts;
}

export async function unpinPrompt(
	context: Context,
	prompt: string,
): Promise<void> {
	uiStateStore.unpinPrompt(prompt);
	context.view.app.preferences.pinnedPrompts = uiStateStore.pinnedPrompts;
}
