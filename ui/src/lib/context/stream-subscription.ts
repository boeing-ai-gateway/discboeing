import type {
	ProjectEventSocket,
	ProjectSocketMessage,
	ProjectSocketRequest,
	ProjectSocketSubscription,
} from "$lib/context/project-subscription";

export type StreamSubscription = {
	open(): Promise<void>;
	close(): void;
};

export type StreamSubscriptionControls = {
	isClosed(): boolean;
	resolveReady(): void;
	rejectReady(error: unknown): void;
};

type CreateProjectStreamSubscriptionOptions<
	TMessage extends ProjectSocketMessage,
> = {
	socket: ProjectEventSocket;
	request: ProjectSocketRequest;
	matches(message: ProjectSocketMessage): boolean;
	onOpen?(): void;
	onClose?(): void;
	onEvent(
		message: TMessage,
		controls: StreamSubscriptionControls,
	): void | Promise<void>;
	closedError: string;
	replacedError: string;
	subscriptionError: string;
};

export function createProjectStreamSubscription<
	TMessage extends ProjectSocketMessage,
>(
	options: CreateProjectStreamSubscriptionOptions<TMessage>,
): StreamSubscription {
	let closed = false;
	let transport: ProjectSocketSubscription | null = null;
	let ready: Promise<void> | null = null;
	let readyResolve: (() => void) | null = null;
	let readyReject: ((error: unknown) => void) | null = null;

	const controls: StreamSubscriptionControls = {
		isClosed: () => closed,
		resolveReady: () => clearReadyPromise(),
		rejectReady: (error) => clearReadyPromise(error),
	};

	return {
		open() {
			if (ready) return ready;
			if (transport) return Promise.resolve();
			closed = false;
			options.onOpen?.();
			ready = createReadyPromise();
			transport = options.socket.subscribe(options.request, {
				onMessage: handleMessage,
				onError: clearReadyPromise,
			});
			void transport.open().catch(clearReadyPromise);
			return ready;
		},
		close() {
			closed = true;
			options.onClose?.();
			clearReadyPromise(new Error(options.closedError));
			transport?.close();
			transport = null;
			ready = null;
		},
	};

	function handleMessage(message: ProjectSocketMessage): void {
		if (closed || !options.matches(message)) return;

		switch (message.type) {
			case "subscribed":
				return;
			case "event":
				void Promise.resolve(
					options.onEvent(message as TMessage, controls),
				).catch((error) => {
					clearReadyPromise(error);
				});
				return;
			case "error":
				clearReadyPromise(
					new Error(
						("error" in message && message.error) || options.subscriptionError,
					),
				);
				return;
			case "complete":
			case "unsubscribed":
				return;
		}
	}

	function createReadyPromise(): Promise<void> {
		clearReadyPromise(new Error(options.replacedError));
		return new Promise((resolve, reject) => {
			readyResolve = resolve;
			readyReject = reject;
		});
	}

	function clearReadyPromise(error?: unknown): void {
		const resolve = readyResolve;
		const reject = readyReject;
		ready = null;
		readyResolve = null;
		readyReject = null;
		if (error) reject?.(error);
		else resolve?.();
	}
}
