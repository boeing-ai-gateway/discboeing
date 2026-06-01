import type {
	AppContextBootstrap,
	AppPreferences,
} from "$lib/app/app-context.types";
import type { ThemeColorScheme } from "$lib/api-types";
import type { UIStateStore } from "$lib/store/ui-state.store.svelte";
import {
	applyColorScheme,
	applyTheme,
	getAvailableThemes,
	getColorScheme,
	getThemeMode,
	resolveThemeMode,
	type ResolvedTheme,
	type ThemeMode,
} from "$lib/theme";

type CreateAppPreferencesDomainArgs = {
	bootstrap: AppContextBootstrap;
	uiStateStore: UIStateStore;
};

export function createAppPreferencesDomain(
	args: CreateAppPreferencesDomainArgs,
): AppPreferences {
	const { uiStateStore } = args;
	let theme = $state<ThemeMode>("system");
	let resolvedTheme = $state<ResolvedTheme>("dark");
	let colorScheme = $state<ThemeColorScheme>("default");

	const availableThemes = $derived.by(() => getAvailableThemes(resolvedTheme));

	const ensureColorSchemeForMode = () => {
		if (!availableThemes.some((t) => t.id === colorScheme)) {
			colorScheme = "default";
		}
	};

	const syncAppliedColorScheme = () => {
		colorScheme = applyColorScheme(colorScheme);
	};

	const applyThemeState = (mode: ThemeMode) => {
		theme = applyTheme(mode);
		resolvedTheme = resolveThemeMode(theme);
		ensureColorSchemeForMode();
		syncAppliedColorScheme();
	};

	// Theme state still needs its own setup, but persisted UI preferences now
	// read straight from UIStateStore instead of being copied here.
	applyThemeState(getThemeMode());
	colorScheme = getColorScheme();
	ensureColorSchemeForMode();
	syncAppliedColorScheme();

	if (typeof window !== "undefined") {
		const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
		mediaQuery.addEventListener("change", () => {
			if (theme !== "system") return;
			applyThemeState("system");
		});
	}

	return {
		get theme() {
			return theme;
		},
		get resolvedTheme() {
			return resolvedTheme;
		},
		get colorScheme() {
			return colorScheme;
		},
		get availableThemes() {
			return availableThemes;
		},
		get promptHistory() {
			return uiStateStore.promptHistory;
		},
		get pinnedPrompts() {
			return uiStateStore.pinnedPrompts;
		},
		get preferredIde() {
			return uiStateStore.preferredIde;
		},
		ideOptions: args.bootstrap.ideOptions,
		get chatWidthMode() {
			return uiStateStore.chatWidthMode;
		},
		get defaultModel() {
			return uiStateStore.defaultModel;
		},
		get recentThreadsVisibleLimit() {
			return uiStateStore.recentThreadsVisibleLimit;
		},
		get sidebarRecentOpen() {
			return uiStateStore.sidebarRecentOpen;
		},
		get sidebarAllOpen() {
			return uiStateStore.sidebarAllOpen;
		},
		get sidebarAllGroupedByWorkspace() {
			return uiStateStore.sidebarAllGroupedByWorkspace;
		},
		get showRefreshButton() {
			return uiStateStore.showRefreshButton;
		},
		get autoScrollOnStream() {
			return uiStateStore.autoScrollOnStream;
		},
		setTheme: (mode) => applyThemeState(mode),
		setColorScheme: (scheme) => {
			if (!availableThemes.some((t) => t.id === scheme)) return;
			colorScheme = applyColorScheme(scheme);
		},
		toggleTheme: () =>
			applyThemeState(resolvedTheme === "dark" ? "light" : "dark"),
		addPromptToHistory: (prompt) => {
			uiStateStore.addPromptToHistory(prompt);
		},
		removePromptFromHistory: (prompt) => {
			uiStateStore.removePromptFromHistory(prompt);
		},
		pinPrompt: (prompt) => {
			uiStateStore.pinPrompt(prompt);
		},
		unpinPrompt: (prompt) => {
			uiStateStore.unpinPrompt(prompt);
		},
		isPromptPinned: (prompt) => uiStateStore.isPromptPinned(prompt),
		setPreferredIde: (ide) => {
			uiStateStore.setPreferredIde(ide);
		},
		setChatWidthMode: (mode) => {
			uiStateStore.setChatWidthMode(mode);
		},
		setDefaultModel: (modelId) => {
			uiStateStore.setDefaultModel(modelId);
		},
		setRecentThreadsVisibleLimit: (value) => {
			uiStateStore.setRecentThreadsVisibleLimit(value);
		},
		setSidebarRecentOpen: (value) => {
			uiStateStore.setSidebarRecentOpen(value);
		},
		setSidebarAllOpen: (value) => {
			uiStateStore.setSidebarAllOpen(value);
		},
		setSidebarAllGroupedByWorkspace: (value) => {
			uiStateStore.setSidebarAllGroupedByWorkspace(value);
		},
		setShowRefreshButton: (value) => {
			uiStateStore.setShowRefreshButton(value);
		},
		setAutoScrollOnStream: (value) => {
			uiStateStore.setAutoScrollOnStream(value);
		},
	};
}
