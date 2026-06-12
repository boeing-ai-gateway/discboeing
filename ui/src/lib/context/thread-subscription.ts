import type { ChatStreamMessage } from "$lib/api-types";
import type { Context } from "$lib/context/context.types";
import { ensureSessionRecord } from "$lib/context/domains/sessions";
import {
	applyThreadEvent,
	createThreadEventTarget,
	ensureThreadContentState,
} from "$lib/context/domains/threads";
import type { ProjectEventSocket } from "$lib/context/project-subscription";
import {
	createProjectStreamSubscription,
	type StreamSubscription,
} from "$lib/context/stream-subscription";
import {
	activateDebugSubscription,
	closeDebugSubscription,
	logDebugSubscriptionEvent,
	openDebugSubscription,
} from "$lib/context/debug";

export type ThreadSubscription = StreamSubscription;

export function connectThreadEvents(
	context: Context,
	sessionId: string,
	threadId: string,
	socket: ProjectEventSocket,
): ThreadSubscription {
	const content = () =>
		ensureThreadContentState(
			ensureSessionRecord(context.data.sessions, sessionId).threads,
			threadId,
		);
	const target = createThreadEventTarget(context, sessionId, threadId);

	const request = {
		type: "subscribe",
		stream: "chat",
		sessionId,
		threadId,
	} as const;

	return createProjectStreamSubscription<ChatStreamMessage>({
		socket,
		request,
		matches(message) {
			return (
				message.stream === "chat" &&
				message.sessionId === sessionId &&
				message.threadId === threadId
			);
		},
		onOpen() {
			openDebugSubscription(context, request);
		},
		onClose() {
			closeDebugSubscription(context, request);
		},
		onEvent: (message, controls) => {
			if (message.event) {
				logDebugSubscriptionEvent(context, request, message.event);
				if (message.event === "history-end") {
					activateDebugSubscription(context, request);
				}
			}
			return applyThreadEvent(target, content(), message, controls);
		},
		closedError: "Thread subscription closed",
		replacedError: "Thread subscription replaced",
		subscriptionError: "Thread subscription failed",
	});
}
