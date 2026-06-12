import { readStorage, writeStorage } from "$lib/local-storage";

const ACTIVE_THREAD_SELECTION_STORAGE_KEY = "active.thread";

export type SavedThreadSelection = {
	sessionId: string;
	threadId: string;
};

function isSavedThreadSelection(value: unknown): value is SavedThreadSelection {
	if (!value || typeof value !== "object") {
		return false;
	}

	const candidate = value as Partial<SavedThreadSelection>;
	return (
		typeof candidate.sessionId === "string" &&
		candidate.sessionId.length > 0 &&
		typeof candidate.threadId === "string" &&
		candidate.threadId.length > 0
	);
}

function readThreadSelection(): SavedThreadSelection | null {
	const stored = readStorage(ACTIVE_THREAD_SELECTION_STORAGE_KEY);
	if (!stored) {
		return null;
	}

	try {
		const parsed = JSON.parse(stored) as unknown;
		if (!isSavedThreadSelection(parsed)) {
			return null;
		}
		return { sessionId: parsed.sessionId, threadId: parsed.threadId };
	} catch {
		return null;
	}
}

function writeThreadSelection(
	entry: SavedThreadSelection | null,
): SavedThreadSelection | null {
	writeStorage(
		ACTIVE_THREAD_SELECTION_STORAGE_KEY,
		entry ? JSON.stringify(entry) : null,
	);
	return entry;
}

function shouldRestoreThreadSelection(): boolean {
	if (typeof performance === "undefined") {
		return false;
	}

	const navEntry = performance.getEntriesByType("navigation")[0] as
		| PerformanceNavigationTiming
		| undefined;
	return navEntry?.type === "reload";
}

export const threadSelectionStore = {
	readInitial(): SavedThreadSelection | null {
		return shouldRestoreThreadSelection() ? readThreadSelection() : null;
	},
	set(entry: SavedThreadSelection): SavedThreadSelection {
		return writeThreadSelection(entry) ?? entry;
	},
	clear(): null {
		writeThreadSelection(null);
		return null;
	},
};
