import assert from "node:assert/strict";
import test from "node:test";

import {
	applyStreamedThreadUpdate,
	clearComposerDraftState,
	getThreadComposerValues,
	getThreadConversationStatus,
	normalizeThreadComposerReasoning,
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

	assert.deepEqual(upserted, ["Fix flaky sidebar refresh"]);
	assert.deepEqual(synced, ["Fix flaky sidebar refresh"]);
	assert.equal(reloaded, true);
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

	assert.equal(reloadCount, 0);
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
			assert.equal(composerDraft, "Start a new session with this prompt");
		},
		clearInMemoryDraft: () => {
			calls.push("memory");
			composerDraft = "";
		},
	});

	assert.deepEqual(calls, ["cancel", "storage", "memory"]);
	assert.equal(composerDraft, "");
});

test("getThreadComposerValues restores thread model and reasoning", () => {
	assert.deepEqual(
		getThreadComposerValues(
			{
				id: "thread-1",
				name: "Main",
				model: "openai/gpt-5",
				reasoning: "high",
			},
			"anthropic/claude-sonnet-4-6",
		),
		{
			modelId: "openai/gpt-5",
			reasoning: "high",
		},
	);
});

test("getThreadComposerValues falls back to the default model", () => {
	assert.deepEqual(
		getThreadComposerValues(null, "anthropic/claude-sonnet-4-6"),
		{
			modelId: "anthropic/claude-sonnet-4-6",
			reasoning: undefined,
		},
	);
});

test("normalizeThreadComposerReasoning keeps explicit levels and drops empty values", () => {
	assert.equal(normalizeThreadComposerReasoning("default"), "default");
	assert.equal(normalizeThreadComposerReasoning("medium"), "medium");
	assert.equal(normalizeThreadComposerReasoning(""), undefined);
	assert.equal(normalizeThreadComposerReasoning(undefined), undefined);
});

test("parseComposerModelSelection keeps only the model identifier", () => {
	assert.deepEqual(parseComposerModelSelection("openai/gpt-5"), {
		modelId: "openai/gpt-5",
	});
	assert.deepEqual(parseComposerModelSelection(null), {
		modelId: null,
	});
});

test("resolveThreadComposerSubmitValues falls back to current values when next values are unset", () => {
	assert.deepEqual(
		resolveThreadComposerSubmitValues({
			modelId: "openai/gpt-5",
			reasoning: "high",
			nextModelId: undefined,
			nextReasoning: undefined,
		}),
		{
			modelId: "openai/gpt-5",
			reasoning: "high",
		},
	);
});

test("resolveThreadComposerSubmitValues clears reasoning when using the default model", () => {
	assert.deepEqual(
		resolveThreadComposerSubmitValues({
			modelId: "openai/gpt-5",
			reasoning: "high",
			nextModelId: null,
			nextReasoning: "default",
		}),
		{
			modelId: null,
			reasoning: undefined,
		},
	);
});

test("resolveThreadComposerSubmitValues prefers staged next values", () => {
	assert.deepEqual(
		resolveThreadComposerSubmitValues({
			modelId: "anthropic/claude-sonnet-4-6",
			reasoning: "auto",
			nextModelId: "openai/gpt-5",
			nextReasoning: "high",
		}),
		{
			modelId: "openai/gpt-5",
			reasoning: "high",
		},
	);
});

test("isThreadSnapshotRunning detects server-side activity", () => {
	assert.equal(
		isThreadSnapshotRunning({
			id: "thread-1",
			name: "Main",
			activityStatus: { status: "running" },
		}),
		true,
	);
	assert.equal(
		isThreadSnapshotRunning({
			id: "thread-1",
			name: "Main",
			activeCommand: "pnpm test",
		}),
		true,
	);
	assert.equal(
		isThreadSnapshotRunning({
			id: "thread-1",
			name: "Main",
			activityStatus: { status: "idle" },
		}),
		false,
	);
});

test("getThreadConversationStatus keeps stop control visible while server says running", () => {
	assert.equal(
		getThreadConversationStatus(
			{
				id: "thread-1",
				name: "Main",
				activityStatus: { status: "running" },
			},
			"ready",
		),
		"streaming",
	);
	assert.equal(
		getThreadConversationStatus(
			{
				id: "thread-1",
				name: "Main",
				activityStatus: { status: "running" },
			},
			"error",
		),
		"error",
	);
});
