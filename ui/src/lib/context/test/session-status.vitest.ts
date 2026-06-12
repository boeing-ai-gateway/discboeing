import { describe, expect, test } from "vitest";

import { CommitStatus } from "$lib/api-constants";
import type { Session } from "$lib/api-types";
import type { Thread } from "$lib/api-types";
import { createSessionRecord } from "$lib/context/domains/sessions";
import {
	resolveSessionDisplayStatus,
	resolveThreadDisplayStatus,
} from "$lib/session-status";
import {
	createThreadRecord,
	ensureThreadContentState,
} from "$lib/context/domains/threads";

describe("resolveSessionDisplayStatus", () => {
	test("returns unknown when no ng session record is available", () => {
		expect(resolveSessionDisplayStatus(undefined)).toBe("unknown");
	});

	test("maps ng session records through session display status rules", () => {
		const record = createSessionRecord("session-1", {
			id: "session-1",
			sandboxStatus: "ready",
			threadStatus: { status: "running" },
		} as Session);

		expect(resolveSessionDisplayStatus(record)).toBe("running");
	});

	test("preserves commit status precedence", () => {
		const record = createSessionRecord("session-1", {
			id: "session-1",
			sandboxStatus: "ready",
			commitStatus: CommitStatus.PENDING,
			threadStatus: { status: "running" },
		} as Session);

		expect(resolveSessionDisplayStatus(record)).toBe("pending");
	});

	test("returns unknown thread status when no ng session record is available", () => {
		expect(resolveThreadDisplayStatus(undefined, "thread-1")).toBe("unknown");
	});

	test("uses ng thread content activity for thread status", () => {
		const record = createSessionRecord("session-1", {
			id: "session-1",
			sandboxStatus: "ready",
		} as Session);
		record.threads.byId["thread-1"] = createThreadRecord("thread-1", {
			id: "thread-1",
			name: "Thread 1",
		} as Thread);
		record.threads.allIds.push("thread-1");
		ensureThreadContentState(record.threads, "thread-1").isStreaming = true;

		expect(resolveThreadDisplayStatus(record, "thread-1")).toBe("running");
	});

	test("uses matching session thread activity for thread status", () => {
		const record = createSessionRecord("session-1", {
			id: "session-1",
			sandboxStatus: "ready",
			threadStatus: { status: "running", threadId: "thread-1" },
		} as Session);

		expect(resolveThreadDisplayStatus(record, "thread-1")).toBe("running");
		expect(resolveThreadDisplayStatus(record, "thread-2")).toBe("idle");
	});
});
