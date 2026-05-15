import { expect, test } from "vitest";

import { StartChatError } from "$lib/api-client";
import type { ChatMessage, PendingQuestion } from "$lib/api-types";
import {
	buildPendingQuestionFallbackToolPart,
	getHasPendingQuestion,
	isStartChatPendingQuestionError,
	resolvePendingQuestionToolName,
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

test("getHasPendingQuestion respects the thread snapshot pending flag", () => {
	expect(getHasPendingQuestion([], false, true)).toBe(true);
});

test("resolvePendingQuestionToolName maps commit-pull approvals", () => {
	const question = {
		toolUseID: "approval-1",
		context: "request_commit_pull",
	} as PendingQuestion;

	expect(resolvePendingQuestionToolName(question)).toBe("RequestCommitPull");
});

test("resolvePendingQuestionToolName maps credential approvals", () => {
	const question = {
		toolUseID: "approval-1",
		credentials: [
			{
				envVar: "GITHUB_TOKEN",
				name: "GitHub token",
				justification: "clone repo",
				approvedUses: [],
			},
		],
	} as PendingQuestion;

	expect(resolvePendingQuestionToolName(question)).toBe(
		"RequestUserCredential",
	);
});

test("buildPendingQuestionFallbackToolPart preserves approval metadata", () => {
	const question = {
		toolUseID: "approval-1",
		questions: [
			{
				header: "Pick one",
				question: "Which option?",
				options: [],
				multiSelect: false,
			},
		],
	} as PendingQuestion;

	expect(buildPendingQuestionFallbackToolPart(question)).toEqual({
		type: "dynamic-tool",
		toolCallId: "approval-1",
		toolName: "AskUserQuestion",
		state: "approval-requested",
		input: {
			questions: question.questions,
		},
		approval: { id: "approval-1" },
	});
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
