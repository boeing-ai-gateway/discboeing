// @vitest-environment node
import { afterEach, beforeEach, expect, test } from "vitest";

import { api } from "$lib/api-client";
import { createCommands } from "$lib/context/commands";
import type { Context } from "$lib/context/context.types";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

type TrackingWebSocket = {
	__discboeingUrl: string;
	readyState: number;
	close(code?: number, reason?: string): void;
};
type NativeWebSocketConstructor = typeof WebSocket;

class TrackingWebSocketWrapper {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	__discboeingUrl: string;
	onopen: ((event: Event) => void) | null = null;
	onmessage: ((event: MessageEvent) => void) | null = null;
	onerror: ((event: Event) => void) | null = null;
	onclose: ((event: CloseEvent) => void) | null = null;
	private inner: WebSocket;

	constructor(
		NativeWebSocket: NativeWebSocketConstructor,
		url: string | URL,
		protocols?: string | string[],
	) {
		this.__discboeingUrl = String(url);
		this.inner = new NativeWebSocket(url, protocols);
		this.inner.onopen = (event) => this.onopen?.(event);
		this.inner.onmessage = (event) => {
			try {
				receivedSocketMessages.push(JSON.parse(String(event.data)));
			} catch {
				receivedSocketMessages.push(event.data);
			}
			this.onmessage?.(event);
		};
		this.inner.onerror = (event) => this.onerror?.(event);
		this.inner.onclose = (event) => this.onclose?.(event);
	}

	get readyState(): number {
		return this.inner.readyState;
	}

	send(data: Parameters<WebSocket["send"]>[0]): void {
		this.inner.send(data);
	}

	close(code?: number, reason?: string): void {
		this.inner.close(code, reason);
	}
}

const LIVE_E2E_ENABLED = process.env.DISCBOEING_LIVE_E2E === "1";
const LIVE_E2E_API_ROOT =
	process.env.DISCBOEING_LIVE_E2E_API_ROOT ?? "http://localhost:3001/api";
const LIVE_E2E_TIMEOUT_MS = 120_000;
const LIVE_E2E_DELTA_TIMEOUT_MS = 30_000;
const WAIT_INTERVAL_MS = 100;

let originalWindow: typeof globalThis.window | undefined;
let originalWebSocket: typeof globalThis.WebSocket | undefined;
let sockets: TrackingWebSocket[] = [];
let receivedSocketMessages: unknown[] = [];

beforeEach(() => {
	originalWindow = globalThis.window;
	originalWebSocket = globalThis.WebSocket;
	sockets = [];
	receivedSocketMessages = [];

	Object.defineProperty(globalThis, "window", {
		configurable: true,
		writable: true,
		value: {
			...globalThis,
			__DISCBOEING_CONFIG__: {
				apiRoot: LIVE_E2E_API_ROOT,
			},
			location: {
				origin: "http://localhost:3100",
				hostname: "localhost",
				protocol: "http:",
				port: "3100",
			},
			setTimeout,
			clearTimeout,
		},
	});

	if (originalWebSocket) {
		const NativeWebSocket = originalWebSocket;
		function DiscboeingTrackingWebSocket(
			this: TrackingWebSocket,
			url: string | URL,
			protocols?: string | string[],
		) {
			const socket = new TrackingWebSocketWrapper(
				NativeWebSocket,
				url,
				protocols,
			) as unknown as TrackingWebSocket;
			sockets.push(socket);
			return socket;
		}
		DiscboeingTrackingWebSocket.CONNECTING = TrackingWebSocketWrapper.CONNECTING;
		DiscboeingTrackingWebSocket.OPEN = TrackingWebSocketWrapper.OPEN;
		DiscboeingTrackingWebSocket.CLOSING = TrackingWebSocketWrapper.CLOSING;
		DiscboeingTrackingWebSocket.CLOSED = TrackingWebSocketWrapper.CLOSED;
		Object.defineProperty(globalThis, "WebSocket", {
			configurable: true,
			writable: true,
			value: DiscboeingTrackingWebSocket,
		});
	}
});

afterEach(() => {
	for (const socket of sockets) {
		if (
			socket.readyState === WebSocket.OPEN ||
			socket.readyState === WebSocket.CONNECTING
		) {
			socket.close();
		}
	}
	Object.defineProperty(globalThis, "window", {
		configurable: true,
		writable: true,
		value: originalWindow,
	});
	Object.defineProperty(globalThis, "WebSocket", {
		configurable: true,
		writable: true,
		value: originalWebSocket,
	});
});

const liveTest = LIVE_E2E_ENABLED ? test : test.skip;

