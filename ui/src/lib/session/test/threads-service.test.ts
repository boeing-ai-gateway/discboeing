import assert from "node:assert/strict";
import test from "node:test";

import type { Session } from "../../api-types";
import {
	buildImplicitThread,
	getNextSelectedThreadId,
} from "../domains/session-domain.helpers";

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
		name: "Session",
		description: "",
		createdAt: "2026-03-10T00:00:00.000Z",
		timestamp: "2026-03-11T00:00:00.000Z",
		status: "creating_sandbox",
		files: [],
		threadStatus: {
			status: "running",
			threadId: "thread-1",
		},
	};

	assert.deepEqual(buildImplicitThread(session), [
		{
			id: "session-1",
			name: "Session",
			model: undefined,
			reasoning: undefined,
			promptQueue: [],
		},
	]);
});
