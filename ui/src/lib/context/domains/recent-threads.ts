import type { Context } from "$lib/context/context.types";
import { listCachedSessions } from "$lib/context/domains/sessions";
import {
	createRecentThreadsStore,
	getVisibleRecentThreads,
	recordRecentThread,
	replaceRecentThreads,
} from "$lib/context/stores/recent-threads";
import type { SavedRecentThreadEntry } from "$lib/context/stores/recent-threads";

const recentThreadsStateByContext = new WeakMap<
	Context,
	ReturnType<typeof createRecentThreadsStore>
>();

function recentThreadsState(context: Context) {
	let state = recentThreadsStateByContext.get(context);
	if (!state) {
		state = createRecentThreadsStore();
		recentThreadsStateByContext.set(context, state);
	}
	return state;
}

function hasCachedThread(
	context: Context,
	sessionId: string,
	threadId: string,
): boolean {
	const sessionRecord = context.data.sessions.byId[sessionId];
	return Boolean(
		sessionRecord?.value && sessionRecord.threads.byId[threadId]?.value,
	);
}

function cachedRecentThreads(
	context: Context,
	recentThreads: SavedRecentThreadEntry[],
): SavedRecentThreadEntry[] {
	return recentThreads.filter((entry) =>
		hasCachedThread(context, entry.sessionId, entry.threadId),
	);
}

function selectedThreadName(context: Context): string | null {
	const sessionId = context.view.selection.sessionId;
	const threadId = context.view.selection.threadId;
	if (!sessionId || !threadId) {
		return null;
	}

	const sessionRecord = context.data.sessions.byId[sessionId];
	const sessionObj = sessionRecord?.value ?? null;
	if (!sessionObj) {
		return null;
	}

	const threadObj = sessionRecord.threads.byId[threadId]?.value ?? null;
	if (!threadObj) {
		return null;
	}

	return (
		threadObj?.name || sessionObj.displayName || sessionObj.name || "New Thread"
	);
}

export function syncRecentThreads(context: Context): void {
	const state = recentThreadsState(context);
	const sessionId = context.view.selection.sessionId;
	const threadId = context.view.selection.threadId;
	const name = selectedThreadName(context);
	const visibleLimit = context.view.app.preferences.recentThreadsVisibleLimit;

	if (
		sessionId &&
		threadId &&
		name &&
		hasCachedThread(context, sessionId, threadId)
	) {
		recordRecentThread(state, {
			sessionId,
			threadId,
			name,
		});
	}

	const cachedEntries = cachedRecentThreads(context, state.entries);
	if (cachedEntries.length !== state.entries.length) {
		replaceRecentThreads(state, cachedEntries);
	}

	context.view.app.recentThreads.visibleItems = getVisibleRecentThreads({
		recentThreads: cachedEntries,
		sessions: listCachedSessions(context),
		limit: visibleLimit > 1 ? visibleLimit : 0,
	});
}