liveTest(
	"ng session stream builds caches, applies deltas, and recovers after websocket reconnect",
	async () => {
		if (!globalThis.WebSocket) {
			throw new Error("global WebSocket is not available in this runtime");
		}

		await expectBackendReady();

		const context = createPlainContext();
		const sessionId = `ng-e2e-${Date.now()}-${Math.random().toString(36).slice(2)}`;
		const initialThreadId = `${sessionId}-thread-a`;
		const recoveredThreadId = `${sessionId}-thread-b`;
		const filePath = `discboeing-ng-e2e-${Date.now()}.txt`;

		try {
			await withTimeout(
				context.commands.projects.activateProject("local", { wait: true }),
				LIVE_E2E_TIMEOUT_MS,
				"project activation",
			);

			await withTimeout(
				context.commands.sessions.createSession(
					{ id: sessionId },
					{ wait: true },
				),
				LIVE_E2E_TIMEOUT_MS,
				"session creation",
			);

			await withTimeout(
				context.commands.sessions.activateSession(sessionId, { wait: true }),
				LIVE_E2E_TIMEOUT_MS,
				"session activation",
			);

			const record = context.data.sessions.byId[sessionId];
			expect(record?.value?.id).toBe(sessionId);
			expect(record.status.state).toBe("ready");
			expect(record.threads.status.state).toBe("ready");
			expect(record.files.statusBySubtree[""]?.state).toBe("ready");
			expect(record.commands.status.state).toBe("ready");
			expect(record.hooks.status.state).toBe("ready");
			expect(record.services.status.state).toBe("ready");
			expect(record.diff.filesStatus.state).toBe("ready");
			expect(record.diff.status.state).toBe("ready");

			expect(sockets.length).toBe(1);
			expect(sockets[0].__discboeingUrl).toContain("/api/projects/local/ws");

			await api.createThread(sessionId, {
				id: initialThreadId,
				name: "initial live e2e thread",
			});
			const threadsAfterCreate = await api.getThreads(sessionId);
			expect(
				threadsAfterCreate.threads.some(
					(thread) => thread.id === initialThreadId,
				),
			).toBe(true);
			await waitFor(
				() =>
					context.data.sessions.byId[sessionId]?.threads.byId[initialThreadId]
						?.value?.name === "initial live e2e thread",
				"initial thread delta",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);

			await withTimeout(
				context.commands.threads.activateThread(sessionId, initialThreadId, {
					wait: true,
				}),
				LIVE_E2E_TIMEOUT_MS,
				"thread activation",
			);
			expect(sockets.length).toBe(1);
			expect(
				context.data.sessions.byId[sessionId]?.threads.byId[initialThreadId]
					?.content.status.state,
			).toBe("ready");
			expect(
				countChatEvents(sessionId, initialThreadId, "history-end"),
			).toBeGreaterThan(0);

			await api.updateThread(sessionId, initialThreadId, {
				name: "renamed live e2e thread",
			});
			await waitFor(
				() =>
					context.data.sessions.byId[sessionId]?.threads.byId[initialThreadId]
						?.value?.name === "renamed live e2e thread",
				"thread rename delta",
			);

			await api.writeSessionFile(sessionId, {
				path: filePath,
				content: "created by ng live e2e\n",
			});
			await waitFor(
				() =>
					!!context.data.sessions.byId[sessionId]?.files.nodesByPath[filePath],
				"file watcher delta",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);

			const firstSocketCount = sockets.length;
			const sessionHistoryEndCount = countSessionEvents(
				sessionId,
				"history-end",
			);
			sockets.at(-1)?.close();
			await waitFor(
				() => sockets.length > firstSocketCount,
				"project websocket reconnect",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);
			await waitFor(
				() =>
					countSessionEvents(sessionId, "history-end") > sessionHistoryEndCount,
				"session resubscribe after reconnect",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);

			await api.createThread(sessionId, {
				id: recoveredThreadId,
				name: "post reconnect live e2e thread",
			});
			await waitFor(
				() =>
					context.data.sessions.byId[sessionId]?.threads.byId[recoveredThreadId]
						?.value?.name === "post reconnect live e2e thread",
				"post-reconnect session delta",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);
		} finally {
			try {
				await api.deleteSession(sessionId);
			} catch {
				// Best-effort cleanup: the server may have restarted during the live test.
			}
			context.commands.lifecycle.shutdown();
		}
	},
	LIVE_E2E_TIMEOUT_MS + 30_000,
);

