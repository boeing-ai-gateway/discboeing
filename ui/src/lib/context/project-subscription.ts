import { appendAuthToken, getWsBase } from "$lib/api-config";
import type {
	ProjectEventsStreamMessage,
	ProjectStreamSocketMessage,
	ProjectStreamSocketRequest,
} from "$lib/api-types";

export type ProjectSocketRequest = ProjectStreamSocketRequest;
export type ProjectSocketMessage = ProjectStreamSocketMessage;

const PROJECT_SUBSCRIPTION_RECONNECT_DELAY_MS = 1000;

export type ProjectEventSocket = {
	open(): Promise<void>;
	close(): void;
	subscribe(
		request: ProjectSocketRequest,
		options: ProjectSocketSubscriptionOptions,
	): ProjectSocketSubscription;
};

export type ProjectEventSocketOptions = {
	onEvent(message: ProjectEventsStreamMessage): void;
	onError?(error: unknown): void;
	onSocketMessage?(
		direction: "in" | "out",
		message: ProjectSocketMessage | ProjectSocketRequest,
	): void;
};

export type ProjectSocketSubscription = {
	open(): Promise<void>;
	close(): void;
};

export type ProjectSocketSubscriptionOptions = {
	onMessage(message: ProjectSocketMessage): void;
	onError?(error: unknown): void;
};

type ManagedSubscription = {
	key: string;
	request: ProjectSocketRequest;
	options: ProjectSocketSubscriptionOptions;
	active: boolean;
	subscribedResolve: (() => void) | null;
	subscribedReject: ((error: unknown) => void) | null;
};

