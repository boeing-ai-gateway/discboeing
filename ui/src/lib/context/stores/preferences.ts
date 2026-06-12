import {
	appendPromptHistoryEntry,
	appendPinnedPrompt,
	removePinnedPrompt,
	removePromptHistoryEntry,
} from "$lib/prompt-history-storage";
import { readStorage, writeStorage } from "$lib/local-storage";
import { isPreferredIde, type PreferredIde } from "$lib/shell/ide-options";

export const PREFERRED_IDE_STORAGE_KEY = "preferred.ide";
export const CHAT_WIDTH_MODE_STORAGE_KEY = "chat.width.mode";
export const DEFAULT_MODEL_STORAGE_KEY = "chat.default.model";
export const DEFAULT_REASONING_STORAGE_KEY = "chat.default.reasoning";
export const DEFAULT_SERVICE_TIER_STORAGE_KEY = "chat.default.serviceTier";
export const IGNORED_UPDATE_VERSION_STORAGE_KEY = "update.ignored.version";
export const TRACK_PRERELEASES_STORAGE_KEY = "update.track.prereleases";
export const SIDEBAR_RECENT_OPEN_STORAGE_KEY = "sidebar.recent.open";
export const SIDEBAR_ALL_OPEN_STORAGE_KEY = "sidebar.all.open";
export const SIDEBAR_ALL_GROUPED_STORAGE_KEY = "sidebar.all.grouped";
export const SHOW_REFRESH_BUTTON_STORAGE_KEY = "header.show.refresh.button";
export const TOP_BAR_ICON_ONLY_STORAGE_KEY = "header.top-bar.icon-only";
export const AUTO_SCROLL_ON_STREAM_STORAGE_KEY = "chat.auto-scroll";
export const RECENT_THREADS_VISIBLE_LIMIT_STORAGE_KEY =
	"recent.threads.visible.limit";
export const PROMPT_HISTORY_STORAGE_KEY = "discobot:composer-history";
export const PINNED_PROMPTS_STORAGE_KEY = "discobot:composer-history:pinned";
export const DEFAULT_PREFERRED_IDE: PreferredIde = "zed";
export const DEFAULT_RECENT_THREADS_VISIBLE_LIMIT = 1;
export const RECENT_THREADS_VISIBLE_LIMIT_PRESETS = [1, 4, 8, 12] as const;

export type ChatWidthModePreference = "full" | "constrained";

function readStringArray(key: string): string[] {
	const stored = readStorage(key);
	if (!stored) {
		return [];
	}

	try {
		const parsed = JSON.parse(stored);
		return Array.isArray(parsed)
			? parsed.filter((item): item is string => typeof item === "string")
			: [];
	} catch {
		return [];
	}
}

function readBoolean(key: string, fallback: boolean): boolean {
	const stored = readStorage(key);
	return stored === null ? fallback : stored === "true";
}

function readPreferredIde(): PreferredIde {
	const stored = readStorage(PREFERRED_IDE_STORAGE_KEY);
	return isPreferredIde(stored) ? stored : DEFAULT_PREFERRED_IDE;
}

function readChatWidthMode(): ChatWidthModePreference {
	return readStorage(CHAT_WIDTH_MODE_STORAGE_KEY) === "full"
		? "full"
		: "constrained";
}

function readRecentThreadsVisibleLimit(): number {
	const value = Number(readStorage(RECENT_THREADS_VISIBLE_LIMIT_STORAGE_KEY));
	return RECENT_THREADS_VISIBLE_LIMIT_PRESETS.includes(
		value as (typeof RECENT_THREADS_VISIBLE_LIMIT_PRESETS)[number],
	)
		? value
		: DEFAULT_RECENT_THREADS_VISIBLE_LIMIT;
}

