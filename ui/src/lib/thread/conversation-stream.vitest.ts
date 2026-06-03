import { describe, expect, test } from "vitest";

import type { ChatMessage } from "$lib/api-types";
import { createChatStreamState } from "$lib/thread/conversation-stream";

describe("createChatStreamState", () => {
	test("accepts workspace file chunks before assistant start", async () => {
		let messages: ChatMessage[] = [];
		const startEvents: Array<{ resume?: boolean } | undefined> = [];
		const state = createChatStreamState({
			getMessages: () => messages,
			setMessages: (nextMessages) => {
				messages = nextMessages;
			},
			onStart: (info) => {
				startEvents.push(info);
			},
		});

		await state.handleStreamEvent({
			event: "chunk",
			data: JSON.stringify({
				type: "data-workspace-files",
				data: {
					root: "/workspaces/project",
					changes: [
						{
							kind: "created",
							path: "src/app.ts",
							entry: {
								path: "src/app.ts",
								isDir: false,
								size: 12,
								mode: 420,
								modTime: "2026-06-02T03:27:49Z",
							},
						},
					],
				},
			}),
		});
		await state.handleStreamEvent({
			event: "chunk",
			data: JSON.stringify({
				type: "data-workspace-ports",
				data: {
					reason: "initial",
					ports: [
						{
							localAddress: "127.0.0.1:3000",
							port: 3000,
							process: "node",
							pid: 123,
							fd: 9,
						},
					],
					resync: true,
					scannedAt: "2026-06-02T03:27:49Z",
				},
			}),
		});
		await state.handleStreamEvent({
			event: "chunk",
			data: JSON.stringify({ type: "start", messageId: "assistant-1" }),
		});

		expect(messages).toHaveLength(1);
		expect(messages[0]?.id).toBe("assistant-1");
		expect(startEvents).toEqual([{ resume: false }]);
	});
});
