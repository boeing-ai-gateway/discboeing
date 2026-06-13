import type { ChatStreamSocketEventName, Thread } from "$lib/api-types";
import { expect, test } from "vitest";

import { createCommands } from "$lib/context/commands";
import type { Context } from "$lib/context/context.types";
import {
	activateSession,
	deactivateSession,
	ensureSessionRecord,
} from "$lib/context/domains/sessions";
import {
	activateThread,
	applyThreadsSnapshotToRecord,
	deactivateThread,
	ensureThreadContentState,
} from "$lib/context/domains/threads";
import { ensureThreadView } from "$lib/context/domains/view";
import { connectSessionEvents } from "$lib/context/session-subscription";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";
import type {
	ProjectEventSocket,
	ProjectSocketMessage,
	ProjectSocketRequest,
	ProjectSocketSubscription,
	ProjectSocketSubscriptionOptions,
} from "$lib/context/project-subscription";
import { connectThreadEvents } from "$lib/context/thread-subscription";

class FakeProjectSocket implements ProjectEventSocket {
	private options: ProjectSocketSubscriptionOptions | null = null;
	request: ProjectSocketRequest | null = null;
	closed = false;
	subscribeCount = 0;

	open(): Promise<void> {
		return Promise.resolve();
	}

	close(): void {
		this.closed = true;
	}

	subscribe(
		request: ProjectSocketRequest,
		options: ProjectSocketSubscriptionOptions,
	): ProjectSocketSubscription {
		this.subscribeCount += 1;
		this.request = request;
		this.options = options;
		return {
			open: () => Promise.resolve(),
			close: () => {
				this.closed = true;
			},
		};
	}

	emit(message: ProjectSocketMessage): void {
		this.options?.onMessage(message);
	}
}

test("thread subscription open is idempotent while activation is pending", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";
	const subscription = connectThreadEvents(
		context,
		sessionId,
		threadId,
		socket,
	);

	const firstOpen = subscription.open();
	const secondOpen = subscription.open();

	expect(socket.subscribeCount).toBe(1);
	expect(socket.request).toEqual({
		type: "subscribe",
		stream: "chat",
		sessionId,
		threadId,
	});

	emitChatEvent(socket, sessionId, threadId, "history-start", "");
	emitChatEvent(socket, sessionId, threadId, "history-end", "");
	await Promise.all([firstOpen, secondOpen]);

	await subscription.open();
	expect(socket.subscribeCount).toBe(1);
});

test("thread activation is idempotent for concurrent calls", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	const firstActivation = activateThread(context, sessionId, threadId, socket);
	const secondActivation = activateThread(context, sessionId, threadId, socket);

	await waitFor(() => socket.subscribeCount === 1);
	expect(socket.subscribeCount).toBe(1);

	emitChatEvent(socket, sessionId, threadId, "history-start", "");
	emitChatEvent(socket, sessionId, threadId, "history-end", "");
	await Promise.all([firstActivation, secondActivation]);

	await activateReadyThread(context, socket, sessionId, threadId);
	expect(socket.subscribeCount).toBe(1);
});

test("thread activation can reactivate immediately after pending deactivate", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	const firstActivation = activateThread(
		context,
		sessionId,
		threadId,
		socket,
	).catch(() => undefined);
	await waitFor(() => socket.subscribeCount === 1);

	await deactivateThread(context, sessionId, threadId);

	const secondActivation = activateThread(context, sessionId, threadId, socket);
	await waitFor(() => socket.subscribeCount === 2);
	expect(socket.subscribeCount).toBe(2);

	emitChatEvent(socket, sessionId, threadId, "history-start", "");
	emitChatEvent(socket, sessionId, threadId, "history-end", "");
	await secondActivation;
	await firstActivation;
});

test("session activation is idempotent for concurrent calls", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";

	const firstActivation = activateSession(context, sessionId, socket, {
		wait: true,
	});
	const secondActivation = activateSession(context, sessionId, socket, {
		wait: true,
	});

	await waitFor(() => socket.subscribeCount === 1);
	expect(socket.subscribeCount).toBe(1);
	expect(socket.request).toEqual({
		type: "subscribe",
		stream: "session",
		sessionId,
	});

	socket.emit({
		type: "event",
		stream: "session",
		sessionId,
		event: "history-start",
	});
	socket.emit({
		type: "event",
		stream: "session",
		sessionId,
		event: "history-end",
	});
	await Promise.all([firstActivation, secondActivation]);

	await activateSession(context, sessionId, socket, { wait: true });
	expect(socket.subscribeCount).toBe(1);
});

test("session activation can reactivate immediately after pending deactivate", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";

	const firstActivation = activateSession(context, sessionId, socket, {
		wait: true,
	}).catch(() => undefined);
	await waitFor(() => socket.subscribeCount === 1);

	await deactivateSession(context, sessionId);

	const secondActivation = activateSession(context, sessionId, socket, {
		wait: true,
	});
	await waitFor(() => socket.subscribeCount === 2);
	expect(socket.subscribeCount).toBe(2);

	emitSessionHistoryEvent(socket, sessionId, "history-start");
	emitSessionHistoryEvent(socket, sessionId, "history-end");
	await secondActivation;
	await firstActivation;
});

test("session deactivation stops active thread subscriptions", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	await activateReadyThread(context, socket, sessionId, threadId);
	const content = ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	);
	expect(content.subscription).not.toBeNull();

	await deactivateSession(context, sessionId);

	expect(socket.closed).toBe(true);
	expect(content.subscription).toBeNull();
});

