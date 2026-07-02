import assert from "node:assert/strict";
import { test } from "vitest";

import type { ChatMessage } from "$lib/api-types";
import {
	getReservedTurnMinHeight,
	groupMessagesIntoTurns,
	orderStreamingCompactionMessages,
} from "../app/conversation-pane-layout";

function makeUserMessage(id: string, text: string): ChatMessage {
	return {
		id,
		role: "user",
		parts: [{ type: "text", text }],
	};
}

function makeAssistantMessage(id: string, text: string): ChatMessage {
	return {
		id,
		role: "assistant",
		parts: [{ type: "text", text }],
	};
}

function withTurnId(message: ChatMessage, turnId: string): ChatMessage {
	return {
		...message,
		metadata: {
			...(message.metadata && typeof message.metadata === "object"
				? message.metadata
				: {}),
			discboeing: {
				turnId,
			},
		},
	};
}

function makeCompactionMessage(
	id: string,
	role: ChatMessage["role"],
	compactionFor: string,
): ChatMessage {
	return {
		id,
		role,
		synthetic: true,
		parts: [{ type: "text", text: "Compacted" }],
		metadata: {
			discboeing: {
				kind: "compaction",
				compactionFor,
				turnId: `compaction-${compactionFor}`,
			},
		},
	};
}

test("groupMessagesIntoTurns groups adjacent user messages into one turn", () => {
	const turns = groupMessagesIntoTurns([
		makeUserMessage("user-1", "first"),
		makeUserMessage("user-2", "second"),
	]);

	assert.deepEqual(turns, [
		{
			id: "user-1",
			userMessages: [
				makeUserMessage("user-1", "first"),
				makeUserMessage("user-2", "second"),
			],
			assistantMessages: [],
		},
	]);
});

test("groupMessagesIntoTurns closes a turn with the first assistant message", () => {
	const turns = groupMessagesIntoTurns([
		makeUserMessage("user-1", "prompt"),
		makeAssistantMessage("assistant-1", "reply"),
		makeUserMessage("user-2", "follow-up"),
	]);

	assert.deepEqual(turns, [
		{
			id: "user-1",
			userMessages: [makeUserMessage("user-1", "prompt")],
			assistantMessages: [makeAssistantMessage("assistant-1", "reply")],
		},
		{
			id: "user-2",
			userMessages: [makeUserMessage("user-2", "follow-up")],
			assistantMessages: [],
		},
	]);
});

test("groupMessagesIntoTurns leaves a user-only trailing turn open", () => {
	const turns = groupMessagesIntoTurns([
		makeUserMessage("user-1", "prompt"),
		makeAssistantMessage("assistant-1", "reply"),
		makeUserMessage("user-2", "queued follow-up"),
		makeUserMessage("user-3", "more context"),
	]);

	assert.deepEqual(turns.at(-1), {
		id: "user-2",
		userMessages: [
			makeUserMessage("user-2", "queued follow-up"),
			makeUserMessage("user-3", "more context"),
		],
		assistantMessages: [],
	});
});

test("groupMessagesIntoTurns keeps multiple assistant messages in one turn", () => {
	const turns = groupMessagesIntoTurns([
		makeUserMessage("user-1", "prompt"),
		makeAssistantMessage("assistant-1", "step one"),
		makeAssistantMessage("assistant-2", "final reply"),
	]);

	assert.deepEqual(turns, [
		{
			id: "user-1",
			userMessages: [makeUserMessage("user-1", "prompt")],
			assistantMessages: [
				makeAssistantMessage("assistant-1", "step one"),
				makeAssistantMessage("assistant-2", "final reply"),
			],
		},
	]);
});

test("groupMessagesIntoTurns handles long grouped turns", () => {
	const messages = [
		makeUserMessage("user-1", "prompt"),
		...Array.from({ length: 5000 }, (_, index) =>
			makeAssistantMessage(`assistant-${index}`, `step ${index}`),
		),
	];
	const turns = groupMessagesIntoTurns(messages);

	assert.equal(turns.length, 1);
	assert.equal(turns[0]?.assistantMessages.length, 5000);
	assert.equal(turns[0]?.assistantMessages.at(-1)?.id, "assistant-4999");
});

