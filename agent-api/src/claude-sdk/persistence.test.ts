import assert from "node:assert";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, sep } from "node:path";
import { after, before, describe, it } from "node:test";
import {
	getLastMessageError,
	getSessionDirectoryForCwd,
} from "./persistence.js";

describe("persistence", () => {
	describe("getSessionDirectoryForCwd", () => {
		it("encodes cwd path correctly", () => {
			const cwd = "/home/user/workspace";
			const result = getSessionDirectoryForCwd(cwd);

			// Should remove leading slash and replace remaining slashes with dashes
			assert.ok(result.includes("home-user-workspace"));
			// path.join produces OS-native separators
			assert.ok(result.includes(`.claude${sep}projects`));
		});

		it("handles root directory", () => {
			const cwd = "/";
			const result = getSessionDirectoryForCwd(cwd);

			// Root should become empty string after removing leading slash
			assert.ok(result.includes(`.claude${sep}projects`));
		});

		it("handles nested paths", () => {
			const cwd = "/var/www/html/project";
			const result = getSessionDirectoryForCwd(cwd);

			assert.ok(result.includes("var-www-html-project"));
		});

		it("handles paths with multiple levels", () => {
			const cwd = "/a/b/c/d/e";
			const result = getSessionDirectoryForCwd(cwd);

			assert.ok(result.includes("a-b-c-d-e"));
			assert.ok(!result.includes("//"));
			assert.ok(!result.startsWith("-"));
		});

		it("produces consistent results", () => {
			const cwd = "/home/user/workspace";
			const result1 = getSessionDirectoryForCwd(cwd);
			const result2 = getSessionDirectoryForCwd(cwd);

			assert.strictEqual(result1, result2);
		});
	});

	describe("getLastMessageError", () => {
		// Create a test directory for JSONL files
		const testDir = join(tmpdir(), `claude-test-${Date.now()}`);
		const testCwd = join(testDir, "workspace");
		// Use getSessionDirectoryForCwd to get the correct encoded path
		const sessionDir = getSessionDirectoryForCwd(testCwd);

		// Helper to write test session file
		const writeTestSession = async (sessionId: string, messages: unknown[]) => {
			const content = messages.map((msg) => JSON.stringify(msg)).join("\n");
			await writeFile(join(sessionDir, `${sessionId}.jsonl`), content);
		};

		// Setup: Create test directory
		before(async () => {
			await mkdir(sessionDir, { recursive: true });
		});

		// Cleanup after all tests
		after(async () => {
			await rm(testDir, { recursive: true, force: true });
		});

		it("detects error with isApiErrorMessage flag and returns user-friendly text", async () => {
			const sessionId = "test-api-error";
			await writeTestSession(sessionId, [
				{
					type: "user",
					message: { role: "user", content: "test" },
				},
				{
					type: "assistant",
					error: "authentication_failed",
					isApiErrorMessage: true,
					message: {
						content: [
							{
								type: "text",
								text: "Invalid API key · Fix external API key",
							},
						],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(error, "Invalid API key · Fix external API key");
		});

		it("detects error field without isApiErrorMessage and returns content text", async () => {
			const sessionId = "test-error-field";
			await writeTestSession(sessionId, [
				{
					type: "assistant",
					error: "some_error_code",
					message: {
						content: [
							{
								type: "text",
								text: "Something went wrong with your request",
							},
						],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(error, "Something went wrong with your request");
		});

		it("does not detect error patterns in text content (only checks error fields)", async () => {
			const sessionId = "test-error-pattern";
			await writeTestSession(sessionId, [
				{
					type: "assistant",
					message: {
						content: [
							{
								type: "text",
								text: "Error: Connection timeout occurred",
							},
						],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(
				error,
				null,
				"Should not detect errors from text content",
			);
		});

		it("does not detect 'invalid api key' pattern in text (only checks error fields)", async () => {
			const sessionId = "test-invalid-key";
			await writeTestSession(sessionId, [
				{
					type: "assistant",
					message: {
						content: [
							{
								type: "text",
								text: "Invalid API Key provided",
							},
						],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(
				error,
				null,
				"Should not detect errors from text content",
			);
		});

		it("does not detect 'failed:' pattern in text (only checks error fields)", async () => {
			const sessionId = "test-failed";
			await writeTestSession(sessionId, [
				{
					type: "assistant",
					message: {
						content: [
							{
								type: "text",
								text: "Operation failed: Unable to process request",
							},
						],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(
				error,
				null,
				"Should not detect errors from text content",
			);
		});

		it("returns null when no error is present", async () => {
			const sessionId = "test-no-error";
			await writeTestSession(sessionId, [
				{
					type: "user",
					message: { role: "user", content: "hello" },
				},
				{
					type: "assistant",
					message: {
						content: [
							{
								type: "text",
								text: "Hello! How can I help you?",
							},
						],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(error, null);
		});

		it("returns null when session file does not exist", async () => {
			const error = await getLastMessageError("non-existent-session", testCwd);
			assert.strictEqual(error, null);
		});

		it("returns null when session file is empty", async () => {
			const sessionId = "test-empty";
			await writeFile(join(sessionDir, `${sessionId}.jsonl`), "");

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(error, null);
		});

		it("handles multiple content blocks and returns first text block", async () => {
			const sessionId = "test-multiple-blocks";
			await writeTestSession(sessionId, [
				{
					type: "assistant",
					error: "test_error",
					isApiErrorMessage: true,
					message: {
						content: [
							{
								type: "text",
								text: "First error message",
							},
							{
								type: "text",
								text: "Second message",
							},
						],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(error, "First error message");
		});

		it("falls back to error code if no content text available", async () => {
			const sessionId = "test-no-content";
			await writeTestSession(sessionId, [
				{
					type: "assistant",
					error: "error_code_123",
					isApiErrorMessage: true,
					message: {
						content: [],
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(error, "error_code_123");
		});

		it("ignores non-assistant messages", async () => {
			const sessionId = "test-user-message";
			await writeTestSession(sessionId, [
				{
					type: "user",
					error: "some_error",
					message: {
						role: "user",
						content: "Error: this is user content",
					},
				},
			]);

			const error = await getLastMessageError(sessionId, testCwd);
			assert.strictEqual(error, null);
		});
	});

	describe("loadSessionMessages - deduplication", () => {
		// Create a test directory for JSONL files
		const testDir = join(tmpdir(), `claude-dedup-test-${Date.now()}`);
		const testCwd = join(testDir, "workspace");
		const sessionDir = getSessionDirectoryForCwd(testCwd);

		// Helper to write test session file
		const writeTestSession = async (sessionId: string, records: unknown[]) => {
			const content = records
				.map((record) => JSON.stringify(record))
				.join("\n");
			await writeFile(join(sessionDir, `${sessionId}.jsonl`), content);
		};

		// Setup: Create test directory
		before(async () => {
			await mkdir(sessionDir, { recursive: true });
		});

		// Cleanup after all tests
		after(async () => {
			await rm(testDir, { recursive: true, force: true });
		});

		it("deduplicates assistant messages with same message.id but different uuid", async () => {
			const sessionId = "test-dedup-basic";

			// Simulate streaming: 3 partial records with same message.id
			await writeTestSession(sessionId, [
				{
					type: "user",
					uuid: "user-1",
					message: {
						role: "user",
						content: "Hello",
					},
				},
				// First partial update - only thinking block (partial)
				{
					type: "assistant",
					uuid: "assistant-uuid-1",
					message: {
						id: "msg_123",
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "Let me think about this...",
							},
						],
					},
				},
				// Second partial update - thinking + tool (more complete)
				{
					type: "assistant",
					uuid: "assistant-uuid-2",
					message: {
						id: "msg_123", // Same message.id!
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "Let me think about this...",
							},
							{
								type: "tool_use",
								id: "tool_1",
								name: "Read",
								input: { file_path: "test.txt" },
							},
						],
					},
				},
				// Third partial update - complete (thinking + tool + text)
				{
					type: "assistant",
					uuid: "assistant-uuid-3",
					message: {
						id: "msg_123", // Same message.id!
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "Let me think about this...",
							},
							{
								type: "tool_use",
								id: "tool_1",
								name: "Read",
								input: { file_path: "test.txt" },
							},
							{
								type: "text",
								text: "Here is the content",
							},
						],
					},
				},
			]);

			const { loadSessionMessages } = await import("./persistence.js");
			const messages = await loadSessionMessages(sessionId, testCwd);

			// Should have 2 messages: 1 user + 1 assistant
			assert.strictEqual(messages.length, 2, "Should have 2 messages");

			const assistantMsg = messages[1];
			assert.strictEqual(assistantMsg.role, "assistant");

			// Should have 3 parts from the LAST (most complete) record
			// No step-start because all 3 partial records had the same message.id
			assert.strictEqual(
				assistantMsg.parts.length,
				3,
				"Should have 3 parts (thinking, tool, text)",
			);
			assert.strictEqual(assistantMsg.parts[0].type, "reasoning");
			assert.strictEqual(assistantMsg.parts[1].type, "dynamic-tool");
			assert.strictEqual(assistantMsg.parts[2].type, "text");
		});

		it("preserves order when deduplicating multiple API calls in agentic loop", async () => {
			const sessionId = "test-dedup-agentic";

			// Simulate agentic loop with partial updates
			await writeTestSession(sessionId, [
				{
					type: "user",
					uuid: "user-1",
					message: {
						role: "user",
						content: "Do a task",
					},
				},
				// First API call - partial
				{
					type: "assistant",
					uuid: "assistant-1-partial",
					message: {
						id: "msg_call1",
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "First thinking",
							},
						],
					},
				},
				// First API call - complete
				{
					type: "assistant",
					uuid: "assistant-1-complete",
					message: {
						id: "msg_call1", // Same as above
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "First thinking",
							},
							{
								type: "tool_use",
								id: "tool_1",
								name: "Read",
								input: {},
							},
						],
					},
				},
				// Tool result
				{
					type: "user",
					uuid: "tool-result-1",
					message: {
						role: "user",
						content: [
							{
								type: "tool_result",
								tool_use_id: "tool_1",
								content: "File content",
							},
						],
					},
				},
				// Second API call - partial
				{
					type: "assistant",
					uuid: "assistant-2-partial",
					message: {
						id: "msg_call2",
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "Second thinking",
							},
						],
					},
				},
				// Second API call - complete
				{
					type: "assistant",
					uuid: "assistant-2-complete",
					message: {
						id: "msg_call2", // Different message.id
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "Second thinking",
							},
							{
								type: "text",
								text: "Final response",
							},
						],
					},
				},
			]);

			const { loadSessionMessages } = await import("./persistence.js");
			const messages = await loadSessionMessages(sessionId, testCwd);

			// Should have 2 messages: 1 user + 1 merged assistant
			assert.strictEqual(messages.length, 2);

			const assistantMsg = messages[1];
			// Should have: thinking1, tool1, step-start, thinking2, text
			assert.strictEqual(assistantMsg.parts.length, 5);
			assert.strictEqual(assistantMsg.parts[0].type, "reasoning");
			assert.strictEqual(assistantMsg.parts[1].type, "dynamic-tool");
			assert.strictEqual(assistantMsg.parts[2].type, "step-start");
			assert.strictEqual(assistantMsg.parts[3].type, "reasoning");
			assert.strictEqual(assistantMsg.parts[4].type, "text");
		});

		it("handles interleaved thinking and tools correctly after deduplication", async () => {
			const sessionId = "test-dedup-interleaved";

			await writeTestSession(sessionId, [
				{
					type: "user",
					uuid: "user-1",
					message: {
						role: "user",
						content: "Test",
					},
				},
				// Partial record - only first thinking
				{
					type: "assistant",
					uuid: "partial-1",
					message: {
						id: "msg_abc",
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "First think",
							},
						],
					},
				},
				// Partial record - first thinking + tool
				{
					type: "assistant",
					uuid: "partial-2",
					message: {
						id: "msg_abc",
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "First think",
							},
							{
								type: "tool_use",
								id: "tool_a",
								name: "Read",
								input: {},
							},
						],
					},
				},
				// Complete record - all content
				{
					type: "assistant",
					uuid: "complete",
					message: {
						id: "msg_abc",
						role: "assistant",
						content: [
							{
								type: "thinking",
								thinking: "First think",
							},
							{
								type: "tool_use",
								id: "tool_a",
								name: "Read",
								input: {},
							},
							{
								type: "thinking",
								thinking: "Second think",
							},
							{
								type: "tool_use",
								id: "tool_b",
								name: "Write",
								input: {},
							},
							{
								type: "text",
								text: "Done",
							},
						],
					},
				},
			]);

			const { loadSessionMessages } = await import("./persistence.js");
			const messages = await loadSessionMessages(sessionId, testCwd);

			const assistantMsg = messages[1];
			// Should preserve exact order: think1, tool1, think2, tool2, text
			assert.strictEqual(assistantMsg.parts.length, 5);
			assert.strictEqual(assistantMsg.parts[0].type, "reasoning");
			assert.strictEqual(assistantMsg.parts[1].type, "dynamic-tool");
			assert.strictEqual(assistantMsg.parts[2].type, "reasoning");
			assert.strictEqual(assistantMsg.parts[3].type, "dynamic-tool");
			assert.strictEqual(assistantMsg.parts[4].type, "text");
		});
	});

	// Note: Integration tests for discoverSessions, loadSessionMessages, etc.
	// should be written as separate integration tests that use actual test
	// session files, rather than complex mocking of fs/promises.
	// See test/integration/ for these tests.
});
