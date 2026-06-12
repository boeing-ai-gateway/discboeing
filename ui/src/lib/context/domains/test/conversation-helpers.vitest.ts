import assert from "node:assert/strict";
import { test } from "vitest";

import type { ChatMessage } from "$lib/api-types";

import {
	buildUserMessageParts,
	createUserMessage,
	createUserMessageAttachment,
	getPendingQuestionApprovalId,
	getPreviousTodoWriteEntries,
	hasUserMessageContent,
	sortServiceItems,
	toServiceItem,
} from "$lib/conversation-helpers";

test("sortServiceItems prioritizes explicit order before alphabetical fallbacks", () => {
	const services = [
		toServiceItem({
			id: "beta",
			name: "Beta",
			status: "stopped",
			path: "/tmp/beta.sh",
		}),
		toServiceItem({
			id: "zeta",
			name: "Zeta",
			order: 20,
			status: "stopped",
			path: "/tmp/zeta.sh",
		}),
		toServiceItem({
			id: "alpha",
			name: "Alpha",
			status: "stopped",
			path: "/tmp/alpha.sh",
		}),
		toServiceItem({
			id: "eta",
			name: "Eta",
			order: 10,
			status: "stopped",
			path: "/tmp/eta.sh",
		}),
		toServiceItem({
			id: "theta",
			name: "Theta",
			order: 10,
			status: "stopped",
			path: "/tmp/theta.sh",
		}),
	];

	assert.deepEqual(
		sortServiceItems(services).map((service) => service.id),
		["eta", "theta", "zeta", "alpha", "beta"],
	);
});

test("createUserMessage can mark a message as provisional", () => {
	const message = createUserMessage("hello", { provisional: true });

	assert.equal(message.role, "user");
	assert.deepEqual(message.parts, [{ type: "text", text: "hello" }]);
	assert.equal(message.provisional, true);
});

test("getPendingQuestionApprovalId returns the latest pending approval id", () => {
	const messages: ChatMessage[] = [
		{
			id: "assistant-1",
			role: "assistant",
			parts: [
				{
					type: "dynamic-tool",
					toolCallId: "call-old",
					toolName: "AskUserQuestion",
					state: "approval-responded",
					approval: { id: "approval-old", approved: true },
					input: { questions: [] },
				},
			],
		},
		{
			id: "assistant-2",
			role: "assistant",
			parts: [
				{
					type: "dynamic-tool",
					toolCallId: "call-new",
					toolName: "AskUserQuestion",
					state: "approval-requested",
					approval: { id: "approval-new" },
					input: { questions: [] },
				},
			],
		},
	];

	assert.equal(getPendingQuestionApprovalId(messages), "approval-new");
});

test("getPendingQuestionApprovalId ignores approval ids answered by stream", () => {
	const messages: ChatMessage[] = [
		{
			id: "assistant-1",
			role: "assistant",
			parts: [
				{
					type: "dynamic-tool",
					toolCallId: "call-new",
					toolName: "AskUserQuestion",
					state: "approval-requested",
					approval: { id: "approval-new" },
					input: { questions: [] },
				},
			],
		},
	];

	assert.equal(
		getPendingQuestionApprovalId(messages, {
			"approval-new": { approved: true },
		}),
		null,
	);
});

test("buildUserMessageParts includes attachments without dropping text", () => {
	const parts = buildUserMessageParts("hello", [
		{
			filename: "preview.png",
			mediaType: "image/png",
			url: "data:image/png;base64,abc123",
		},
	]);

	assert.deepEqual(parts, [
		{ type: "text", text: "hello" },
		{
			type: "file",
			filename: "preview.png",
			mediaType: "image/png",
			url: "data:image/png;base64,abc123",
		},
	]);
});

test("hasUserMessageContent treats attachment-only messages as non-empty", () => {
	assert.equal(
		hasUserMessageContent([
			{
				type: "file",
				filename: "preview.png",
				mediaType: "image/png",
				url: "data:image/png;base64,abc123",
			},
		]),
		true,
	);
	assert.equal(hasUserMessageContent([{ type: "text", text: "   " }]), false);
});

test("createUserMessageAttachment encodes files as data URLs", async () => {
	const file = new File([Uint8Array.from([104, 105])], "greeting.txt", {
		type: "text/plain",
	});

	const attachment = await createUserMessageAttachment(file);

	assert.deepEqual(attachment, {
		filename: "greeting.txt",
		mediaType: "text/plain",
		url: "data:text/plain;base64,aGk=",
	});
});

test("getPreviousTodoWriteEntries returns the prior TodoWrite update", () => {
	const messages: ChatMessage[] = [
		{
			id: "assistant-1",
			role: "assistant",
			parts: [
				{
					type: "dynamic-tool",
					toolCallId: "todo-1",
					toolName: "TodoWrite",
					state: "output-available",
					input: {
						todos: [
							{
								content: "Finish audit",
								activeForm: "Finishing audit",
								status: "completed",
							},
						],
					},
					output: {},
				},
			],
		},
		{
			id: "assistant-2",
			role: "assistant",
			parts: [
				{
					type: "dynamic-tool",
					toolCallId: "todo-2",
					toolName: "TodoWrite",
					state: "output-available",
					input: {
						todos: [
							{
								content: "Ship UI polish",
								activeForm: "Shipping UI polish",
								status: "in_progress",
							},
						],
					},
					output: {},
				},
			],
		},
	];

	assert.deepEqual(getPreviousTodoWriteEntries(messages, "todo-2"), [
		{
			content: "Finish audit",
			activeForm: "Finishing audit",
			status: "completed",
		},
	]);
});
