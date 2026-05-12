import assert from "node:assert/strict";
import test from "node:test";

import { getReconciledSelectedSessionId } from "../domains/app-sessions.helpers";
import { getVisibleRecentThreads } from "../view/create-app-view-state.svelte";
import type { Session } from "$lib/api-types";
import type { RecentThreadSummary, SessionSummary } from "$lib/shell-types";

const sessions: SessionSummary[] = [
	{ id: "session-1", name: "One", isRecent: true, status: "ready" },
	{ id: "session-2", name: "Two", isRecent: false, status: "ready" },
];

test("getReconciledSelectedSessionId prefers an explicit valid session", () => {
	assert.equal(
		getReconciledSelectedSessionId(sessions, "session-1", "session-2"),
		"session-2",
	);
});

test("getReconciledSelectedSessionId keeps the current valid selection", () => {
	assert.equal(
		getReconciledSelectedSessionId(sessions, "session-1"),
		"session-1",
	);
});

test("getReconciledSelectedSessionId clears invalid selections", () => {
	assert.equal(
		getReconciledSelectedSessionId(sessions, "missing-session"),
		null,
	);
});

const appSessions: Session[] = [
	{
		id: "session-1",
		name: "One",
		description: "",
		createdAt: "2024-01-01T00:00:00.000Z",
		timestamp: "2024-01-01T00:00:00.000Z",
		status: "ready",
		files: [],
	},
	{
		id: "session-2",
		name: "Two",
		description: "",
		createdAt: "2024-01-02T00:00:00.000Z",
		timestamp: "2024-01-02T00:00:00.000Z",
		status: "ready",
		files: [],
	},
];

test("getVisibleRecentThreads picks the most recently accessed threads before stabilizing order", () => {
	const visibleRecentThreads = getVisibleRecentThreads({
		recentThreads: [
			{
				sessionId: "session-1",
				threadId: "thread-b",
				name: "B",
				lastAccessedAt: "2024-01-05T00:00:00.000Z",
			},
			{
				sessionId: "session-2",
				threadId: "session-2",
				name: "Two",
				lastAccessedAt: "2024-01-04T00:00:00.000Z",
			},
			{
				sessionId: "session-1",
				threadId: "thread-a",
				name: "A",
				lastAccessedAt: "2024-01-03T00:00:00.000Z",
			},
		],
		sessions: appSessions,
		limit: 2,
	});

	assert.deepEqual(
		visibleRecentThreads.map(
			(thread) => `${thread.sessionId}:${thread.threadId}`,
		),
		["session-2:session-2", "session-1:thread-b"],
	);
});

test("getVisibleRecentThreads keeps same-session rows stable when access times change", () => {
	const recentThreadsA: RecentThreadSummary[] = [
		{
			sessionId: "session-1",
			threadId: "thread-b",
			name: "B",
			lastAccessedAt: "2024-01-05T00:00:00.000Z",
		},
		{
			sessionId: "session-1",
			threadId: "thread-a",
			name: "A",
			lastAccessedAt: "2024-01-04T00:00:00.000Z",
		},
		{
			sessionId: "session-2",
			threadId: "session-2",
			name: "Two",
			lastAccessedAt: "2024-01-03T00:00:00.000Z",
		},
	];
	const recentThreadsB: RecentThreadSummary[] = [
		{
			sessionId: "session-1",
			threadId: "thread-a",
			name: "A",
			lastAccessedAt: "2024-01-05T00:00:00.000Z",
		},
		{
			sessionId: "session-1",
			threadId: "thread-b",
			name: "B",
			lastAccessedAt: "2024-01-04T00:00:00.000Z",
		},
		{
			sessionId: "session-2",
			threadId: "session-2",
			name: "Two",
			lastAccessedAt: "2024-01-03T00:00:00.000Z",
		},
	];

	const visibleRecentThreadsA = getVisibleRecentThreads({
		recentThreads: recentThreadsA,
		sessions: appSessions,
		limit: 3,
	});
	const visibleRecentThreadsB = getVisibleRecentThreads({
		recentThreads: recentThreadsB,
		sessions: appSessions,
		limit: 3,
	});

	assert.deepEqual(
		visibleRecentThreadsA.map(
			(thread) => `${thread.sessionId}:${thread.threadId}`,
		),
		visibleRecentThreadsB.map(
			(thread) => `${thread.sessionId}:${thread.threadId}`,
		),
	);
	assert.deepEqual(
		visibleRecentThreadsA.map(
			(thread) => `${thread.sessionId}:${thread.threadId}`,
		),
		["session-2:session-2", "session-1:thread-a", "session-1:thread-b"],
	);
});
