import { readStorage, writeStorage } from "$lib/local-storage";
import type { ConversationComment } from "$lib/context/context.types";

const CONVERSATION_COMMENTS_STORAGE_PREFIX = "discboeing:conversation-comments:";
export const PENDING_CONVERSATION_COMMENTS_STORAGE_KEY = `${CONVERSATION_COMMENTS_STORAGE_PREFIX}pending`;

export function resolveConversationCommentsStorageKey({
	isPending,
	threadId,
	sessionId,
}: {
	isPending: boolean;
	threadId: string | null | undefined;
	sessionId?: string | null | undefined;
}): string {
	const resolvedId = threadId || sessionId;
	if (isPending || !resolvedId) {
		return PENDING_CONVERSATION_COMMENTS_STORAGE_KEY;
	}

	return `${CONVERSATION_COMMENTS_STORAGE_PREFIX}${resolvedId}`;
}

function normalizeConversationComments(value: unknown): ConversationComment[] {
	if (!Array.isArray(value)) {
		return [];
	}

	return value.flatMap((item) => {
		if (!item || typeof item !== "object") {
			return [];
		}
		const candidate = item as Partial<ConversationComment>;
		const id = typeof candidate.id === "string" ? candidate.id.trim() : "";
		const snippet =
			typeof candidate.snippet === "string" ? candidate.snippet.trim() : "";
		const comment =
			typeof candidate.comment === "string" ? candidate.comment.trim() : "";
		if (!id || !snippet || !comment) {
			return [];
		}
		return [{ id, snippet, comment }];
	});
}

export function serializeConversationComments(
	comments: ConversationComment[],
): string {
	return JSON.stringify(normalizeConversationComments(comments));
}

export function readConversationComments(
	storageKey: string,
): ConversationComment[] {
	const raw = readStorage(storageKey);
	if (!raw) {
		return [];
	}

	try {
		return normalizeConversationComments(JSON.parse(raw));
	} catch {
		return [];
	}
}

export function writeConversationComments(
	storageKey: string,
	comments: ConversationComment[],
): void {
	const normalizedComments = normalizeConversationComments(comments);
	writeStorage(
		storageKey,
		normalizedComments.length > 0 ? JSON.stringify(normalizedComments) : null,
	);
}

export function clearConversationComments(storageKey: string): void {
	writeStorage(storageKey, null);
}