test("thread snapshots preserve active subscription state", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	await activateReadyThread(context, socket, sessionId, threadId);
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	const subscription = record.threads.byId[threadId]?.content.subscription;

	applyThreadsSnapshotToRecord(record, [createThread(threadId)]);

	expect(record.threads.byId[threadId]?.content.subscription).toBe(
		subscription,
	);
	await activateThread(context, sessionId, threadId, socket);
	expect(socket.subscribeCount).toBe(1);
});

test("session history preserves active thread subscription state", async () => {
	const context = createPlainContext();
	const threadSocket = new FakeProjectSocket();
	const sessionSocket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	await activateReadyThread(context, threadSocket, sessionId, threadId);
	const record = ensureSessionRecord(context.data.sessions, sessionId);
	const subscription = record.threads.byId[threadId]?.content.subscription;

	const sessionSubscription = connectSessionEvents(
		context,
		sessionId,
		sessionSocket,
	);
	const opened = sessionSubscription.open();
	emitSessionHistoryEvent(sessionSocket, sessionId, "history-start");
	sessionSocket.emit({
		type: "event",
		stream: "session",
		sessionId,
		event: "threads_updated",
		data: { threads: [createThread(threadId)] },
	});
	emitSessionHistoryEvent(sessionSocket, sessionId, "history-end");
	await opened;

	const updatedRecord = ensureSessionRecord(context.data.sessions, sessionId);
	expect(updatedRecord.threads.byId[threadId]?.content.subscription).toBe(
		subscription,
	);
	await activateThread(context, sessionId, threadId, threadSocket);
	expect(threadSocket.subscribeCount).toBe(1);
});

test("thread activation builds messages from shared websocket chat chunks", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	);

	const subscription = connectThreadEvents(
		context,
		sessionId,
		threadId,
		socket,
	);
	const opened = subscription.open();

	expect(socket.request).toEqual({
		type: "subscribe",
		stream: "chat",
		sessionId,
		threadId,
	});

	emitChatEvent(socket, sessionId, threadId, "history-start", "");
	emitChatEvent(socket, sessionId, threadId, "history-end", "");
	await opened;

	emitChatEvent(
		socket,
		sessionId,
		threadId,
		"chunk",
		JSON.stringify({ type: "start", messageId: "assistant-1" }),
	);
	emitChatEvent(
		socket,
		sessionId,
		threadId,
		"chunk",
		JSON.stringify({ type: "text-start", id: "text-1" }),
	);
	emitChatEvent(
		socket,
		sessionId,
		threadId,
		"chunk",
		JSON.stringify({ type: "text-delta", id: "text-1", delta: "hello" }),
	);
	emitChatEvent(
		socket,
		sessionId,
		threadId,
		"chunk",
		JSON.stringify({ type: "text-end", id: "text-1" }),
	);
	emitChatEvent(
		socket,
		sessionId,
		threadId,
		"chunk",
		JSON.stringify({ type: "finish" }),
	);

	await waitFor(() => {
		const message =
			context.data.sessions.byId[sessionId]?.threads.byId[threadId]?.content
				?.messages[0];
		return (
			message?.parts[0]?.type === "text" && message.parts[0].text === "hello"
		);
	});

	const content =
		context.data.sessions.byId[sessionId]?.threads.byId[threadId]?.content;
	expect(content?.status.state).toBe("ready");
	expect(content?.messages).toHaveLength(1);
	expect(content?.messages[0]?.id).toBe("assistant-1");
	expect(content?.isStreaming).toBe(false);
});

test("thread history initializes stick-to-bottom from preference", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	context.view.app.preferences.autoScrollOnStream = false;
	await activateReadyThread(context, socket, sessionId, threadId);

	expect(
		context.view.sessions[sessionId]?.threads[threadId]?.conversation
			.stickToBottom,
	).toBe(false);
});

test("thread history preserves existing stick-to-bottom state", async () => {
	const context = createPlainContext();
	const socket = new FakeProjectSocket();
	const sessionId = "session-1";
	const threadId = "thread-1";

	ensureThreadView(context, sessionId, threadId).conversation.stickToBottom =
		false;
	await activateReadyThread(context, socket, sessionId, threadId);

	expect(
		context.view.sessions[sessionId]?.threads[threadId]?.conversation
			.stickToBottom,
	).toBe(false);
});

function emitChatEvent(
	socket: FakeProjectSocket,
	sessionId: string,
	threadId: string,
	event: ChatStreamSocketEventName,
	data: string,
): void {
	socket.emit({
		type: "event",
		stream: "chat",
		sessionId,
		threadId,
		event,
		data,
	});
}

function emitSessionHistoryEvent(
	socket: FakeProjectSocket,
	sessionId: string,
	event: "history-start" | "history-end",
): void {
	socket.emit({
		type: "event",
		stream: "session",
		sessionId,
		event,
	});
}

function createThread(threadId: string): Thread {
	return {
		id: threadId,
		name: "Thread",
	} as Thread;
}

async function activateReadyThread(
	context: Context,
	socket: FakeProjectSocket,
	sessionId: string,
	threadId: string,
): Promise<void> {
	const activation = activateThread(context, sessionId, threadId, socket);
	await waitFor(() => socket.subscribeCount > 0);
	emitChatEvent(socket, sessionId, threadId, "history-start", "");
	emitChatEvent(socket, sessionId, threadId, "history-end", "");
	await activation;
}

async function waitFor(condition: () => boolean): Promise<void> {
	for (let attempt = 0; attempt < 20; attempt += 1) {
		if (condition()) return;
		await new Promise((resolve) => setTimeout(resolve, 0));
	}
	throw new Error("condition was not met");
}

function createPlainContext(): Context {
	const context: Context = {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState(),
		commands: undefined as unknown as Context["commands"],
	};
	context.commands = createCommands(context);
	return context;
}