// Centralize localStorage-backed UI preferences so the rest of the app can
// update context.view while this helper persists the same values.
export class UIStateStore {
	#preferredIde: PreferredIde = readPreferredIde();
	#chatWidthMode: ChatWidthModePreference = readChatWidthMode();
	#defaultModel = readStorage(DEFAULT_MODEL_STORAGE_KEY) ?? "";
	#defaultReasoning = readStorage(DEFAULT_REASONING_STORAGE_KEY) ?? "";
	#defaultServiceTier = readStorage(DEFAULT_SERVICE_TIER_STORAGE_KEY) ?? "";
	#recentThreadsVisibleLimit = readRecentThreadsVisibleLimit();
	#sidebarRecentOpen = readBoolean(SIDEBAR_RECENT_OPEN_STORAGE_KEY, true);
	#sidebarAllOpen = readBoolean(SIDEBAR_ALL_OPEN_STORAGE_KEY, true);
	#sidebarAllGroupedByWorkspace = readBoolean(
		SIDEBAR_ALL_GROUPED_STORAGE_KEY,
		true,
	);
	#showRefreshButton = readBoolean(SHOW_REFRESH_BUTTON_STORAGE_KEY, true);
	#topBarIconOnly = readBoolean(TOP_BAR_ICON_ONLY_STORAGE_KEY, false);
	#autoScrollOnStream = readBoolean(AUTO_SCROLL_ON_STREAM_STORAGE_KEY, true);
	#promptHistory = readStringArray(PROMPT_HISTORY_STORAGE_KEY);
	#pinnedPrompts = readStringArray(PINNED_PROMPTS_STORAGE_KEY);
	#ignoredUpdateVersion = readStorage(IGNORED_UPDATE_VERSION_STORAGE_KEY);
	#trackPrereleases = readBoolean(TRACK_PRERELEASES_STORAGE_KEY, false);

	get preferredIde(): PreferredIde {
		return this.#preferredIde;
	}

	get chatWidthMode(): ChatWidthModePreference {
		return this.#chatWidthMode;
	}

	get defaultModel(): string {
		return this.#defaultModel;
	}

	get defaultReasoning(): string {
		return this.#defaultReasoning;
	}

	get defaultServiceTier(): string {
		return this.#defaultServiceTier;
	}

	get recentThreadsVisibleLimit(): number {
		return this.#recentThreadsVisibleLimit;
	}

	get sidebarRecentOpen(): boolean {
		return this.#sidebarRecentOpen;
	}

	get sidebarAllOpen(): boolean {
		return this.#sidebarAllOpen;
	}

	get sidebarAllGroupedByWorkspace(): boolean {
		return this.#sidebarAllGroupedByWorkspace;
	}

	get showRefreshButton(): boolean {
		return this.#showRefreshButton;
	}

	get topBarIconOnly(): boolean {
		return this.#topBarIconOnly;
	}

	get autoScrollOnStream(): boolean {
		return this.#autoScrollOnStream;
	}

	get promptHistory(): string[] {
		return this.#promptHistory;
	}

	get pinnedPrompts(): string[] {
		return this.#pinnedPrompts;
	}

	get ignoredUpdateVersion(): string | null {
		return this.#ignoredUpdateVersion;
	}

	get trackPrereleases(): boolean {
		return this.#trackPrereleases;
	}

	addPromptToHistory(prompt: string): void {
		this.#promptHistory = appendPromptHistoryEntry(this.#promptHistory, prompt);
		writeStorage(
			PROMPT_HISTORY_STORAGE_KEY,
			JSON.stringify(this.#promptHistory),
		);
	}

	removePromptFromHistory(prompt: string): void {
		this.#promptHistory = removePromptHistoryEntry(this.#promptHistory, prompt);
		writeStorage(
			PROMPT_HISTORY_STORAGE_KEY,
			JSON.stringify(this.#promptHistory),
		);
	}

	pinPrompt(prompt: string): void {
		this.#pinnedPrompts = appendPinnedPrompt(this.#pinnedPrompts, prompt);
		writeStorage(
			PINNED_PROMPTS_STORAGE_KEY,
			JSON.stringify(this.#pinnedPrompts),
		);
	}

	unpinPrompt(prompt: string): void {
		this.#pinnedPrompts = removePinnedPrompt(this.#pinnedPrompts, prompt);
		writeStorage(
			PINNED_PROMPTS_STORAGE_KEY,
			JSON.stringify(this.#pinnedPrompts),
		);
	}

	setPreferredIde(ide: PreferredIde): void {
		this.#preferredIde = ide;
		writeStorage(PREFERRED_IDE_STORAGE_KEY, ide);
	}

	setChatWidthMode(mode: ChatWidthModePreference): void {
		this.#chatWidthMode = mode;
		writeStorage(CHAT_WIDTH_MODE_STORAGE_KEY, mode);
	}

	setDefaultModel(modelId: string): void {
		this.#defaultModel = modelId;
		writeStorage(DEFAULT_MODEL_STORAGE_KEY, modelId || null);
	}

	setDefaultReasoning(reasoning: string): void {
		this.#defaultReasoning = reasoning;
		writeStorage(DEFAULT_REASONING_STORAGE_KEY, reasoning || null);
	}

	setDefaultServiceTier(serviceTier: string): void {
		this.#defaultServiceTier = serviceTier;
		writeStorage(DEFAULT_SERVICE_TIER_STORAGE_KEY, serviceTier || null);
	}

	setRecentThreadsVisibleLimit(value: number): void {
		this.#recentThreadsVisibleLimit = value;
		writeStorage(RECENT_THREADS_VISIBLE_LIMIT_STORAGE_KEY, String(value));
	}

	setSidebarRecentOpen(value: boolean): void {
		this.#sidebarRecentOpen = value;
		writeStorage(SIDEBAR_RECENT_OPEN_STORAGE_KEY, String(value));
	}

	setSidebarAllOpen(value: boolean): void {
		this.#sidebarAllOpen = value;
		writeStorage(SIDEBAR_ALL_OPEN_STORAGE_KEY, String(value));
	}

	setSidebarAllGroupedByWorkspace(value: boolean): void {
		this.#sidebarAllGroupedByWorkspace = value;
		writeStorage(SIDEBAR_ALL_GROUPED_STORAGE_KEY, String(value));
	}

	setShowRefreshButton(value: boolean): void {
		this.#showRefreshButton = value;
		writeStorage(SHOW_REFRESH_BUTTON_STORAGE_KEY, String(value));
	}

	setTopBarIconOnly(value: boolean): void {
		this.#topBarIconOnly = value;
		writeStorage(TOP_BAR_ICON_ONLY_STORAGE_KEY, String(value));
	}

	setAutoScrollOnStream(value: boolean): void {
		this.#autoScrollOnStream = value;
		writeStorage(AUTO_SCROLL_ON_STREAM_STORAGE_KEY, String(value));
	}

	ignoreUpdateVersion(version: string | null): void {
		this.#ignoredUpdateVersion = version;
		writeStorage(IGNORED_UPDATE_VERSION_STORAGE_KEY, version);
	}

	setTrackPrereleases(value: boolean): void {
		this.#trackPrereleases = value;
		writeStorage(TRACK_PRERELEASES_STORAGE_KEY, String(value));
	}

	isPromptPinned(prompt: string): boolean {
		return this.#pinnedPrompts.includes(prompt);
	}
}