test("groupMessagesIntoTurns prefers stable backend turn ids from message metadata", () => {
	const turns = groupMessagesIntoTurns([
		withTurnId(makeUserMessage("user-1", "prompt"), "turn-a"),
		withTurnId(makeAssistantMessage("assistant-1", "step one"), "turn-a"),
		withTurnId(makeAssistantMessage("assistant-2", "final"), "turn-a"),
		withTurnId(makeUserMessage("user-2", "follow-up"), "turn-b"),
	]);

	assert.deepEqual(turns, [
		{
			id: "turn-a",
			userMessages: [withTurnId(makeUserMessage("user-1", "prompt"), "turn-a")],
			assistantMessages: [
				withTurnId(makeAssistantMessage("assistant-1", "step one"), "turn-a"),
				withTurnId(makeAssistantMessage("assistant-2", "final"), "turn-a"),
			],
		},
		{
			id: "turn-b",
			userMessages: [
				withTurnId(makeUserMessage("user-2", "follow-up"), "turn-b"),
			],
			assistantMessages: [],
		},
	]);
});

test("groupMessagesIntoTurns assigns unique stable render ids for duplicate ids", () => {
	const turns = groupMessagesIntoTurns([
		withTurnId(makeUserMessage("user-1", "prompt"), "turn-a"),
		withTurnId(makeAssistantMessage("assistant-1", "reply"), "turn-a"),
		withTurnId(makeUserMessage("user-2", "retry"), "turn-b"),
		withTurnId(makeAssistantMessage("assistant-1", "replacement"), "turn-a"),
	]);

	assert.deepEqual(
		turns.map((turn) => turn.renderId),
		["turn-a", "turn-b", "turn-a#2"],
	);
	assert.deepEqual(
		turns.flatMap((turn) =>
			[...turn.userMessages, ...turn.assistantMessages].map(
				(message) => message.renderId,
			),
		),
		["user-1", "assistant-1", "user-2", "assistant-1#2"],
	);
	assert.deepEqual(Object.keys(turns[0] ?? {}), [
		"id",
		"userMessages",
		"assistantMessages",
	]);
	assert.deepEqual(Object.keys(turns[0]?.userMessages[0] ?? {}), [
		"id",
		"role",
		"parts",
		"metadata",
	]);
});

test("orderStreamingCompactionMessages moves compaction before active assistant", () => {
	const streamingAssistant: ChatMessage = {
		...makeAssistantMessage("assistant-streaming", "partial"),
		status: "streaming",
	};
	const compactionUser = makeCompactionMessage(
		"compaction-user",
		"user",
		"assistant-previous",
	);
	const compactionAssistant = makeCompactionMessage(
		"compaction-assistant",
		"assistant",
		"assistant-previous",
	);

	const ordered = orderStreamingCompactionMessages([
		makeUserMessage("user-1", "prompt"),
		streamingAssistant,
		compactionUser,
		compactionAssistant,
	]);

	assert.deepEqual(
		ordered.map((message) => message.id),
		[
			"user-1",
			"compaction-user",
			"compaction-assistant",
			"assistant-streaming",
		],
	);
});

test("orderStreamingCompactionMessages preserves completed history order", () => {
	const messages = [
		makeUserMessage("user-1", "prompt"),
		makeAssistantMessage("assistant-1", "reply"),
		makeCompactionMessage("compaction-user", "user", "assistant-1"),
		makeCompactionMessage("compaction-assistant", "assistant", "assistant-1"),
	];

	assert.equal(orderStreamingCompactionMessages(messages), messages);
});

test("getReservedTurnMinHeight fills the visible viewport when the turn is short", () => {
	assert.equal(
		getReservedTurnMinHeight({
			currentTurnHeight: 124.2,
			contentTopPadding: 12,
			turnTopPadding: 0,
			viewportClientHeight: 640,
			viewportPaddingBottom: 16,
			viewportPaddingTop: 16,
		}),
		596,
	);
});

test("getReservedTurnMinHeight compensates for turn top padding", () => {
	assert.equal(
		getReservedTurnMinHeight({
			currentTurnHeight: 124.2,
			contentTopPadding: 12,
			turnTopPadding: 64,
			viewportClientHeight: 640,
			viewportPaddingBottom: 16,
			viewportPaddingTop: 16,
		}),
		660,
	);
});

test("getReservedTurnMinHeight preserves taller turns", () => {
	assert.equal(
		getReservedTurnMinHeight({
			currentTurnHeight: 712.4,
			contentTopPadding: 24,
			turnTopPadding: 0,
			viewportClientHeight: 640,
			viewportPaddingBottom: 16,
			viewportPaddingTop: 16,
		}),
		713,
	);
});
