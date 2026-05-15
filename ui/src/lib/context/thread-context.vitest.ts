import { expect, test } from "vitest";

import {
	applyStreamedThreadUpdate,
	getThreadIsStreaming,
} from "./thread-context.svelte";
import { isThreadSnapshotRunning } from "$lib/app/thread-status";

test("applyStreamedThreadUpdate syncs the primary session title and reloads it", async () => {
	const upserted: string[] = [];
	const synced: string[] = [];
	let reloaded = false;

	applyStreamedThreadUpdate({
		sessionId: "session-1",
		sessionDisplayName: null,
		previousThreadName: "New Thread",
		thread: {
			id: "session-1",
			name: "Fix flaky sidebar refresh",
			lastMessage: "Fix the delayed sidebar titles",
		},
		upsertThread: (thread) => {
			upserted.push(thread.name);
		},
		syncSessionName: (name) => {
			synced.push(name);
		},
		reloadSession: async () => {
			reloaded = true;
		},
	});

	expect(upserted).toEqual(["Fix flaky sidebar refresh"]);
	expect(synced).toEqual(["Fix flaky sidebar refresh"]);
	expect(reloaded).toBe(true);
});

test("applyStreamedThreadUpdate avoids reloading renamed or secondary sessions", () => {
	let reloadCount = 0;

	applyStreamedThreadUpdate({
		sessionId: "session-1",
		sessionDisplayName: "Pinned session",
		previousThreadName: "Old title",
		thread: {
			id: "session-1",
			name: "New streamed title",
			lastMessage: "latest prompt",
		},
		upsertThread: () => {},
		syncSessionName: () => {},
		reloadSession: () => {
			reloadCount += 1;
		},
	});

	applyStreamedThreadUpdate({
		sessionId: "session-1",
		sessionDisplayName: null,
		previousThreadName: "Secondary thread",
		thread: {
			id: "thread-2",
			name: "Secondary thread",
			lastMessage: "follow-up prompt",
		},
		upsertThread: () => {},
		syncSessionName: () => {},
		reloadSession: () => {
			reloadCount += 1;
		},
	});

	expect(reloadCount).toBe(0);
});

test("isThreadSnapshotRunning detects server-side activity", () => {
	expect(
		isThreadSnapshotRunning({
			id: "thread-1",
			name: "Main",
			activityStatus: { status: "running" },
		}),
	).toBe(true);
	expect(
		isThreadSnapshotRunning({
			id: "thread-1",
			name: "Main",
			activeCommand: "pnpm test",
		}),
	).toBe(true);
	expect(
		isThreadSnapshotRunning({
			id: "thread-1",
			name: "Main",
			activityStatus: { status: "idle" },
		}),
	).toBe(false);
});

test("getThreadIsStreaming follows local stream and thread snapshot state", () => {
	expect(
		getThreadIsStreaming(
			{
				id: "thread-1",
				name: "Main",
				activityStatus: { status: "running" },
			},
			false,
		),
	).toBe(true);
	expect(
		getThreadIsStreaming(
			{
				id: "thread-1",
				name: "Main",
				activityStatus: { status: "idle" },
			},
			true,
		),
	).toBe(true);
	expect(
		getThreadIsStreaming(
			{
				id: "thread-1",
				name: "Main",
				activeCommand: "pnpm test",
			},
			false,
		),
	).toBe(true);
	expect(
		getThreadIsStreaming(
			{
				id: "thread-1",
				name: "Main",
			},
			true,
		),
	).toBe(true);
	expect(
		getThreadIsStreaming(
			{
				id: "thread-2",
				name: "Other",
			},
			false,
		),
	).toBe(false);
});
