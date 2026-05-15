import { appendAuthToken, getWsBase } from "$lib/api-config";
import type {
	ProjectStreamSocketEventName,
	ProjectStreamSocketMessage,
	ProjectStreamSocketRequest,
} from "$lib/api-types";

const PROJECT_STREAM_RECONNECT_DELAY_MS = 1000;
const PROJECT_STREAM_RETRY_DELAY_MS = 1000;
const SERVICE_OUTPUT_EVENT_NAME = "message";

export type ProjectStreamEventName = ProjectStreamSocketEventName;

export type ProjectStreamEventSource<EventName extends string = string> = {
	addEventListener: (
		type: EventName,
		listener: (event: MessageEvent<string>) => void,
	) => void;
	removeEventListener: (
		type: EventName,
		listener: (event: MessageEvent<string>) => void,
	) => void;
};

export type ProjectStreamEventListenerBinding<
	EventName extends string = string,
> = {
	type: EventName;
	listener: (event: MessageEvent<string>) => void;
};

type BaseSubscription<EventSource> = {
	eventSource: EventSource;
	unsubscribe: () => void;
};

export type ProjectStreamSubscription = BaseSubscription<
	ProjectStreamEventSource<ProjectStreamEventName>
>;
export type ServiceOutputSubscription = BaseSubscription<
	ProjectStreamEventSource<typeof SERVICE_OUTPUT_EVENT_NAME>
>;
export type ProjectEventsSubscription = BaseSubscription<
	ProjectStreamEventSource<string>
>;

export type ProjectStreamManager = {
	subscribe: (args: {
		sessionId: string;
		threadId: string;
		replay?: boolean;
		lastEventId?: string;
		listeners?: ProjectStreamEventListenerBinding<ProjectStreamEventName>[];
		onOpen?: () => void;
		onError?: (error: unknown) => void;
	}) => ProjectStreamSubscription;
	subscribeServiceOutput: (args: {
		sessionId: string;
		serviceId: string;
		onOpen?: () => void;
		onError?: (error: unknown) => void;
	}) => ServiceOutputSubscription;
	subscribeProjectEvents: (args?: {
		afterId?: string;
		onOpen?: () => void;
		onError?: (error: unknown) => void;
	}) => ProjectEventsSubscription;
	dispose: () => void;
};

class StreamSource<
	EventName extends string = string,
> implements ProjectStreamEventSource<EventName> {
	private listeners = new Map<
		string,
		Set<(event: MessageEvent<string>) => void>
	>();

	addEventListener(
		type: EventName,
		listener: (event: MessageEvent<string>) => void,
	) {
		let listeners = this.listeners.get(type);
		if (!listeners) {
			listeners = new Set();
			this.listeners.set(type, listeners);
		}
		listeners.add(listener);
	}

	removeEventListener(
		type: EventName,
		listener: (event: MessageEvent<string>) => void,
	) {
		this.listeners.get(type)?.delete(listener);
	}

	dispatch(type: EventName, data: string) {
		const message = new MessageEvent<string>(type, { data });
		for (const listener of this.listeners.get(type) ?? []) {
			listener(message);
		}
	}
}

type StreamConsumer = {
	onOpen?: () => void;
	onError?: (error: unknown) => void;
};

type StreamEntry = {
	key: string;
	retryTimeoutId: number | null;
	source: StreamSource;
	consumers: Map<symbol, StreamConsumer>;
	subscribeMessage: () => ProjectStreamSocketRequest;
	unsubscribeMessage: () => ProjectStreamSocketRequest;
	handleEvent: (message: ProjectStreamSocketMessage) => void;
	matchesMessage: (message: ProjectStreamSocketMessage) => boolean;
};

function chatEntryKey(sessionId: string, threadId: string) {
	return `chat:${sessionId}:${threadId}`;
}

function serviceEntryKey(sessionId: string, serviceId: string) {
	return `service:${sessionId}:${serviceId}`;
}

function projectEventsEntryKey() {
	return "project-events";
}

function matchesEntryMessage(
	entry: StreamEntry,
	message: ProjectStreamSocketMessage,
) {
	return entry.matchesMessage(message);
}

