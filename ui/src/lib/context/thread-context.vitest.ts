import { expect, test } from "vitest";

import {
	applyStreamedThreadUpdate,
	clearComposerDraftState,
	getThreadComposerValues,
	getThreadConversationStatus,
	getThreadIsStreaming,
	normalizeThreadComposerReasoning,
	normalizeThreadComposerServiceTier,
	parseComposerModelSelection,
	resolveThreadComposerSubmitValues,
} from "./thread-context.svelte";
import { isThreadSnapshotRunning } from "$lib/app/thread-status";

test("applyStreamedThreadUpdate syncs the primary session title and reloads it", async () => {
	const upserted: string[] = [];
	const synced: string[] = [];
	let reloaded = false;

	applyStreamedThreadUpdate({
		sessionId: "session-1",
		sessionName: "New Session",
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
		sessionName: "Pinned session",
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
		sessionName: "Pinned session",
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

test("clearComposerDraftState clears storage before resetting the in-memory draft", () => {
	const calls: string[] = [];
	let composerDraft = "Start a new session with this prompt";

	clearComposerDraftState({
		cancelPersist: () => {
			calls.push("cancel");
		},
		clearStoredDraft: () => {
			calls.push("storage");
			expect(composerDraft).toBe("Start a new session with this prompt");
		},
		clearInMemoryDraft: () => {
			calls.push("memory");
			composerDraft = "";
		},
	});

	expect(calls).toEqual(["cancel", "storage", "memory"]);
	expect(composerDraft).toBe("");
});

test("getThreadComposerValues restores thread model and reasoning", () => {
	expect(
		getThreadComposerValues(
			{
				id: "thread-1",
				name: "Main",
				model: "openai/gpt-5",
				reasoning: "high",
				serviceTier: "priority",
			},
			"anthropic/claude-sonnet-4-6",
		),
	).toEqual({
		modelId: "openai/gpt-5",
		reasoning: "high",
		serviceTier: "priority",
	});
});

test("getThreadComposerValues falls back to the default model", () => {
	expect(getThreadComposerValues(null, "anthropic/claude-sonnet-4-6")).toEqual({
		modelId: "anthropic/claude-sonnet-4-6",
		reasoning: undefined,
		serviceTier: undefined,
	});
});

test("normalizeThreadComposerReasoning keeps explicit levels and drops empty values", () => {
	expect(normalizeThreadComposerReasoning("default")).toBe("default");
	expect(normalizeThreadComposerReasoning("medium")).toBe("medium");
	expect(normalizeThreadComposerReasoning("")).toBe(undefined);
	expect(normalizeThreadComposerReasoning(undefined)).toBe(undefined);
});

test("normalizeThreadComposerServiceTier keeps explicit tiers and drops empty values", () => {
	expect(normalizeThreadComposerServiceTier("priority")).toBe("priority");
	expect(normalizeThreadComposerServiceTier("fast")).toBe("priority");
	expect(normalizeThreadComposerServiceTier("")).toBe(undefined);
	expect(normalizeThreadComposerServiceTier(undefined)).toBe(undefined);
});

test("parseComposerModelSelection keeps only the model identifier", () => {
	expect(parseComposerModelSelection("openai/gpt-5")).toEqual({
		modelId: "openai/gpt-5",
	});
	expect(parseComposerModelSelection(null)).toEqual({
		modelId: null,
	});
});

test("resolveThreadComposerSubmitValues falls back to current values when next values are unset", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "openai/gpt-5",
			reasoning: "high",
			serviceTier: "priority",
			nextModelId: undefined,
			nextReasoning: undefined,
			nextServiceTier: undefined,
		}),
	).toEqual({
		modelId: "openai/gpt-5",
		reasoning: "high",
		serviceTier: "priority",
	});
});

test("resolveThreadComposerSubmitValues clears reasoning when using the default model", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "openai/gpt-5",
			reasoning: "high",
			serviceTier: "priority",
			nextModelId: null,
			nextReasoning: "default",
			nextServiceTier: "priority",
		}),
	).toEqual({
		modelId: null,
		reasoning: undefined,
		serviceTier: undefined,
	});
});

test("resolveThreadComposerSubmitValues prefers staged next values", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "anthropic/claude-sonnet-4-6",
			reasoning: "auto",
			serviceTier: undefined,
			nextModelId: "openai/gpt-5",
			nextReasoning: "high",
			nextServiceTier: "priority",
		}),
	).toEqual({
		modelId: "openai/gpt-5",
		reasoning: "high",
		serviceTier: "priority",
	});
});

test("resolveThreadComposerSubmitValues allows staged standard service tier", () => {
	expect(
		resolveThreadComposerSubmitValues({
			modelId: "codex/gpt-5.5",
			reasoning: "default",
			serviceTier: "priority",
			nextModelId: undefined,
			nextReasoning: undefined,
			nextServiceTier: null,
		}),
	).toEqual({
		modelId: "codex/gpt-5.5",
		reasoning: "default",
		serviceTier: undefined,
	});
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

test("getThreadIsStreaming keeps stop control visible while server says running", () => {
	expect(
		getThreadIsStreaming(
			"thread-1",
			{
				id: "thread-1",
				name: "Main",
				activityStatus: { status: "running" },
			},
			false,
		),
	).toBe(true);
	expect(getThreadConversationStatus("error")).toBe("error");
	expect(
		getThreadIsStreaming(
			"thread-1",
			{
				id: "thread-1",
				name: "Main",
			},
			false,
			{ status: "running", threadId: "thread-1" },
		),
	).toBe(true);
	expect(
		getThreadIsStreaming(
			"thread-2",
			{
				id: "thread-2",
				name: "Other",
			},
			false,
			{ status: "running", threadId: "thread-1" },
		),
	).toBe(false);
});
