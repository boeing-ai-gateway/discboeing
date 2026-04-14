import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

import { StartChatError } from "$lib/api-client";
import type { ChatMessage } from "$lib/api-types";
import {
	getPendingQuestionState,
	getStartChatErrorDetails,
	getSubmitMessages,
	removeProvisionalSubmitMessage,
} from "./conversation.svelte";

const CONVERSATION_DOMAIN_SOURCE = path.resolve(
	import.meta.dirname,
	"./conversation.svelte.ts",
);

function makeUserMessage(
	parts: ChatMessage["parts"] = [{ type: "text", text: "latest prompt" }],
	provisional = false,
): ChatMessage {
	return {
		id: "user-1",
		role: "user",
		parts,
		...(provisional ? { provisional: true } : {}),
	};
}

test("getSubmitMessages only includes the latest user message", () => {
	const userMessage = makeUserMessage();

	assert.deepEqual(getSubmitMessages(userMessage), [userMessage]);
});

test("getSubmitMessages strips the provisional flag", () => {
	const userMessage = makeUserMessage(
		[{ type: "text", text: "latest prompt" }],
		true,
	);

	assert.deepEqual(getSubmitMessages(userMessage), [
		{
			id: "user-1",
			role: "user",
			parts: [{ type: "text", text: "latest prompt" }],
		},
	]);
});

test("getSubmitMessages preserves attachment parts", () => {
	const userMessage = makeUserMessage(
		[
			{ type: "text", text: "latest prompt" },
			{
				type: "file",
				filename: "preview.png",
				mediaType: "image/png",
				url: "data:image/png;base64,abc123",
			},
		],
		true,
	);

	assert.deepEqual(getSubmitMessages(userMessage), [
		{
			id: "user-1",
			role: "user",
			parts: [
				{ type: "text", text: "latest prompt" },
				{
					type: "file",
					filename: "preview.png",
					mediaType: "image/png",
					url: "data:image/png;base64,abc123",
				},
			],
		},
	]);
});

test("removeProvisionalSubmitMessage removes the failed optimistic message", () => {
	const failedMessage = makeUserMessage(
		[{ type: "text", text: "pending" }],
		true,
	);
	const keptMessage: ChatMessage = {
		id: "assistant-1",
		role: "assistant",
		parts: [{ type: "text", text: "existing" }],
	};

	assert.deepEqual(
		removeProvisionalSubmitMessage(
			[failedMessage, keptMessage],
			failedMessage.id,
		),
		[keptMessage],
	);
});

test("getPendingQuestionState prefers the pending approval from messages", () => {
	const messages: ChatMessage[] = [
		{
			id: "assistant-1",
			role: "assistant",
			parts: [
				{
					type: "dynamic-tool",
					toolCallId: "tool-1",
					toolName: "AskUserQuestion",
					state: "approval-requested",
					approval: { id: "approval-from-messages" },
					input: { questions: [] },
				},
			],
		},
	];

	assert.deepEqual(getPendingQuestionState(messages, "approval-from-error"), {
		hasPendingQuestion: true,
		pendingQuestionId: "approval-from-messages",
	});
});

test("getStartChatErrorDetails extracts pending-question metadata", () => {
	const error = new StartChatError(
		"Answer the earlier question first.",
		"pending_question_requires_answer",
		"approval-123",
	);

	assert.deepEqual(getStartChatErrorDetails(error), {
		message: "Answer the earlier question first.",
		pendingQuestionId: "approval-123",
		completionId: null,
	});
});

test("getStartChatErrorDetails suppresses the auto-resume conflict message", () => {
	const error = new StartChatError(
		"This thread has an interrupted turn that must resume before sending a new message.",
		"interrupted_turn_requires_resume",
		undefined,
		"resume-123",
	);

	assert.deepEqual(getStartChatErrorDetails(error), {
		message: null,
		pendingQuestionId: null,
		completionId: "resume-123",
	});
});

test("conversation loader derives running state from websocket-backed lifecycle state", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(source, /completionRunning/);
	assert.match(source, /onCompletionStatus/);
	assert.match(source, /app\.chatStreams\.subscribe/);
	assert.match(source, /onHistoryReplayEnd/);
	assert.doesNotMatch(source, /getThreadMessages/);
	assert.doesNotMatch(source, /isStreamingAssistantMessage/);
	assert.doesNotMatch(source, /hasStreamingAssistantMessage/);
});

test("conversation loader no longer treats websocket close as an immediate fatal error", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/const LOST_PROJECT_STREAM_CONNECTION_MESSAGE = "Lost project stream connection";/,
	);
	assert.match(source, /if \(isLostProjectStreamConnection\(error\)\) \{/);
	assert.match(
		source,
		/if \(loadStatus !== "loading"\) \{[\s\S]*streamError = errorMessage;/,
	);
	assert.doesNotMatch(source, /Lost chat stream connection/);
});

test("conversation loader keeps closed-stream recovery at the websocket layer", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(source, /shouldIgnoreClosedStreamError\?: \(\) => boolean;/);
	assert.doesNotMatch(
		source,
		/disconnectStream\(\);[\s\S]*shouldIgnoreClosedStreamError/,
	);
	assert.match(source, /if \(fatalStreamError && !forceResubscribe\) \{/);
	assert.match(
		source,
		/if \(activeStreamKey === streamKey\(args\.sessionId\)\) \{[\s\S]*if \(forceResubscribe\) \{[\s\S]*fatalStreamError = false;[\s\S]*activeSubscription\?\.resubscribe\(\);/,
	);
});

test("conversation loader only clears stream errors when a completion starts", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/onCompletionStatus: \(\{ isRunning \}\) => \{[\s\S]*if \(isRunning\) \{[\s\S]*streamError = null;[\s\S]*\}[\s\S]*completionRunning = isRunning;/,
	);
});

test("conversation loader keeps websocket disconnects transient while preserving fatal stream parse errors", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/const LOST_PROJECT_STREAM_CONNECTION_MESSAGE = "Lost project stream connection";/,
	);
	assert.match(
		source,
		/function isLostProjectStreamConnection\(error: unknown\): boolean/,
	);
	assert.match(
		source,
		/onError: \(error\) => \{[\s\S]*if \(isLostProjectStreamConnection\(error\)\) \{[\s\S]*return;[\s\S]*\}[\s\S]*fatalStreamError = true/,
	);
	assert.match(
		source,
		/onError: \(error\) => \{[\s\S]*fatalStreamError = true[\s\S]*disconnectStream\(\)/,
	);
	assert.match(source, /if \(fatalStreamError && !forceResubscribe\) \{/);
});
