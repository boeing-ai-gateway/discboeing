import { compareIsoDatesDesc, getCurrentTimestamp } from "$lib/app/app-helpers";
import { readStorage, writeStorage } from "$lib/local-storage";

const RECENT_THREADS_STORAGE_KEY = "recent.threads";
const ACTIVE_THREAD_SELECTION_STORAGE_KEY = "active.thread";

export type SavedRecentThreadEntry = {
	sessionId: string;
	threadId: string;
	name: string;
	lastAccessedAt: string;
};

function recentThreadKey(sessionId: string, threadId: string): string {
	return `${sessionId}:${threadId}`;
}

function splitRecentThreadKey(key: string): {
	sessionId: string;
	threadId: string;
} | null {
	const separatorIndex = key.indexOf(":");
	if (separatorIndex <= 0 || separatorIndex >= key.length - 1) {
		return null;
	}

	return {
		sessionId: key.slice(0, separatorIndex),
		threadId: key.slice(separatorIndex + 1),
	};
}

type LegacyRecentThreadEntry = {
	sessionId: string;
	threadId: string;
	lastAccessedAt: string;
	name?: string;
	sessionName?: string;
	threadName?: string;
};

function isLegacyRecentThreadEntry(
	value: unknown,
): value is LegacyRecentThreadEntry {
	if (!value || typeof value !== "object") {
		return false;
	}

	const candidate = value as Partial<LegacyRecentThreadEntry>;
	return (
		typeof candidate.sessionId === "string" &&
		candidate.sessionId.length > 0 &&
		typeof candidate.threadId === "string" &&
		candidate.threadId.length > 0 &&
		typeof candidate.lastAccessedAt === "string"
	);
}

function isSavedRecentThreadEntry(
	value: unknown,
): value is SavedRecentThreadEntry {
	if (!value || typeof value !== "object") {
		return false;
	}

	const candidate = value as Partial<SavedRecentThreadEntry>;
	return (
		typeof candidate.sessionId === "string" &&
		candidate.sessionId.length > 0 &&
		typeof candidate.threadId === "string" &&
		candidate.threadId.length > 0 &&
		typeof candidate.name === "string" &&
		candidate.name.length > 0 &&
		typeof candidate.lastAccessedAt === "string"
	);
}

function isSavedThreadSelection(value: unknown): value is {
	sessionId: string;
	threadId: string;
} {
	if (!value || typeof value !== "object") {
		return false;
	}

	const candidate = value as Partial<SavedRecentThreadEntry>;
	return (
		typeof candidate.sessionId === "string" &&
		candidate.sessionId.length > 0 &&
		typeof candidate.threadId === "string" &&
		candidate.threadId.length > 0
	);
}

// Keep only valid entries and sort the newest ones first.
function normalizeEntries(
	entries: SavedRecentThreadEntry[],
): SavedRecentThreadEntry[] {
	const validEntries = entries.filter((entry) =>
		splitRecentThreadKey(recentThreadKey(entry.sessionId, entry.threadId)),
	);

	validEntries.sort((left, right) =>
		compareIsoDatesDesc(left.lastAccessedAt, right.lastAccessedAt),
	);
	return validEntries;
}

// Support both the new entry list and the older storage formats already in use.
function readEntries(): SavedRecentThreadEntry[] {
	const stored = readStorage(RECENT_THREADS_STORAGE_KEY);
	if (!stored) {
		return [];
	}

	try {
		const parsed = JSON.parse(stored) as unknown;
		if (Array.isArray(parsed)) {
			if (parsed.every(isSavedRecentThreadEntry)) {
				return normalizeEntries(parsed);
			}

			return normalizeEntries(
				parsed.filter(isLegacyRecentThreadEntry).map((entry) => ({
					sessionId: entry.sessionId,
					threadId: entry.threadId,
					name:
						entry.name || entry.threadName || entry.sessionName || "New Thread",
					lastAccessedAt: entry.lastAccessedAt,
				})),
			);
		}

		if (!parsed || typeof parsed !== "object") {
			return [];
		}

		return normalizeEntries(
			Object.entries(parsed).flatMap(([key, value]) => {
				const parts = splitRecentThreadKey(key);
				return typeof value === "string" && parts
					? [{ ...parts, name: "New Thread", lastAccessedAt: value }]
					: [];
			}),
		);
	} catch {
		return [];
	}
}

// Returns the most recently accessed session+thread without creating a store.
// Used during app bootstrap to restore the last viewed session/thread on refresh.
export function readInitialThreadSelection(): {
	sessionId: string;
	threadId: string;
} | null {
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

function writeActiveThreadSelection(
	entry: Pick<SavedRecentThreadEntry, "sessionId" | "threadId"> | null,
): void {
	writeStorage(
		ACTIVE_THREAD_SELECTION_STORAGE_KEY,
		entry ? JSON.stringify(entry) : null,
	);
}

export class RecentThreadStore {
	#entries = $state<SavedRecentThreadEntry[]>(readEntries());
	#lastRecordedKey: string | null = null;

	get entries(): SavedRecentThreadEntry[] {
		return this.#entries;
	}

	clearTrackedSelection(): void {
		this.#lastRecordedKey = null;
		writeActiveThreadSelection(null);
	}

	recordSelection(entry: Omit<SavedRecentThreadEntry, "lastAccessedAt">): void {
		const key = recentThreadKey(entry.sessionId, entry.threadId);
		writeActiveThreadSelection(entry);
		const currentEntry = this.#entries.find(
			(item) =>
				item.sessionId === entry.sessionId && item.threadId === entry.threadId,
		);

		if (this.#lastRecordedKey === key && currentEntry) {
			const nextEntry = {
				...currentEntry,
				...entry,
				lastAccessedAt: currentEntry.lastAccessedAt,
			};
			if (JSON.stringify(nextEntry) === JSON.stringify(currentEntry)) {
				return;
			}
			this.#setEntries([
				nextEntry,
				...this.#entries.filter(
					(item) =>
						item.sessionId !== entry.sessionId ||
						item.threadId !== entry.threadId,
				),
			]);
			return;
		}

		this.#lastRecordedKey = key;
		this.#setEntries([
			{
				...currentEntry,
				...entry,
				lastAccessedAt: getCurrentTimestamp(),
			},
			...this.#entries.filter(
				(item) =>
					item.sessionId !== entry.sessionId ||
					item.threadId !== entry.threadId,
			),
		]);
	}

	pruneSession(sessionId: string): void {
		this.#setEntries(
			this.#entries.filter((entry) => entry.sessionId !== sessionId),
		);

		if (this.#lastRecordedKey?.startsWith(`${sessionId}:`)) {
			this.#lastRecordedKey = null;
			writeActiveThreadSelection(null);
		}
	}

	pruneThread(sessionId: string, threadId: string): void {
		this.#setEntries(
			this.#entries.filter(
				(entry) => entry.sessionId !== sessionId || entry.threadId !== threadId,
			),
		);

		if (this.#lastRecordedKey === recentThreadKey(sessionId, threadId)) {
			this.#lastRecordedKey = null;
			writeActiveThreadSelection(null);
		}
	}

	#setEntries(entries: SavedRecentThreadEntry[]): void {
		const nextEntries = normalizeEntries(entries);
		this.#entries = nextEntries;
		writeStorage(
			RECENT_THREADS_STORAGE_KEY,
			nextEntries.length > 0 ? JSON.stringify(nextEntries) : null,
		);
	}
}
