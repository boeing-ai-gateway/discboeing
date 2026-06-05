import { appendAuthToken, getWsBase } from "$lib/api-config";
import type {
	ProjectStreamSocketMessage,
	ProjectStreamSocketRequest,
	ProjectStreamSubscriptionState,
} from "$lib/api-types";
import type {
	ChatStreamEventListenerBinding,
	ChatStreamEventName,
	ChatStreamEventSource,
} from "$lib/thread/conversation-stream";

const CHAT_STREAM_RECONNECT_DELAY_MS = 1000;
const SERVICE_OUTPUT_EVENT_NAME = "message";

type ProjectStreamEventSource<EventName extends string = string> = {
	addEventListener: (
		type: EventName,
		listener: (event: MessageEvent<string>) => void,
	) => void;
	removeEventListener: (
		type: EventName,
		listener: (event: MessageEvent<string>) => void,
	) => void;
};

type BaseSubscription<EventSource> = {
	eventSource: EventSource;
	unsubscribe: () => void;
	resubscribe: () => void;
	getState: () => ProjectStreamSubscriptionState;
};

export type ChatStreamSubscription = BaseSubscription<ChatStreamEventSource>;
export type ServiceOutputSubscription = BaseSubscription<
	ProjectStreamEventSource<typeof SERVICE_OUTPUT_EVENT_NAME>
>;
export type ProjectEventsSubscription = BaseSubscription<
	ProjectStreamEventSource<string>
>;

export type ChatStreamManager = {
	subscribe: (args: {
		sessionId: string;
		threadId: string;
		replay?: boolean;
		lastEventId?: string;
		listeners?: ChatStreamEventListenerBinding[];
		onOpen?: () => void;
		onError?: (error: unknown) => void;
	}) => ChatStreamSubscription;
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
	state: ProjectStreamSubscriptionState;
	wantsSubscription: boolean;
	resumeAfterId?: string;
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

export function createChatStreamManager(): ChatStreamManager {
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
		entry.wantsSubscription = true;
		entry.state = "subscribing";
		console.debug("[WS] Subscribing to project stream", {
			key: entry.key,
			message: entry.subscribeMessage(),
		});
		if (!send(entry.subscribeMessage())) {
			entry.state = "idle";
		}
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
			delayMs: CHAT_STREAM_RECONNECT_DELAY_MS,
			entries: [...entries.values()].map((entry) => ({
				key: entry.key,
				state: entry.state,
				wantsSubscription: entry.wantsSubscription,
			})),
		});
		reconnectTimeoutId = window.setTimeout(() => {
			reconnectTimeoutId = null;
			ensureSocket();
		}, CHAT_STREAM_RECONNECT_DELAY_MS);
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
				entry.state = "streaming";
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
				entry.state = "idle";
				entry.wantsSubscription = false;
				console.debug("[WS] Project stream became idle", {
					key: entry.key,
					reason: message.type,
					stream: message.stream,
				});
				return;
			case "error":
				entry.state = "idle";
				entry.wantsSubscription = false;
				console.warn("[WS] Project stream error", {
					key: entry.key,
					stream: message.stream,
					error: message.error || "Failed to process project stream",
				});
				notifyError(
					entry,
					new Error(message.error || "Failed to process project stream"),
				);
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
					state: entry.state,
					wantsSubscription: entry.wantsSubscription,
				})),
			});
			for (const entry of entries.values()) {
				if (!entry.wantsSubscription) {
					continue;
				}
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
					state: entry.state,
					wantsSubscription: entry.wantsSubscription,
				})),
			});
			for (const entry of entries.values()) {
				entry.state = "idle";
			}
		};

		nextSocket.onclose = () => {
			if (socket !== nextSocket) {
				return;
			}
			socket = null;
			for (const entry of entries.values()) {
				const wasActive = entry.state !== "idle";
				entry.state = "idle";
				if (wasActive) {
					notifyError(entry, new Error("Lost project stream connection"));
				}
			}
			scheduleReconnect();
		};
	};

	const removeEntry = (entry: StreamEntry) => {
		entries.delete(entry.key);
		if (socket?.readyState === WebSocket.OPEN) {
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
		const entry = getOrCreateEntry();
		for (const binding of listenerBindings ?? []) {
			entry.source.addEventListener(binding.type, binding.listener);
		}
		const consumerId = Symbol(key);
		entry.consumers.set(consumerId, { onOpen, onError });
		ensureSocket();
		if (entry.state === "idle" && socket?.readyState === WebSocket.OPEN) {
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
			resubscribe: () => {
				const currentEntry = entries.get(key);
				if (!currentEntry) {
					return;
				}
				currentEntry.wantsSubscription = true;
				console.debug("[WS] Project stream resubscribe requested", {
					key: currentEntry.key,
					state: currentEntry.state,
				});
				ensureSocket();
				if (socket?.readyState === WebSocket.OPEN) {
					sendSubscribe(currentEntry);
				}
			},
			getState: () => entries.get(key)?.state ?? "idle",
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
						const source = new StreamSource<ChatStreamEventName>();
						entry = {
							key,
							state: "idle",
							wantsSubscription: true,
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
										message.event as ChatStreamEventName,
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
			) as unknown as ChatStreamSubscription;
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
							state: "idle",
							wantsSubscription: true,
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
						const createdEntry: StreamEntry = {
							key,
							state: "idle",
							wantsSubscription: true,
							resumeAfterId: afterId,
							source,
							consumers: new Map(),
							subscribeMessage: () => ({
								type: "subscribe",
								stream: "project-events",
								...(createdEntry.resumeAfterId
									? { afterId: createdEntry.resumeAfterId }
									: {}),
							}),
							unsubscribeMessage: () => ({
								type: "unsubscribe",
								stream: "project-events",
							}),
							handleEvent: (message) => {
								if (message.id) {
									createdEntry.resumeAfterId = message.id;
								}
								if (message.event && typeof message.data === "string") {
									source.dispatch(message.event, message.data);
								}
							},
							matchesMessage: (message) => message.stream === "project-events",
						};
						entry = createdEntry;
						entries.set(key, entry);
					} else if (afterId && !entry.resumeAfterId) {
						entry.resumeAfterId = afterId;
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
			entries.clear();
			closeSocket();
		},
	};
}