export function connectProjectEvents(
	options: ProjectEventSocketOptions,
): ProjectEventSocket {
	let socket: WebSocket | null = null;
	let closed = false;
	let reconnectTimeoutId: number | null = null;
	let subscribedResolve: (() => void) | null = null;
	let subscribedReject: ((error: unknown) => void) | null = null;
	const subscriptions = new Map<string, ManagedSubscription>();

	const eventSocket: ProjectEventSocket = {
		open() {
			if (typeof window === "undefined") {
				return Promise.reject(
					new Error("Project events websocket requires a browser"),
				);
			}
			closed = false;
			closeSocket();
			const subscribed = createSubscribedPromise();
			openSocket();
			return subscribed;
		},
		close() {
			closed = true;
			clearReconnectTimeout();
			clearSubscribedPromise(new Error("Project event socket closed"));
			for (const subscription of subscriptions.values()) {
				clearSubscriptionPromise(
					subscription,
					new Error("Project event socket closed"),
				);
			}
			subscriptions.clear();
			closeSocket();
		},
		subscribe(request, subscriptionOptions) {
			const key = subscriptionKey(request);
			let subscription: ManagedSubscription | null = null;

			return {
				open() {
					if (typeof window === "undefined") {
						return Promise.reject(
							new Error("Project events websocket requires a browser"),
						);
					}
					if (closed) {
						return Promise.reject(new Error("Project event socket is closed"));
					}
					if (subscriptions.has(key)) {
						return Promise.reject(
							new Error(`Project stream subscription already exists: ${key}`),
						);
					}
					subscription = {
						key,
						request,
						options: subscriptionOptions,
						active: true,
						subscribedResolve: null,
						subscribedReject: null,
					};
					subscriptions.set(key, subscription);
					const subscribed = createSubscriptionPromise(subscription);
					sendSubscribe(subscription);
					return subscribed;
				},
				close() {
					if (!subscription) return;
					subscription.active = false;
					subscriptions.delete(subscription.key);
					clearSubscriptionPromise(
						subscription,
						new Error("Project stream subscription closed"),
					);
					sendUnsubscribe(subscription.request);
					subscription = null;
				},
			};
		},
	};

	return eventSocket;

	function openSocket(): void {
		if (typeof window === "undefined" || closed) return;

		clearReconnectTimeout();
		const nextSocket = new WebSocket(appendAuthToken(`${getWsBase()}/ws`));
		socket = nextSocket;

		nextSocket.onopen = () => {
			if (socket !== nextSocket || closed) return;
			sendProjectEventsSubscribe();
			for (const subscription of subscriptions.values()) {
				sendSubscribe(subscription);
			}
		};

		nextSocket.onmessage = (event) => {
			if (socket !== nextSocket || closed) return;
			try {
				const message = JSON.parse(event.data) as ProjectSocketMessage;
				options.onSocketMessage?.("in", message);
				handleMessage(message);
			} catch (error) {
				options.onError?.(error);
			}
		};

		nextSocket.onerror = () => {
			if (socket !== nextSocket || closed) return;
			options.onError?.(new Error("Project event stream failed"));
		};

		nextSocket.onclose = () => {
			if (socket !== nextSocket || closed) return;
			socket = null;
			scheduleReconnect();
		};
	}

	function handleMessage(message: ProjectSocketMessage): void {
		if (message.stream === "project-events") {
			handleProjectEventsMessage(message as ProjectStreamSocketMessage);
			return;
		}

		const subscription = subscriptions.get(subscriptionKey(message));
		if (!subscription?.active) return;

		switch (message.type) {
			case "subscribed":
				clearSubscriptionPromise(subscription);
				return;
			case "error": {
				const error = new Error(
					message.error || "Project stream subscription failed",
				);
				clearSubscriptionPromise(subscription, error);
				subscription.options.onError?.(error);
				return;
			}
		}

		subscription.options.onMessage(message);
	}

	function handleProjectEventsMessage(
		message: ProjectStreamSocketMessage,
	): void {
		switch (message.type) {
			case "subscribed":
				clearSubscribedPromise();
				return;
			case "event":
				options.onEvent(message as ProjectEventsStreamMessage);
				return;
			case "error": {
				const error = new Error(
					message.error || "Project event subscription failed",
				);
				clearSubscribedPromise(error);
				options.onError?.(error);
				return;
			}
			case "complete":
			case "unsubscribed":
				return;
		}
	}

	function sendProjectEventsSubscribe(): void {
		const message: ProjectStreamSocketRequest = {
			type: "subscribe",
			stream: "project-events",
		};
		options.onSocketMessage?.("out", message);
		socket?.send(JSON.stringify(message));
	}

	function sendSubscribe(subscription: ManagedSubscription): void {
		if (!socket || socket.readyState !== WebSocket.OPEN) return;
		const message = {
			...subscription.request,
			type: "subscribe",
		} satisfies ProjectSocketRequest;
		options.onSocketMessage?.("out", message);
		socket.send(JSON.stringify(message));
	}

	function sendUnsubscribe(request: ProjectSocketRequest): void {
		if (!socket || socket.readyState !== WebSocket.OPEN) return;
		const message = {
			...request,
			type: "unsubscribe",
		} as ProjectSocketRequest;
		options.onSocketMessage?.("out", message);
		socket.send(JSON.stringify(message));
	}

	function scheduleReconnect(): void {
		if (
			typeof window === "undefined" ||
			closed ||
			reconnectTimeoutId !== null
		) {
			return;
		}

		reconnectTimeoutId = window.setTimeout(() => {
			reconnectTimeoutId = null;
			if (closed) return;
			openSocket();
		}, PROJECT_SUBSCRIPTION_RECONNECT_DELAY_MS);
	}

	function createSubscribedPromise(): Promise<void> {
		clearSubscribedPromise(new Error("Project event subscription replaced"));
		return new Promise((resolve, reject) => {
			subscribedResolve = resolve;
			subscribedReject = reject;
		});
	}

	function clearSubscribedPromise(error?: unknown): void {
		const resolve = subscribedResolve;
		const reject = subscribedReject;
		subscribedResolve = null;
		subscribedReject = null;
		if (error) reject?.(error);
		else resolve?.();
	}

	function createSubscriptionPromise(
		subscription: ManagedSubscription,
	): Promise<void> {
		clearSubscriptionPromise(
			subscription,
			new Error("Project stream subscription replaced"),
		);
		return new Promise((resolve, reject) => {
			subscription.subscribedResolve = resolve;
			subscription.subscribedReject = reject;
		});
	}

	function clearSubscriptionPromise(
		subscription: ManagedSubscription,
		error?: unknown,
	): void {
		const resolve = subscription.subscribedResolve;
		const reject = subscription.subscribedReject;
		subscription.subscribedResolve = null;
		subscription.subscribedReject = null;
		if (error) reject?.(error);
		else resolve?.();
	}

	function clearReconnectTimeout(): void {
		if (reconnectTimeoutId !== null) {
			window.clearTimeout(reconnectTimeoutId);
			reconnectTimeoutId = null;
		}
	}

	function closeSocket(): void {
		const current = socket;
		socket = null;
		current?.close();
	}
}

type SubscriptionKeyFields = {
	stream?: string;
	sessionId?: string;
	threadId?: string;
	serviceId?: string;
};

function subscriptionKey(message: SubscriptionKeyFields): string {
	return [
		message.stream ?? "",
		message.sessionId ?? "",
		message.threadId ?? "",
		message.serviceId ?? "",
	].join("\u0000");
}