liveTest(
	"stopping an active session preserves session-scoped caches",
	async () => {
		if (!globalThis.WebSocket) {
			throw new Error("global WebSocket is not available in this runtime");
		}

		await expectBackendReady();

		const context = createPlainContext();
		const sessionId = `ng-e2e-stop-${Date.now()}-${Math.random().toString(36).slice(2)}`;
		const threadId = `${sessionId}-thread`;
		const filePath = `discboeing-ng-stop-e2e-${Date.now()}.txt`;

		try {
			await withTimeout(
				context.commands.projects.activateProject("local", { wait: true }),
				LIVE_E2E_TIMEOUT_MS,
				"project activation",
			);
			await withTimeout(
				context.commands.sessions.createSession(
					{ id: sessionId },
					{ wait: true },
				),
				LIVE_E2E_TIMEOUT_MS,
				"session creation",
			);
			await withTimeout(
				context.commands.sessions.activateSession(sessionId, { wait: true }),
				LIVE_E2E_TIMEOUT_MS,
				"session activation",
			);

			await api.createThread(sessionId, {
				id: threadId,
				name: "cached thread before stop",
			});
			await waitFor(
				() =>
					context.data.sessions.byId[sessionId]?.threads.byId[threadId]?.value
						?.name === "cached thread before stop",
				"thread cache before stop",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);

			await api.writeSessionFile(sessionId, {
				path: filePath,
				content: "cached file before stop\n",
			});
			await waitFor(
				() =>
					!!context.data.sessions.byId[sessionId]?.files.nodesByPath[filePath],
				"file cache before stop",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);

			await api.stopSession(sessionId);
			await waitFor(
				() =>
					context.data.sessions.byId[sessionId]?.value?.sandboxStatus ===
					"stopped",
				"stopped session status",
				LIVE_E2E_DELTA_TIMEOUT_MS,
			);

			const stoppedRecord = context.data.sessions.byId[sessionId];
			expect(stoppedRecord?.threads.byId[threadId]?.value?.name).toBe(
				"cached thread before stop",
			);
			expect(stoppedRecord?.files.nodesByPath[filePath]?.entry?.name).toBe(
				filePath,
			);

			void context.commands.sessions.activateSession(sessionId);
			expect(
				context.data.sessions.byId[sessionId]?.threads.byId[threadId]?.value
					?.name,
			).toBe("cached thread before stop");
			expect(
				context.data.sessions.byId[sessionId]?.files.nodesByPath[filePath]
					?.entry?.name,
			).toBe(filePath);
		} finally {
			try {
				await api.deleteSession(sessionId);
			} catch {
				// Best-effort cleanup: the server may have restarted during the live test.
			}
			context.commands.lifecycle.shutdown();
		}
	},
	LIVE_E2E_TIMEOUT_MS + 30_000,
);

function createPlainContext(): Context {
	const context: Context = {
		data: createInitialDataState({ projectId: "local" }),
		view: createInitialViewState(),
		commands: undefined as unknown as Context["commands"],
	};
	context.commands = createCommands(context);
	return context;
}

async function expectBackendReady(): Promise<void> {
	const response = await fetch(`${LIVE_E2E_API_ROOT}/status`);
	if (!response.ok) {
		throw new Error(
			`live backend is not ready at ${LIVE_E2E_API_ROOT}: ${response.status}`,
		);
	}
}

async function withTimeout<T>(
	promise: Promise<T>,
	timeoutMs: number,
	label: string,
): Promise<T> {
	let timeout: ReturnType<typeof setTimeout> | undefined;
	try {
		return await Promise.race([
			promise,
			new Promise<never>((_, reject) => {
				timeout = setTimeout(
					() => reject(new Error(`${label} timed out after ${timeoutMs}ms`)),
					timeoutMs,
				);
			}),
		]);
	} finally {
		if (timeout) clearTimeout(timeout);
	}
}

function countSessionEvents(sessionId: string, eventName: string): number {
	return receivedSocketMessages.filter(
		(message) =>
			typeof message === "object" &&
			message !== null &&
			"stream" in message &&
			message.stream === "session" &&
			"sessionId" in message &&
			message.sessionId === sessionId &&
			"event" in message &&
			message.event === eventName,
	).length;
}

function countChatEvents(
	sessionId: string,
	threadId: string,
	eventName: string,
): number {
	return receivedSocketMessages.filter(
		(message) =>
			typeof message === "object" &&
			message !== null &&
			"stream" in message &&
			message.stream === "chat" &&
			"sessionId" in message &&
			message.sessionId === sessionId &&
			"threadId" in message &&
			message.threadId === threadId &&
			"event" in message &&
			message.event === eventName,
	).length;
}

async function waitFor(
	condition: () => boolean,
	label: string,
	timeoutMs = LIVE_E2E_TIMEOUT_MS,
): Promise<void> {
	const deadline = Date.now() + timeoutMs;
	while (Date.now() < deadline) {
		if (condition()) return;
		await new Promise((resolve) => setTimeout(resolve, WAIT_INTERVAL_MS));
	}
	throw new Error(
		`${label} timed out after ${timeoutMs}ms. Thread messages: ${JSON.stringify(
			receivedSocketMessages.filter(
				(message) =>
					typeof message === "object" &&
					message !== null &&
					"event" in message &&
					message.event === "threads_updated",
			),
		)}. Recent websocket messages: ${JSON.stringify(
			receivedSocketMessages.slice(-20),
		)}`,
	);
}
