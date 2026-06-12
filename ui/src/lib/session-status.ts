import {
	resolveSessionDisplayStatus as resolveFromSession,
	resolveThreadDisplayStatus as resolveThreadFromSession,
} from "$lib/session-status-helpers";
import type { SessionRecord } from "$lib/context/domains/sessions";

export function resolveSessionDisplayStatus(
	record: SessionRecord | undefined,
): string {
	if (!record?.value) {
		return "unknown";
	}
	return resolveFromSession(record.value);
}

export function resolveThreadDisplayStatus(
	record: SessionRecord | undefined,
	threadId: string,
): string {
	if (!record?.value) {
		return "unknown";
	}

	const thread = record.threads.byId[threadId]?.value ?? null;
	const content = record.threads.byId[threadId]?.content;
	const hasPendingQuestion =
		!!content?.pendingQuestionId &&
		!content.answeredApprovalIds[content.pendingQuestionId];
	return resolveThreadFromSession({
		session: record.value,
		sessionThreadStatus:
			record.value.threadStatus?.threadId === threadId
				? record.value.threadStatus
				: undefined,
		thread,
		localActivityStatus: content?.isStreaming
			? { status: "running" }
			: hasPendingQuestion
				? { status: "needs_attention" }
				: null,
	});
}
