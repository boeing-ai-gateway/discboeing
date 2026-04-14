import assert from "node:assert/strict";
import test from "node:test";

import {
	getAvailableSwitcherThreads,
	getThreadSwitcherThreads,
	recentThreadKey,
} from "./thread-switcher";

test("getAvailableSwitcherThreads includes every session and sorts by last access", () => {
	const allThreads = getAvailableSwitcherThreads({
		sessions: [
			{
				id: "session-1",
				name: "Session 1",
				displayName: "Session One",
				createdAt: "2024-01-01T00:00:00.000Z",
				status: "ready",
			},
			{
				id: "session-2",
				name: "Session 2",
				createdAt: "2024-01-02T00:00:00.000Z",
				status: "ready",
			},
			{
				id: "session-3",
				name: "Session 3",
				createdAt: "2024-01-03T00:00:00.000Z",
				status: "ready",
			},
		],
		recentThreads: [
			{
				sessionId: "session-1",
				sessionName: "Session One",
				sessionStatus: "ready",
				threadId: "thread-2",
				threadName: "Follow-up",
				lastMessage: "Ship the fix",
				lastAccessedAt: "2024-01-05T00:00:00.000Z",
			},
			{
				sessionId: "session-2",
				sessionName: "Session 2",
				sessionStatus: "ready",
				threadId: "session-2",
				threadName: "Session 2",
				lastAccessedAt: "2024-01-04T00:00:00.000Z",
			},
		],
	});

	assert.deepEqual(
		allThreads.map((thread) =>
			recentThreadKey(thread.sessionId, thread.threadId),
		),
		["session-1:thread-2", "session-2:session-2", "session-3:session-3"],
	);
	assert.equal(allThreads[0]?.threadName, "Follow-up");
	assert.equal(allThreads[0]?.lastMessage, "Ship the fix");
	assert.equal(allThreads[2]?.threadName, "Session 3");
});

test("getAvailableSwitcherThreads only adds implicit primary entries for sessions missing from recents", () => {
	const allThreads = getAvailableSwitcherThreads({
		sessions: [
			{
				id: "session-1",
				name: "Session 1",
				createdAt: "2024-01-01T00:00:00.000Z",
				status: "ready",
			},
			{
				id: "session-2",
				name: "Session 2",
				createdAt: "2024-01-02T00:00:00.000Z",
				status: "ready",
			},
		],
		recentThreads: [
			{
				sessionId: "session-1",
				sessionName: "Session 1",
				sessionStatus: "ready",
				threadId: "thread-2",
				threadName: "Follow-up",
				lastAccessedAt: "2024-01-03T00:00:00.000Z",
			},
		],
	});

	assert.deepEqual(
		allThreads.map((thread) =>
			recentThreadKey(thread.sessionId, thread.threadId),
		),
		["session-1:thread-2", "session-2:session-2"],
	);
});

test("getThreadSwitcherThreads keeps the current thread at slot zero", () => {
	const switcherThreads = getThreadSwitcherThreads({
		threads: [
			{
				sessionId: "session-1",
				sessionName: "Session One",
				sessionStatus: "ready",
				threadId: "thread-2",
				threadName: "Follow-up",
				lastAccessedAt: "2024-01-05T00:00:00.000Z",
			},
			{
				sessionId: "session-2",
				sessionName: "Session 2",
				sessionStatus: "ready",
				threadId: "session-2",
				threadName: "Session 2",
				lastAccessedAt: "2024-01-04T00:00:00.000Z",
			},
			{
				sessionId: "session-1",
				sessionName: "Session One",
				sessionStatus: "ready",
				threadId: "session-1",
				threadName: "Session One",
				lastAccessedAt: "2024-01-01T00:00:00.000Z",
			},
		],
		selectedThreadKey: "session-1:session-1",
	});

	assert.deepEqual(
		switcherThreads.map((thread) =>
			recentThreadKey(thread.sessionId, thread.threadId),
		),
		["session-1:session-1", "session-1:thread-2", "session-2:session-2"],
	);
});
