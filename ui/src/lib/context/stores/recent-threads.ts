import { readStorage, writeStorage } from "../../local-storage";
import type { Session } from "../../api-types";
import type { SavedThreadSelection } from "$lib/context/stores/thread-selection";

const RECENT_THREADS_STORAGE_KEY = "recent.threads";

export type SavedRecentThreadEntry = SavedThreadSelection & {
	name: string;
	lastAccessedAt: string;
};

function getCurrentTimestamp(): string {
	return new Date().toISOString();
}

function compareIsoDatesDesc(left: string, right: string): number {
	const leftTime = new Date(left).getTime();
	const rightTime = new Date(right).getTime();
	if (Number.isNaN(leftTime) || Number.isNaN(rightTime)) {
		return 0;
	}
	return rightTime - leftTime;
}

function compareVisibleRecentThreadOrder(args: {
	left: SavedRecentThreadEntry;
	right: SavedRecentThreadEntry;
	sessionsById: Record<string, Session>;
}): number {
	const { left, right, sessionsById } = args;
	const sessionCreatedAtCompare = compareIsoDatesDesc(
		sessionsById[left.sessionId]?.createdAt ?? "",
		sessionsById[right.sessionId]?.createdAt ?? "",
	);
	if (sessionCreatedAtCompare !== 0) {
		return sessionCreatedAtCompare;
	}

	if (left.sessionId !== right.sessionId) {
		return left.sessionId.localeCompare(right.sessionId);
	}

	const leftIsPrimaryThread = left.threadId === left.sessionId;
	const rightIsPrimaryThread = right.threadId === right.sessionId;
	if (leftIsPrimaryThread !== rightIsPrimaryThread) {
		return leftIsPrimaryThread ? -1 : 1;
	}

	return left.threadId.localeCompare(right.threadId);
}

export function getVisibleRecentThreads(args: {
	recentThreads: SavedRecentThreadEntry[];
	sessions: Session[];
	limit: number;
}): SavedRecentThreadEntry[] {
	const { recentThreads, sessions, limit } = args;
	if (limit <= 0 || recentThreads.length === 0) {
		return [];
	}

	const sessionsById = Object.fromEntries(
		sessions.map((session) => [session.id, session] as const),
	);

	// First pick the most recently visited threads, then keep the sidebar grouped
	// by newer sessions so the list feels stable next to the full session list.
	// Use a deterministic tie-breaker within each session group so touching a
	// thread does not reshuffle the visible rows every time.
	return [...recentThreads]
		.sort((left, right) =>
			compareIsoDatesDesc(left.lastAccessedAt, right.lastAccessedAt),
		)
		.slice(0, limit)
		.sort((left, right) =>
			compareVisibleRecentThreadOrder({ left, right, sessionsById }),
		);
}

function recentThreadKey(sessionId: string, threadId: string): string {
	return `${sessionId}:${threadId}`;
}

function splitRecentThreadKey(key: string): SavedThreadSelection | null {
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

export function readRecentThreads(): SavedRecentThreadEntry[] {
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

export type RecentThreadsState = {
	entries: SavedRecentThreadEntry[];
	lastRecordedKey: string | null;
};

export function createRecentThreadsState(): RecentThreadsState {
	return {
		entries: readRecentThreads(),
		lastRecordedKey: null,
	};
}

function writeRecentThreads(entries: SavedRecentThreadEntry[]): void {
	writeStorage(
		RECENT_THREADS_STORAGE_KEY,
		entries.length > 0 ? JSON.stringify(entries) : null,
	);
}

function setRecentThreadsEntries(
	state: RecentThreadsState,
	entries: SavedRecentThreadEntry[],
): void {
	const nextEntries = normalizeEntries(entries);
	state.entries = nextEntries;
	writeRecentThreads(nextEntries);
}

export function clearRecentThreadRecording(state: RecentThreadsState): void {
	state.lastRecordedKey = null;
}

export function recordRecentThread(
	state: RecentThreadsState,
	entry: Omit<SavedRecentThreadEntry, "lastAccessedAt">,
): void {
	const key = recentThreadKey(entry.sessionId, entry.threadId);
	const currentEntry = state.entries.find(
		(item) =>
			item.sessionId === entry.sessionId && item.threadId === entry.threadId,
	);

	if (state.lastRecordedKey === key && currentEntry) {
		const nextEntry = {
			...currentEntry,
			...entry,
			lastAccessedAt: currentEntry.lastAccessedAt,
		};
		if (JSON.stringify(nextEntry) === JSON.stringify(currentEntry)) {
			return;
		}
		setRecentThreadsEntries(state, [
			nextEntry,
			...state.entries.filter(
				(item) =>
					item.sessionId !== entry.sessionId ||
					item.threadId !== entry.threadId,
			),
		]);
		return;
	}

	state.lastRecordedKey = key;
	setRecentThreadsEntries(state, [
		{
			...currentEntry,
			...entry,
			lastAccessedAt: getCurrentTimestamp(),
		},
		...state.entries.filter(
			(item) =>
				item.sessionId !== entry.sessionId || item.threadId !== entry.threadId,
		),
	]);
}

export function pruneRecentSession(
	state: RecentThreadsState,
	sessionId: string,
): void {
	setRecentThreadsEntries(
		state,
		state.entries.filter((entry) => entry.sessionId !== sessionId),
	);

	if (state.lastRecordedKey?.startsWith(`${sessionId}:`)) {
		clearRecentThreadRecording(state);
	}
}

export function pruneRecentThread(
	state: RecentThreadsState,
	sessionId: string,
	threadId: string,
): void {
	setRecentThreadsEntries(
		state,
		state.entries.filter(
			(entry) => entry.sessionId !== sessionId || entry.threadId !== threadId,
		),
	);

	if (state.lastRecordedKey === recentThreadKey(sessionId, threadId)) {
		clearRecentThreadRecording(state);
	}
}

export function createRecentThreadsStore(): RecentThreadsState {
	return createRecentThreadsState();
}
