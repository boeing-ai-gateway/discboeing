import { beforeEach, expect, test, vi } from "vitest";

import type { Session, Thread } from "$lib/api-types";
import type { Context } from "$lib/context/context.types";
import { syncRecentThreads } from "$lib/context/domains/recent-threads";
import { createSessionRecord } from "$lib/context/domains/sessions";
import { createThreadRecord } from "$lib/context/domains/threads";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

const RECENT_THREADS_STORAGE_KEY = "recent.threads";

beforeEach(() => {
	const storage = new Map<string, string>();
	vi.stubGlobal("localStorage", {
		getItem: (key: string) => storage.get(key) ?? null,
		removeItem: (key: string) => {
			storage.delete(key);
		},
		setItem: (key: string, value: string) => {
			storage.set(key, value);
		},
	});
});

function createPlainContext(): Context {
	return {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState(),
		commands: undefined as unknown as Context["commands"],
	};
}

function addThread(context: Context, index: number): void {
	const sessionId = `session-${index}`;
	const threadId = `thread-${index}`;
	const sessionRecord = createSessionRecord(sessionId, {
		id: sessionId,
		name: `Session ${index}`,
		createdAt: `2026-06-12T00:0${index}:00.000Z`,
		sandboxStatus: "ready",
	} as Session);

	sessionRecord.threads.byId[threadId] = createThreadRecord(threadId, {
		id: threadId,
		name: `Thread ${index}`,
	} as Thread);
	sessionRecord.threads.allIds = [threadId];
	context.data.sessions.byId[sessionId] = sessionRecord;
	context.data.sessions.allIds.push(sessionId);
}

function selectThread(context: Context, index: number): void {
	context.view.selection.sessionId = `session-${index}`;
	context.view.selection.threadId = `thread-${index}`;
}

test("syncRecentThreads keeps recent threads hidden when the off sentinel is selected", () => {
	const context = createPlainContext();
	context.view.app.preferences.recentThreadsVisibleLimit = 1;

	addThread(context, 1);
	selectThread(context, 1);
	syncRecentThreads(context);

	expect(context.view.app.recentThreads.visibleItems).toEqual([]);
});

test("syncRecentThreads makes four selected recent threads visible when the limit is four", () => {
	const context = createPlainContext();
	context.view.app.preferences.recentThreadsVisibleLimit = 4;

	for (let index = 1; index <= 5; index += 1) {
		addThread(context, index);
		selectThread(context, index);
		syncRecentThreads(context);
	}

	expect(context.view.app.recentThreads.visibleItems).toHaveLength(4);
	expect(
		context.view.app.recentThreads.visibleItems.map(
			(thread) => thread.threadId,
		),
	).toEqual(["thread-5", "thread-4", "thread-3", "thread-2"]);
});

test("syncRecentThreads filters stale stored entries without cached sessions or threads", () => {
	window.localStorage.setItem(
		RECENT_THREADS_STORAGE_KEY,
		JSON.stringify([
			{
				sessionId: "deleted-session",
				threadId: "deleted-thread",
				name: "Deleted Thread",
				lastAccessedAt: "2026-06-12T00:05:00.000Z",
			},
			{
				sessionId: "session-1",
				threadId: "missing-thread",
				name: "Missing Thread",
				lastAccessedAt: "2026-06-12T00:04:00.000Z",
			},
			{
				sessionId: "session-1",
				threadId: "thread-1",
				name: "Thread 1",
				lastAccessedAt: "2026-06-12T00:03:00.000Z",
			},
		]),
	);

	const context = createPlainContext();
	context.view.app.preferences.recentThreadsVisibleLimit = 4;
	addThread(context, 1);

	syncRecentThreads(context);

	expect(
		context.view.app.recentThreads.visibleItems.map(
			(thread) => thread.threadId,
		),
	).toEqual(["thread-1"]);
});
