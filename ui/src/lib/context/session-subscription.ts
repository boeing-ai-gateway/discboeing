import type { SessionStreamMessage } from "$lib/api-types";
import type { Context } from "$lib/context/context.types";
import {
	applySessionEvent,
	createSessionRecord,
	ensureSessionRecord,
	type SessionRecord,
} from "$lib/context/domains/sessions";
import { preserveThreadRuntimeState } from "$lib/context/domains/threads";
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

export type SessionSubscription = StreamSubscription;

export function connectSessionEvents(
	context: Context,
	sessionId: string,
	socket: ProjectEventSocket,
): SessionSubscription {
	let historyTarget: SessionRecord | null = null;
	const request = {
		type: "subscribe",
		stream: "session",
		sessionId,
	} as const;

	return createProjectStreamSubscription<SessionStreamMessage>({
		socket,
		request,
		matches(message) {
			return message.stream === "session" && message.sessionId === sessionId;
		},
		onOpen() {
			historyTarget = null;
			openDebugSubscription(context, request);
		},
		onClose() {
			historyTarget = null;
			closeDebugSubscription(context, request);
		},
		onEvent: (message, controls) => {
			logDebugSubscriptionEvent(context, request, message.event);
			if (message.event === "history-end") {
				publishHistoryTarget();
				controls.resolveReady();
				activateDebugSubscription(context, request);
			}
			if (message.event === "history-start") {
				const existing = context.data.sessions.byId[sessionId];
				historyTarget = createSessionRecord(sessionId, existing?.value ?? null);
				historyTarget.files.activeSubtrees[""] = true;
				return;
			}
			return applySessionEvent(targetRecord(), message);
		},
		closedError: "Session subscription closed",
		replacedError: "Session subscription replaced",
		subscriptionError: "Session subscription failed",
	});

	function targetRecord(): SessionRecord {
		return (
			historyTarget ?? ensureSessionRecord(context.data.sessions, sessionId)
		);
	}

	function publishHistoryTarget(): void {
		if (!historyTarget) return;
		const record = ensureSessionRecord(context.data.sessions, sessionId);
		const subscription = record.subscription;
		preserveThreadRuntimeState(historyTarget, record);
		Object.assign(record, historyTarget, { subscription });
		historyTarget = null;
	}
}
