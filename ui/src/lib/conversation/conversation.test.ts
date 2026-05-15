import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

import type { ChatMessage } from "$lib/api-types";
import {
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

test("conversation loader derives running state from websocket-backed lifecycle state", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(source, /completionRunning/);
	assert.match(source, /onCompletionStatus/);
	assert.match(source, /args\.projectStreams\.subscribe/);
	assert.doesNotMatch(source, /getThreadMessages/);
	assert.doesNotMatch(source, /isStreamingAssistantMessage/);
	assert.doesNotMatch(source, /hasStreamingAssistantMessage/);
});

test("conversation loader keeps stream errors retryable at the project stream layer", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/onError: \(error\) => \{[\s\S]*streamError = getStreamErrorMessage\(error\);/,
	);
	assert.doesNotMatch(source, /fatalStreamError/);
	assert.doesNotMatch(source, /isLostProjectStreamConnection/);
	assert.doesNotMatch(source, /Lost chat stream connection/);
});

test("conversation connect starts streams without awaiting readiness", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/function connect\(\) \{[\s\S]*streamError = null;[\s\S]*ensureStream\(\);[\s\S]*return Promise\.resolve\(\);/,
	);
	assert.doesNotMatch(source, /function refresh\(\)/);
	assert.doesNotMatch(source, /loadStatus/);
	assert.doesNotMatch(source, /startStreamLoad/);
	assert.doesNotMatch(source, /pendingLoadPromise/);
	assert.doesNotMatch(source, /beginLoadPromise/);
});

test("conversation submit leaves stream connection decisions to the caller", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/console\.debug\("\[WS\] Preparing chat submit"[\s\S]*const response = await args\.startChat\(/,
	);
	assert.doesNotMatch(source, /hasSession: \(\) => boolean/);
	assert.doesNotMatch(source, /canStream\?: \(\) => boolean/);
	assert.doesNotMatch(source, /args\.hasSession/);
	assert.doesNotMatch(source, /args\.canStream/);
	assert.doesNotMatch(source, /resubscribeIdleStream/);
	assert.doesNotMatch(source, /\.resubscribe\(/);
	assert.doesNotMatch(source, /\.getState\(/);
	assert.doesNotMatch(source, /allowEmptyPendingMessage/);
});

test("conversation loader falls back to stream finish when completion status never clears", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/const handleCompletionStart = \(\) => \{[\s\S]*if \(completionRunning\) \{[\s\S]*return;[\s\S]*completionRunning = true;/,
	);
	assert.match(
		source,
		/const handleCompletionFinish = \(\) => \{[\s\S]*if \(!completionRunning\) \{[\s\S]*return;[\s\S]*completionRunning = false;[\s\S]*dismissRetryToast\(args\.threadId\);[\s\S]*args\.afterTurn\?\.\(\);/,
	);
	assert.doesNotMatch(source, /afterTurnPending/);
	assert.doesNotMatch(source, /runAfterTurnIfNeeded/);
	assert.match(source, /onStart: \(\) => \{[\s\S]*handleCompletionStart\(\);/);
	assert.match(
		source,
		/onCompletionStatus: \(\{ isRunning \}\) => \{[\s\S]*if \(isRunning\) \{[\s\S]*handleCompletionStart\(\);[\s\S]*return;[\s\S]*\}[\s\S]*handleCompletionFinish\(\);/,
	);
	assert.match(
		source,
		/onFinish: \(\) => \{[\s\S]*handleCompletionFinish\(\);/,
	);
});

test("conversation loader clears the running flag when disposed", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/dispose: \(\) => \{[\s\S]*completionRunning = false;[\s\S]*disconnectStream\(\);/,
	);
	assert.doesNotMatch(source, /function disconnect\(\)/);
});

test("conversation loader tracks browser events by turn id", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(source, /browserEventsByTurnId/);
	assert.match(source, /onBrowserEvent:/);
	assert.match(source, /turnId = event\.turnId\?\.trim\(\)/);
});

test("conversation cancel runs cancellation in the background", () => {
	const source = readFileSync(CONVERSATION_DOMAIN_SOURCE, "utf-8");

	assert.match(
		source,
		/cancel: \(\) => \{[\s\S]*void api\.cancelThreadChat\(args\.sessionId, args\.threadId\)\.then\([\s\S]*args\.refreshThread\(\)[\s\S]*return Promise\.resolve\(\);/,
	);
	assert.doesNotMatch(source, /void refresh\(\);/);
	assert.doesNotMatch(source, /await api\.cancelThreadChat/);
});
