import { expect, test } from "vitest";

import { StartChatError } from "$lib/api-client";
import type { ChatMessage } from "$lib/api-types";
import {
	getHasPendingQuestion,
	isStartChatPendingQuestionError,
} from "./thread-pending-question.svelte";

test("getHasPendingQuestion detects pending approvals from messages", () => {
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

	expect(getHasPendingQuestion(messages, false)).toBe(true);
});

test("getHasPendingQuestion keeps submit-error pending state without messages", () => {
	expect(getHasPendingQuestion([], true)).toBe(true);
});

test("isStartChatPendingQuestionError detects pending-question errors", () => {
	const error = new StartChatError(
		"Answer the earlier question first.",
		"pending_question_requires_answer",
		"approval-123",
	);

	expect(isStartChatPendingQuestionError(error)).toBe(true);
});

test("isStartChatPendingQuestionError ignores non-pending chat errors", () => {
	const error = new StartChatError(
		"Failed to start chat",
		"request_failed",
		undefined,
		"resume-123",
	);

	expect(isStartChatPendingQuestionError(error)).toBe(false);
});
