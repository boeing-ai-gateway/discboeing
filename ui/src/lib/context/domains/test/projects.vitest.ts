import type { ProjectEventsStreamMessage, Session } from "$lib/api-types";
import { expect, test } from "vitest";

import type { Context } from "$lib/context/context.types";
import { createSessionRecord } from "$lib/context/domains/sessions";
import { applyProjectEvent } from "$lib/context/domains/projects";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

test("removed selected session clears selected session and thread ids", () => {
	const context = createPlainContext();
	context.data.sessions.byId["session-1"] = createSessionRecord("session-1", {
		id: "session-1",
		sandboxStatus: "ready",
	} as Session);
	context.data.sessions.allIds = ["session-1"];
	context.view.selection.sessionId = "session-1";
	context.view.selection.threadId = "thread-1";
	context.view.selection.requestedThreadIdBySessionId["session-1"] = "thread-1";

	applyProjectEvent(
		context,
		context.data,
		createSessionRemovedEvent("session-1"),
	);

	expect(context.data.sessions.byId["session-1"]).toBeUndefined();
	expect(context.data.sessions.allIds).toEqual([]);
	expect(context.view.selection.sessionId).toBeNull();
	expect(context.view.selection.threadId).toBeNull();
	expect(context.view.selection.requestedThreadIdBySessionId).toEqual({});
});

test("removed unselected session keeps current selection", () => {
	const context = createPlainContext();
	context.data.sessions.byId["session-1"] = createSessionRecord("session-1", {
		id: "session-1",
		sandboxStatus: "ready",
	} as Session);
	context.data.sessions.byId["session-2"] = createSessionRecord("session-2", {
		id: "session-2",
		sandboxStatus: "ready",
	} as Session);
	context.data.sessions.allIds = ["session-1", "session-2"];
	context.view.selection.sessionId = "session-1";
	context.view.selection.threadId = "thread-1";
	context.view.selection.requestedThreadIdBySessionId["session-1"] = "thread-1";

	applyProjectEvent(
		context,
		context.data,
		createSessionRemovedEvent("session-2"),
	);

	expect(context.data.sessions.byId["session-2"]).toBeUndefined();
	expect(context.data.sessions.allIds).toEqual(["session-1"]);
	expect(context.view.selection.sessionId).toBe("session-1");
	expect(context.view.selection.threadId).toBe("thread-1");
	expect(context.view.selection.requestedThreadIdBySessionId).toEqual({
		"session-1": "thread-1",
	});
});

function createPlainContext(): Context {
	return {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState(),
		commands: {} as Context["commands"],
	};
}

function createSessionRemovedEvent(
	sessionId: string,
): ProjectEventsStreamMessage {
	return {
		type: "event",
		stream: "project-events",
		event: "session_updated",
		data: {
			id: "event-1",
			seq: 1,
			type: "session_updated",
			timestamp: "2026-06-11T00:00:00.000Z",
			data: {
				id: sessionId,
				sandboxStatus: "removed",
			} as Session,
		},
	};
}