export function createProjectStreamManager(): ProjectStreamManager {
	let socket: WebSocket | null = null;
	let reconnectTimeoutId: number | null = null;
	let disposed = false;
	const entries = new Map<string, StreamEntry>();

	const clearReconnectTimeout = () => {
		if (reconnectTimeoutId !== null) {
			window.clearTimeout(reconnectTimeoutId);
			reconnectTimeoutId = null;
		}
	};

	const notifyError = (entry: StreamEntry, error: unknown) => {
		for (const consumer of entry.consumers.values()) {
			consumer.onError?.(error);
		}
	};

	const notifyOpen = (entry: StreamEntry) => {
		for (const consumer of entry.consumers.values()) {
			consumer.onOpen?.();
		}
	};

	const clearEntryRetry = (entry: StreamEntry) => {
		if (entry.retryTimeoutId !== null) {
			window.clearTimeout(entry.retryTimeoutId);
			entry.retryTimeoutId = null;
		}
	};

	const closeSocket = () => {
		const current = socket;
		socket = null;
		current?.close();
	};

	const send = (message: ProjectStreamSocketRequest) => {
		if (socket?.readyState !== WebSocket.OPEN) {
			return false;
		}
		socket.send(JSON.stringify(message));
		return true;
	};

	const sendSubscribe = (entry: StreamEntry) => {
		clearEntryRetry(entry);
		console.debug("[WS] Subscribing to project stream", {
			key: entry.key,
			message: entry.subscribeMessage(),
		});
		send(entry.subscribeMessage());
	};

	const scheduleEntryRetry = (entry: StreamEntry) => {
		if (
			disposed ||
			typeof window === "undefined" ||
			entry.retryTimeoutId !== null ||
			!entries.has(entry.key)
		) {
			return;
		}
		console.warn("[WS] Project stream error; scheduling retry", {
			key: entry.key,
			delayMs: PROJECT_STREAM_RETRY_DELAY_MS,
		});
		entry.retryTimeoutId = window.setTimeout(() => {
			entry.retryTimeoutId = null;
			if (!entries.has(entry.key)) {
				return;
			}
			ensureSocket();
			if (socket?.readyState === WebSocket.OPEN) {
				sendSubscribe(entry);
			}
		}, PROJECT_STREAM_RETRY_DELAY_MS);
	};

	const scheduleReconnect = () => {
		if (
			disposed ||
			typeof window === "undefined" ||
			reconnectTimeoutId !== null ||
			entries.size === 0
		) {
			return;
		}
		console.warn("[WS] Project stream socket closed; scheduling reconnect", {
			delayMs: PROJECT_STREAM_RECONNECT_DELAY_MS,
			entries: [...entries.values()].map((entry) => ({
				key: entry.key,
			})),
		});
		reconnectTimeoutId = window.setTimeout(() => {
			reconnectTimeoutId = null;
			ensureSocket();
		}, PROJECT_STREAM_RECONNECT_DELAY_MS);
	};

	const handleMessage = (message: ProjectStreamSocketMessage) => {
		const entry = [...entries.values()].find((candidate) =>
			matchesEntryMessage(candidate, message),
		);
		if (!entry) {
			return;
		}

		switch (message.type) {
			case "subscribed":
				clearEntryRetry(entry);
				console.debug("[WS] Project stream subscribed", {
					key: entry.key,
					stream: message.stream,
				});
				notifyOpen(entry);
				return;
			case "event":
				entry.handleEvent(message);
				return;
			case "complete":
			case "unsubscribed":
				console.debug("[WS] Project stream ended", {
					key: entry.key,
					reason: message.type,
					stream: message.stream,
				});
				removeEntry(entry, { sendUnsubscribe: false });
				return;
			case "error":
				console.warn("[WS] Project stream error", {
					key: entry.key,
					stream: message.stream,
					error: message.error || "Failed to process project stream",
				});
				notifyError(
					entry,
					new Error(message.error || "Failed to process project stream"),
				);
				scheduleEntryRetry(entry);
				return;
		}
	};

	const ensureSocket = () => {
		if (
			disposed ||
			typeof window === "undefined" ||
			socket ||
			entries.size === 0
		) {
			return;
		}

		clearReconnectTimeout();
		const nextSocket = new WebSocket(appendAuthToken(`${getWsBase()}/ws`));
		socket = nextSocket;

		nextSocket.onopen = () => {
			if (socket !== nextSocket) {
				return;
			}
			console.debug("[WS] Project stream socket opened", {
				entries: [...entries.values()].map((entry) => ({
					key: entry.key,
				})),
			});
			for (const entry of entries.values()) {
				sendSubscribe(entry);
			}
		};

		nextSocket.onmessage = (event) => {
			if (socket !== nextSocket) {
				return;
			}
			try {
				handleMessage(JSON.parse(event.data) as ProjectStreamSocketMessage);
			} catch (error) {
				console.error(
					"Failed to parse project stream websocket message",
					error,
				);
			}
		};

		nextSocket.onerror = () => {
			if (socket !== nextSocket) {
				return;
			}
			console.warn("[WS] Project stream socket error", {
				entries: [...entries.values()].map((entry) => ({
					key: entry.key,
				})),
			});
		};

		nextSocket.onclose = () => {
			if (socket !== nextSocket) {
				return;
			}
			socket = null;
			for (const entry of entries.values()) {
				notifyError(entry, new Error("Lost project stream connection"));
			}
			scheduleReconnect();
		};
	};

	const removeEntry = (
		entry: StreamEntry,
		options: { sendUnsubscribe?: boolean } = {},
	) => {
		clearEntryRetry(entry);
		entries.delete(entry.key);
		if (
			options.sendUnsubscribe !== false &&
			socket?.readyState === WebSocket.OPEN
		) {
			send(entry.unsubscribeMessage());
		}
		if (entries.size === 0) {
			clearReconnectTimeout();
			closeSocket();
		}
	};

	const createSubscription = (
		key: string,
		getOrCreateEntry: () => StreamEntry,
		onOpen?: () => void,
		onError?: (error: unknown) => void,
		listenerBindings?: Array<{
			type: string;
			listener: (event: MessageEvent<string>) => void;
		}>,
	): BaseSubscription<ProjectStreamEventSource> => {
		const wasSubscribed = entries.has(key);
		const entry = getOrCreateEntry();
		for (const binding of listenerBindings ?? []) {
			entry.source.addEventListener(binding.type, binding.listener);
		}
		const consumerId = Symbol(key);
		entry.consumers.set(consumerId, { onOpen, onError });
		ensureSocket();
		if (!wasSubscribed && socket?.readyState === WebSocket.OPEN) {
			sendSubscribe(entry);
		}

		return {
			eventSource: entry.source,
			unsubscribe: () => {
				const currentEntry = entries.get(key);
				if (!currentEntry) {
					return;
				}
				for (const binding of listenerBindings ?? []) {
					currentEntry.source.removeEventListener(
						binding.type,
						binding.listener,
					);
				}
				currentEntry.consumers.delete(consumerId);
				if (currentEntry.consumers.size === 0) {
					removeEntry(currentEntry);
				}
			},
		};
	};

	return {
		subscribe: ({
			sessionId,
			threadId,
			replay = true,
			lastEventId = "",
			listeners,
			onOpen,
			onError,
		}) => {
			const key = chatEntryKey(sessionId, threadId);
			return createSubscription(
				key,
				() => {
					let entry = entries.get(key);
					if (!entry) {
						const source = new StreamSource<ProjectStreamEventName>();
						entry = {
							key,
							retryTimeoutId: null,
							source,
							consumers: new Map(),
							subscribeMessage: () => ({
								type: "subscribe",
								stream: "chat",
								sessionId,
								threadId,
								replay,
								...(lastEventId ? { lastEventId } : {}),
							}),
							unsubscribeMessage: () => ({
								type: "unsubscribe",
								stream: "chat",
								sessionId,
								threadId,
							}),
							handleEvent: (message) => {
								if (message.id) {
									lastEventId = message.id;
								}
								if (message.event && typeof message.data === "string") {
									source.dispatch(
										message.event as ProjectStreamEventName,
										message.data,
									);
								}
							},
							matchesMessage: (message) =>
								message.stream === "chat" &&
								message.sessionId === sessionId &&
								message.threadId === threadId,
						};
						entries.set(key, entry);
					} else {
						entry.subscribeMessage = () => ({
							type: "subscribe",
							stream: "chat",
							sessionId,
							threadId,
							replay,
							...(lastEventId ? { lastEventId } : {}),
						});
					}
					return entry;
				},
				onOpen,
				onError,
				listeners,
			) as unknown as ProjectStreamSubscription;
		},
		subscribeServiceOutput: ({ sessionId, serviceId, onOpen, onError }) => {
			const key = serviceEntryKey(sessionId, serviceId);
			return createSubscription(
				key,
				() => {
					let entry = entries.get(key);
					if (!entry) {
						const source = new StreamSource<typeof SERVICE_OUTPUT_EVENT_NAME>();
						entry = {
							key,
							retryTimeoutId: null,
							source,
							consumers: new Map(),
							subscribeMessage: () => ({
								type: "subscribe",
								stream: "service",
								sessionId,
								serviceId,
							}),
							unsubscribeMessage: () => ({
								type: "unsubscribe",
								stream: "service",
								sessionId,
								serviceId,
							}),
							handleEvent: (message) => {
								if (typeof message.data === "string") {
									source.dispatch(SERVICE_OUTPUT_EVENT_NAME, message.data);
								}
							},
							matchesMessage: (message) =>
								message.stream === "service" &&
								message.sessionId === sessionId &&
								message.serviceId === serviceId,
						};
						entries.set(key, entry);
					}
					return entry;
				},
				onOpen,
				onError,
			) as unknown as ServiceOutputSubscription;
		},
		subscribeProjectEvents: ({ afterId = "", onOpen, onError } = {}) => {
			const key = projectEventsEntryKey();
			return createSubscription(
				key,
				() => {
					let entry = entries.get(key);
					if (!entry) {
						const source = new StreamSource<string>();
						entry = {
							key,
							retryTimeoutId: null,
							source,
							consumers: new Map(),
							subscribeMessage: () => ({
								type: "subscribe",
								stream: "project-events",
								...(afterId ? { afterId } : {}),
							}),
							unsubscribeMessage: () => ({
								type: "unsubscribe",
								stream: "project-events",
							}),
							handleEvent: (message) => {
								if (message.event && typeof message.data === "string") {
									source.dispatch(message.event, message.data);
								}
							},
							matchesMessage: (message) => message.stream === "project-events",
						};
						entries.set(key, entry);
					} else {
						entry.subscribeMessage = () => ({
							type: "subscribe",
							stream: "project-events",
							...(afterId ? { afterId } : {}),
						});
					}
					return entry;
				},
				onOpen,
				onError,
			) as unknown as ProjectEventsSubscription;
		},
		dispose: () => {
			disposed = true;
			clearReconnectTimeout();
			for (const entry of entries.values()) {
				clearEntryRetry(entry);
			}
			entries.clear();
			closeSocket();
		},
	};
}
