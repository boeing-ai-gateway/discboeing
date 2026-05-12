import assert from "node:assert/strict";
import test from "node:test";

import { resolveSidebarThreadStatus } from "./thread-status";
import {
	getAvailableSwitcherThreads,
	getThreadSwitcherThreads,
	recentThreadKey,
} from "./thread-switcher";

test("resolveSidebarThreadStatus prefers thread state over ready session", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionStatus: "ready",
			threadState: "interrupted",
		}),
		"needs_attention",
	);
});

test("resolveSidebarThreadStatus prefers active session thread status", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionStatus: "ready",
			sessionActivityStatus: "running",
			threadActivityStatus: "idle",
		}),
		"running",
	);
});

test("resolveSidebarThreadStatus suppresses stale thread activity when session is idle", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionStatus: "ready",
			sessionActivityStatus: "idle",
			threadActivityStatus: "running",
		}),
		"ready",
	);
});

test("resolveSidebarThreadStatus still surfaces thread state when session is idle", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionStatus: "ready",
			sessionActivityStatus: "idle",
			threadState: "interrupted",
		}),
		"needs_attention",
	);
});

test("resolveSidebarThreadStatus falls back to session status", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionStatus: "ready",
		}),
		"ready",
	);
});

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
				threadId: "thread-2",
				name: "Follow-up",
				lastAccessedAt: "2024-01-05T00:00:00.000Z",
			},
			{
				sessionId: "session-2",
				threadId: "session-2",
				name: "Session 2",
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
	assert.equal(allThreads[0]?.name, "Follow-up");
	assert.equal(allThreads[2]?.name, "Session 3");
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
				threadId: "thread-2",
				name: "Follow-up",
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
				threadId: "thread-2",
				name: "Follow-up",
				lastAccessedAt: "2024-01-05T00:00:00.000Z",
			},
			{
				sessionId: "session-2",
				threadId: "session-2",
				name: "Session 2",
				lastAccessedAt: "2024-01-04T00:00:00.000Z",
			},
			{
				sessionId: "session-1",
				threadId: "session-1",
				name: "Session One",
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
