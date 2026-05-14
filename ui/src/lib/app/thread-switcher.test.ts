import assert from "node:assert/strict";
import test from "node:test";

import {
	resolveSessionDisplayStatus,
	resolveSidebarThreadStatus,
	resolveThreadDisplayStatus,
} from "./thread-status";
import {
	getAvailableSwitcherThreads,
	getThreadSwitcherThreads,
	recentThreadKey,
} from "./thread-switcher";

test("resolveSidebarThreadStatus prefers thread state", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			thread: { state: "interrupted" },
		}),
		"needs_attention",
	);
});

test("resolveSidebarThreadStatus prefers active session thread status", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionThreadStatus: { status: "running" },
			thread: { activityStatus: { status: "idle" } },
		}),
		"running",
	);
});

test("resolveSidebarThreadStatus suppresses stale thread activity when session thread is idle", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionThreadStatus: { status: "idle" },
			thread: { activityStatus: { status: "running" } },
		}),
		null,
	);
});

test("resolveSidebarThreadStatus still surfaces thread state when session thread is idle", () => {
	assert.equal(
		resolveSidebarThreadStatus({
			sessionThreadStatus: { status: "idle" },
			thread: { state: "interrupted" },
		}),
		"needs_attention",
	);
});

test("resolveSidebarThreadStatus does not fall back to session status", () => {
	assert.equal(resolveSidebarThreadStatus({}), null);
});

test("resolveSessionDisplayStatus normalizes resting ready sessions", () => {
	assert.equal(
		resolveSessionDisplayStatus({
			sandboxStatus: "ready",
			threadStatus: { status: "idle" },
		}),
		"idle",
	);
});

test("resolveThreadDisplayStatus inherits committed session display", () => {
	assert.equal(
		resolveThreadDisplayStatus({
			session: {
				sandboxStatus: "ready",
				commitStatus: "completed",
				commitOperation: "commit",
			},
			thread: {
				activityStatus: { status: "running" },
				pendingQuestion: true,
			},
		}),
		"committed",
	);
});

test("resolveThreadDisplayStatus only applies matching session thread status", () => {
	assert.equal(
		resolveThreadDisplayStatus({
			session: {
				sandboxStatus: "ready",
				threadStatus: { status: "running", threadId: "thread-active" },
			},
			thread: { activityStatus: { status: "idle" } },
		}),
		"idle",
	);
	assert.equal(
		resolveThreadDisplayStatus({
			session: {
				sandboxStatus: "ready",
				threadStatus: { status: "running", threadId: "thread-active" },
			},
			sessionThreadStatus: { status: "running", threadId: "thread-active" },
			thread: { activityStatus: { status: "idle" } },
		}),
		"running",
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
				sandboxStatus: "ready",
			},
			{
				id: "session-2",
				name: "Session 2",
				createdAt: "2024-01-02T00:00:00.000Z",
				sandboxStatus: "ready",
			},
			{
				id: "session-3",
				name: "Session 3",
				createdAt: "2024-01-03T00:00:00.000Z",
				sandboxStatus: "ready",
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
				sandboxStatus: "ready",
			},
			{
				id: "session-2",
				name: "Session 2",
				createdAt: "2024-01-02T00:00:00.000Z",
				sandboxStatus: "ready",
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
