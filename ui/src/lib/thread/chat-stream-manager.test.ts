import assert from "node:assert/strict";
import test from "node:test";

import { createChatStreamManager } from "$lib/thread/chat-stream-manager";

class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;
	static instances: MockWebSocket[] = [];

	readyState = MockWebSocket.CONNECTING;
	onopen: (() => void) | null = null;
	onmessage: ((event: MessageEvent<string>) => void) | null = null;
	onerror: (() => void) | null = null;
	onclose: (() => void) | null = null;
	sent: string[] = [];
	url: string;

	constructor(url: string) {
		this.url = url;
		MockWebSocket.instances.push(this);
	}

	send(payload: string) {
		this.sent.push(payload);
	}

	close() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	}

	emitOpen() {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.();
	}

	emitMessage(payload: unknown) {
		this.onmessage?.(
			new MessageEvent("message", { data: JSON.stringify(payload) }),
		);
	}

	emitClose() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	}
}

function flush() {
	return new Promise((resolve) => setTimeout(resolve, 0));
}

test.beforeEach(() => {
	MockWebSocket.instances = [];
	Object.defineProperty(globalThis, "WebSocket", {
		configurable: true,
		writable: true,
		value: MockWebSocket,
	});
});

test("chat stream manager routes websocket events and resubscribes after completion", async () => {
	const manager = createChatStreamManager();
	let opened = 0;
	const chunkEvents: string[] = [];
	const subscription = manager.subscribe({
		sessionId: "session-1",
		threadId: "thread-1",
		replay: true,
		onOpen: () => {
			opened += 1;
		},
	});

	subscription.eventSource.addEventListener("chunk", (event) => {
		chunkEvents.push(event.data);
	});

	assert.equal(MockWebSocket.instances.length, 1);
	const socket = MockWebSocket.instances[0];
	socket.emitOpen();

	assert.deepEqual(JSON.parse(socket.sent[0]), {
		type: "subscribe",
		stream: "chat",
		sessionId: "session-1",
		threadId: "thread-1",
		replay: true,
	});

	socket.emitMessage({
		type: "subscribed",
		stream: "chat",
		sessionId: "session-1",
		threadId: "thread-1",
	});
	socket.emitMessage({
		type: "event",
		stream: "chat",
		sessionId: "session-1",
		threadId: "thread-1",
		event: "chunk",
		data: '{"type":"text","text":"hello"}',
		id: "completion-1:0",
	});
	socket.emitMessage({
		type: "complete",
		stream: "chat",
		sessionId: "session-1",
		threadId: "thread-1",
	});
	await flush();

	assert.equal(opened, 1);
	assert.deepEqual(chunkEvents, ['{"type":"text","text":"hello"}']);
	assert.equal(subscription.getState(), "idle");

	subscription.resubscribe();
	assert.deepEqual(JSON.parse(socket.sent[1]), {
		type: "subscribe",
		stream: "chat",
		sessionId: "session-1",
		threadId: "thread-1",
		replay: true,
		lastEventId: "completion-1:0",
	});

	subscription.unsubscribe();
	assert.deepEqual(JSON.parse(socket.sent[2]), {
		type: "unsubscribe",
		stream: "chat",
		sessionId: "session-1",
		threadId: "thread-1",
	});
	manager.dispose();
});

test("chat stream manager reconnects and resubscribes active streams", async () => {
	const manager = createChatStreamManager();
	const subscription = manager.subscribe({
		sessionId: "session-2",
		threadId: "thread-2",
		replay: true,
	});

	const firstSocket = MockWebSocket.instances[0];
	firstSocket.emitOpen();
	assert.deepEqual(JSON.parse(firstSocket.sent[0]), {
		type: "subscribe",
		stream: "chat",
		sessionId: "session-2",
		threadId: "thread-2",
		replay: true,
	});

	firstSocket.emitClose();
	await new Promise((resolve) => setTimeout(resolve, 1100));

	assert.equal(MockWebSocket.instances.length, 2);
	const secondSocket = MockWebSocket.instances[1];
	secondSocket.emitOpen();
	assert.deepEqual(JSON.parse(secondSocket.sent[0]), {
		type: "subscribe",
		stream: "chat",
		sessionId: "session-2",
		threadId: "thread-2",
		replay: true,
	});

	subscription.unsubscribe();
	manager.dispose();
});

test("chat stream manager routes service output over the shared websocket", () => {
	const manager = createChatStreamManager();
	const messages: string[] = [];
	const subscription = manager.subscribeServiceOutput({
		sessionId: "session-3",
		serviceId: "service-1",
	});

	subscription.eventSource.addEventListener("message", (event) => {
		messages.push(event.data);
	});

	assert.equal(MockWebSocket.instances.length, 1);
	const socket = MockWebSocket.instances[0];
	socket.emitOpen();

	assert.deepEqual(JSON.parse(socket.sent[0]), {
		type: "subscribe",
		stream: "service",
		sessionId: "session-3",
		serviceId: "service-1",
	});

	socket.emitMessage({
		type: "subscribed",
		stream: "service",
		sessionId: "session-3",
		serviceId: "service-1",
	});
	socket.emitMessage({
		type: "event",
		stream: "service",
		sessionId: "session-3",
		serviceId: "service-1",
		data: '{"type":"stdout","data":"hello","timestamp":"2026-01-01T00:00:00Z"}',
	});

	assert.deepEqual(messages, [
		'{"type":"stdout","data":"hello","timestamp":"2026-01-01T00:00:00Z"}',
	]);

	subscription.unsubscribe();
	assert.deepEqual(JSON.parse(socket.sent[1]), {
		type: "unsubscribe",
		stream: "service",
		sessionId: "session-3",
		serviceId: "service-1",
	});
	manager.dispose();
});

test("chat stream manager routes project events over the shared websocket", () => {
	const manager = createChatStreamManager();
	const connectedEvents: string[] = [];
	const sessionEvents: string[] = [];
	const subscription = manager.subscribeProjectEvents({ afterId: "event-1" });

	subscription.eventSource.addEventListener("connected", (event) => {
		connectedEvents.push(event.data);
	});
	subscription.eventSource.addEventListener("session_updated", (event) => {
		sessionEvents.push(event.data);
	});

	assert.equal(MockWebSocket.instances.length, 1);
	const socket = MockWebSocket.instances[0];
	socket.emitOpen();

	assert.deepEqual(JSON.parse(socket.sent[0]), {
		type: "subscribe",
		stream: "project-events",
		afterId: "event-1",
	});

	socket.emitMessage({
		type: "subscribed",
		stream: "project-events",
	});
	socket.emitMessage({
		type: "event",
		stream: "project-events",
		event: "connected",
		data: '{"projectId":"local"}',
	});
	socket.emitMessage({
		type: "event",
		stream: "project-events",
		event: "session_updated",
		data: '{"id":"event-2","type":"session_updated"}',
		id: "event-2",
	});

	assert.deepEqual(connectedEvents, ['{"projectId":"local"}']);
	assert.deepEqual(sessionEvents, [
		'{"id":"event-2","type":"session_updated"}',
	]);

	subscription.unsubscribe();
	assert.deepEqual(JSON.parse(socket.sent[1]), {
		type: "unsubscribe",
		stream: "project-events",
	});
	manager.dispose();
});
