import assert from "node:assert/strict";
import { test } from "vitest";

import type { Session } from "$lib/api-types";
import {
	buildImplicitThread,
	getNextSelectedThreadId,
} from "$lib/conversation-helpers";

test("getNextSelectedThreadId picks the next thread after removal", () => {
	const nextId = getNextSelectedThreadId(
		[
			{ id: "a", name: "A" },
			{ id: "b", name: "B" },
			{ id: "c", name: "C" },
		],
		"b",
		"b",
	);

	assert.equal(nextId, "c");
});

test("getNextSelectedThreadId falls back to the previous thread", () => {
	const nextId = getNextSelectedThreadId(
		[
			{ id: "a", name: "A" },
			{ id: "b", name: "B" },
		],
		"b",
		"b",
	);

	assert.equal(nextId, "a");
});

test("buildImplicitThread uses the session id while threads are unavailable", () => {
	const session: Session = {
		id: "session-1",
		projectId: "project-1",
		workspaceId: "workspace-1",
		name: "Session",
		description: "",
		createdAt: "2026-03-10T00:00:00.000Z",
		updatedAt: "2026-03-11T00:00:00.000Z",
		sandboxStatus: "creating_sandbox",
		commitStatus: "",
		threadStatus: {
			status: "running",
			threadId: "thread-1",
		},
	};

	assert.deepEqual(buildImplicitThread(session), [
		{
			id: "session-1",
			name: "Session",
			promptQueue: [],
		},
	]);
});
