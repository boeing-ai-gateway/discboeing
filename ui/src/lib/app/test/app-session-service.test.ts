import assert from "node:assert/strict";
import test from "node:test";

import type { Session } from "$lib/api-types";
import {
	getNextSelectedSessionId,
	sortSessionsByCreatedAt,
	upsertSession,
} from "../domains/app-sessions.helpers";

const sessions = [
	{ id: "session-1" },
	{ id: "session-2" },
	{ id: "session-3" },
];

function makeSession(overrides: Partial<Session> = {}): Session {
	return {
		id: "session-1",
		name: "Session",
		description: "",
		createdAt: "2026-01-01T00:00:00Z",
		timestamp: "2026-01-01T00:00:00Z",
		status: "ready",
		files: [],
		...overrides,
	};
}

test("getNextSelectedSessionId keeps the current selection when another session is deleted", () => {
	assert.equal(
		getNextSelectedSessionId(sessions, "session-1", "session-2"),
		"session-2",
	);
});

test("getNextSelectedSessionId falls back to the first remaining session", () => {
	assert.equal(
		getNextSelectedSessionId(sessions, "session-2", "session-2"),
		"session-1",
	);
});

test("getNextSelectedSessionId returns null when the last session is deleted", () => {
	assert.equal(
		getNextSelectedSessionId([{ id: "session-1" }], "session-1", "session-1"),
		null,
	);
});

test("sortSessionsByCreatedAt sorts all sessions by createdAt descending", () => {
	const sorted = sortSessionsByCreatedAt([
		makeSession({
			id: "session-1",
			name: "Oldest",
			createdAt: "2026-01-01T00:00:00Z",
		}),
		makeSession({
			id: "session-2",
			name: "Newest",
			createdAt: "2026-01-03T00:00:00Z",
		}),
		makeSession({
			id: "session-3",
			name: "Middle",
			createdAt: "2026-01-02T00:00:00Z",
		}),
	]);

	assert.deepEqual(
		sorted.map((session) => session.id),
		["session-2", "session-3", "session-1"],
	);
});

test("sortSessionsByCreatedAt preserves session objects", () => {
	const sorted = sortSessionsByCreatedAt([
		makeSession({
			id: "session-1",
			name: "Workspace session",
			workspaceId: "workspace-1",
		}),
	]);

	assert.equal(sorted[0]?.workspaceId, "workspace-1");
});

test("upsertSession replaces an existing session in place", () => {
	const existingSessions: Session[] = [
		makeSession({
			id: "session-1",
			name: "One",
			createdAt: "2026-01-01T00:00:00Z",
		}),
		makeSession({
			id: "session-2",
			name: "Two",
			createdAt: "2026-01-02T00:00:00Z",
		}),
	];

	const nextSession: Session = {
		...existingSessions[1],
		status: "error",
		errorMessage: "boom",
	};

	assert.deepEqual(upsertSession(existingSessions, nextSession), [
		existingSessions[0],
		nextSession,
	]);
});

test("upsertSession appends a newly seen session", () => {
	const existingSessions: Session[] = [
		makeSession({
			id: "session-1",
			name: "One",
			createdAt: "2026-01-01T00:00:00Z",
		}),
	];

	const nextSession: Session = makeSession({
		id: "session-2",
		name: "Two",
		createdAt: "2026-01-02T00:00:00Z",
		timestamp: "2026-01-02T00:00:00Z",
	});

	assert.deepEqual(upsertSession(existingSessions, nextSession), [
		existingSessions[0],
		nextSession,
	]);
});
