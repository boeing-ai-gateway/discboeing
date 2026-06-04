import type { ChatMessage } from "$lib/api-types";
import type { ThreadContextValue } from "$lib/session/session-context.types";
import {
	addRuntimeThreadPendingComment,
	cancelRuntimeThread,
	clearRuntimeComposerDraft,
	clearRuntimeThreadNextComposerValues,
	clearRuntimeThreadPendingComments,
	deleteRuntimeQueuedPrompt,
	refreshRuntimeThread,
	removeRuntimeThreadPendingComment,
	setRuntimeComposerDraft,
	setRuntimeConversationScrollTop,
	setRuntimeThreadNextModelId,
	setRuntimeThreadNextReasoning,
	setRuntimeThreadNextServiceTier,
	submitRuntimeThread,
	updateRuntimeQueuedPrompt,
} from "$lib/app/app-runtime.svelte";

export async function submitThread(
	sessionId: string,
	threadId: string,
	payload: {
		parts: ChatMessage["parts"];
		workspaceId?: string;
		providerId?: string;
		workspaceType?: "local" | "git" | null;
		workspacePath?: string | null;
		allowEmptyPendingMessage?: boolean;
		runAfter?: string;
	},
): ReturnType<typeof submitRuntimeThread> {
	return submitRuntimeThread(sessionId, threadId, payload);
}

export async function cancelThread(
	sessionId: string,
	threadId: string,
): Promise<void> {
	await cancelRuntimeThread(sessionId, threadId);
}

export async function refreshThread(
	sessionId: string,
	threadId: string,
): Promise<void> {
	await refreshRuntimeThread(sessionId, threadId);
}

export function setComposerDraft(sessionId: string, value: string): void {
	setRuntimeComposerDraft(sessionId, value);
}

export function clearComposerDraft(
	sessionId: string,
	threadId: string,
	storageKey?: string,
): void {
	clearRuntimeComposerDraft(sessionId, threadId, storageKey);
}

export function setThreadNextModelId(
	sessionId: string,
	threadId: string,
	modelId: string | null | undefined,
): void {
	setRuntimeThreadNextModelId(sessionId, threadId, modelId);
}

export function setThreadNextReasoning(
	sessionId: string,
	threadId: string,
	reasoning: string | undefined,
): void {
	setRuntimeThreadNextReasoning(sessionId, threadId, reasoning);
}

export function setThreadNextServiceTier(
	sessionId: string,
	threadId: string,
	serviceTier: string | null | undefined,
): void {
	setRuntimeThreadNextServiceTier(sessionId, threadId, serviceTier);
}

export function clearThreadNextComposerValues(
	sessionId: string,
	threadId: string,
): void {
	clearRuntimeThreadNextComposerValues(sessionId, threadId);
}

export function addThreadPendingComment(
	sessionId: string,
	threadId: string,
	comment: Parameters<ThreadContextValue["addPendingComment"]>[0],
): void {
	addRuntimeThreadPendingComment(sessionId, threadId, comment);
}

export function removeThreadPendingComment(
	sessionId: string,
	threadId: string,
	commentId: string,
): void {
	removeRuntimeThreadPendingComment(sessionId, threadId, commentId);
}

export function clearThreadPendingComments(
	sessionId: string,
	threadId: string,
): void {
	clearRuntimeThreadPendingComments(sessionId, threadId);
}

export async function deleteQueuedPrompt(
	sessionId: string,
	threadId: string,
	queueId: string,
): Promise<void> {
	await deleteRuntimeQueuedPrompt(sessionId, threadId, queueId);
}

export async function updateQueuedPrompt(
	sessionId: string,
	threadId: string,
	queueId: string,
	payload: Parameters<ThreadContextValue["updateQueuedPrompt"]>[1],
): Promise<void> {
	await updateRuntimeQueuedPrompt(sessionId, threadId, queueId, payload);
}

export function setConversationScrollTop(
	sessionId: string,
	threadId: string,
	scrollTop: number,
): void {
	setRuntimeConversationScrollTop(sessionId, threadId, scrollTop);
}
