import type {
	ProjectEventSocket,
	ProjectSocketMessage,
	ProjectSocketSubscription,
} from "$lib/context/project-subscription";

export type ServiceOutputSubscription = {
	open(): Promise<void>;
	close(): void;
};

export function connectServiceOutput(args: {
	socket: ProjectEventSocket;
	sessionId: string;
	serviceId: string;
	onOpen?: () => void;
	onClose?: () => void;
	onError?: (error: unknown) => void;
	onMessage: (data: string) => void;
}): ServiceOutputSubscription {
	let transport: ProjectSocketSubscription | null = null;

	return {
		open() {
			transport?.close();
			transport = args.socket.subscribe(
				{
					type: "subscribe",
					stream: "service",
					sessionId: args.sessionId,
					serviceId: args.serviceId,
				},
				{
					onMessage: handleMessage,
					onError: args.onError,
				},
			);
			return transport.open();
		},
		close() {
			args.onClose?.();
			transport?.close();
			transport = null;
		},
	};

	function handleMessage(message: ProjectSocketMessage): void {
		if (
			message.stream !== "service" ||
			message.sessionId !== args.sessionId ||
			message.serviceId !== args.serviceId
		) {
			return;
		}

		switch (message.type) {
			case "subscribed":
				args.onOpen?.();
				return;
			case "event":
				if (typeof message.data === "string") {
					args.onMessage(message.data);
				}
				return;
			case "complete":
			case "unsubscribed":
				args.onClose?.();
				return;
			case "error":
				args.onError?.(
					new Error(message.error || "Service output subscription failed"),
				);
				return;
		}
	}
}
