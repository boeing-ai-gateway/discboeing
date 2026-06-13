import { api } from "$lib/api-client";
import type { ChatMessage, UpdateQueuedPromptRequest } from "$lib/api-types";
import {
	createUserMessageFromParts,
	getPendingQuestionApprovalId,
	hasUserMessageContent,
} from "$lib/conversation-helpers";
import { createErrorStatus, createReadyStatus } from "$lib/context/cache";
import {
	clearComposerDraft as clearStoredComposerDraft,
	moveComposerDraft,
	resolveComposerDraftStorageKey,
	writeComposerDraft,
} from "$lib/context/stores/composer-drafts";
import {
	clearConversationComments,
	resolveConversationCommentsStorageKey,
	writeConversationComments,
} from "$lib/context/stores/conversation-comments";
import type { ConversationComment, Context } from "$lib/context/context.types";
import { ensureSessionRecord } from "$lib/context/domains/sessions";
import {
	ensureThreadContentState,
	loadThreadIntoCache,
	peekThreadInCache,
} from "$lib/context/domains/threads";
import { ensureSessionView, ensureThreadView } from "$lib/context/domains/view";
import { resolveThreadComposerSubmitValues } from "$lib/thread-composer-helpers";

type SubmitThreadPayload = {
	parts: ChatMessage["parts"];
	workspaceId?: string;
	providerId?: string;
	workspaceType?: "local" | "git" | null;
	workspacePath?: string | null;
	allowEmptyPendingMessage?: boolean;
	runAfter?: string;
};

type SubmitThreadResult = {
	sessionId: string;
	threadId: string;
	queued: boolean;
};

function isPendingSession(context: Context, sessionId: string): boolean {
	return context.view.selection.pendingSessionId === sessionId;
}

function resolveActiveThreadId(
	context: Context,
	sessionId: string,
): string | null {
	if (context.view.selection.sessionId === sessionId) {
		return context.view.selection.threadId;
	}
	return context.view.selection.requestedThreadIdBySessionId[sessionId] ?? null;
}

function resolveDraftStorageKey(
	context: Context,
	sessionId: string,
	threadId: string | null | undefined,
): string {
	return resolveComposerDraftStorageKey({
		isPending: isPendingSession(context, sessionId),
		sessionId,
		threadId,
	});
}

function resolveCommentsStorageKey(
	context: Context,
	sessionId: string,
	threadId: string,
): string {
	return resolveConversationCommentsStorageKey({
		isPending: isPendingSession(context, sessionId),
		sessionId,
		threadId,
	});
}

function normalizeModelId(modelId: string | null): string | undefined {
	if (!modelId) {
		return undefined;
	}
	return modelId.endsWith(":thinking")
		? modelId.slice(0, -":thinking".length)
		: modelId;
}

function getThreadMessages(
	context: Context,
	sessionId: string,
	threadId: string,
): ChatMessage[] {
	return ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	).messages;
}

function setThreadMessages(
	context: Context,
	sessionId: string,
	threadId: string,
	messages: ChatMessage[],
): void {
	ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	).messages = messages;
}

function removeProvisionalSubmitMessage(
	messages: ChatMessage[],
	messageId: string,
): ChatMessage[] {
	return messages.filter((message) => message.id !== messageId);
}

function getSubmitMessages(userMessage: ChatMessage): ChatMessage[] {
	const submittedMessage = { ...userMessage };
	delete submittedMessage.provisional;
	return [submittedMessage];
}

function getCurrentComposerValues(
	context: Context,
	sessionId: string,
	threadId: string,
): {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
} {
	const thread = peekThreadInCache(context, sessionId, threadId);
	const useDefaultComposerPreferences = thread?.model === undefined;
	const defaultModel = context.view.app.preferences.defaultModel;
	const defaultReasoning = context.view.app.preferences.defaultReasoning;
	const defaultServiceTier = context.view.app.preferences.defaultServiceTier;
	const modelId =
		(thread?.model && thread.model.length > 0
			? thread.model
			: useDefaultComposerPreferences
				? defaultModel
				: null) || null;
	return {
		modelId,
		reasoning: useDefaultComposerPreferences
			? thread?.reasoning || defaultReasoning || undefined
			: (thread?.reasoning ?? undefined),
		serviceTier: useDefaultComposerPreferences
			? thread?.serviceTier || defaultServiceTier || undefined
			: (thread?.serviceTier ?? undefined),
	};
}

export async function setComposerDraft(
	context: Context,
	sessionId: string,
	value: string,
): Promise<void> {
	ensureSessionView(context, sessionId).composer.draft = value;
	const threadId = resolveActiveThreadId(context, sessionId);
	writeComposerDraft(
		resolveDraftStorageKey(context, sessionId, threadId),
		value,
	);
}

export async function clearComposerDraft(
	context: Context,
	sessionId: string,
	threadId: string,
	storageKey?: string,
): Promise<void> {
	ensureSessionView(context, sessionId).composer.draft = "";
	clearStoredComposerDraft(
		storageKey ?? resolveDraftStorageKey(context, sessionId, threadId),
	);
}

export async function movePendingComposerDraftToThread(
	_context: Context,
	threadId: string,
	nextThreadId: string,
	value: string,
): Promise<void> {
	moveComposerDraft({
		fromStorageKey: resolveComposerDraftStorageKey({
			isPending: true,
			threadId,
		}),
		toStorageKey: resolveComposerDraftStorageKey({
			isPending: false,
			threadId: nextThreadId,
		}),
		value,
	});
}

export async function setThreadNextModelId(
	context: Context,
	sessionId: string,
	threadId: string,
	modelId: string | null | undefined,
): Promise<void> {
	ensureThreadView(context, sessionId, threadId).composer.nextModelId = modelId;
}

export async function setThreadNextReasoning(
	context: Context,
	sessionId: string,
	threadId: string,
	reasoning: string | undefined,
): Promise<void> {
	ensureThreadView(context, sessionId, threadId).composer.nextReasoning =
		reasoning;
}

export async function setThreadNextServiceTier(
	context: Context,
	sessionId: string,
	threadId: string,
	serviceTier: string | null | undefined,
): Promise<void> {
	ensureThreadView(context, sessionId, threadId).composer.nextServiceTier =
		serviceTier;
}

export async function clearThreadNextComposerValues(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	const composer = ensureThreadView(context, sessionId, threadId).composer;
	composer.nextModelId = undefined;
	composer.nextReasoning = undefined;
	composer.nextServiceTier = undefined;
}

export async function removeThreadPendingComment(
	context: Context,
	sessionId: string,
	threadId: string,
	commentId: string,
): Promise<void> {
	const composer = ensureThreadView(context, sessionId, threadId).composer;
	composer.pendingComments = composer.pendingComments.filter(
		(comment) => comment.id !== commentId,
	);
	writeConversationComments(
		resolveCommentsStorageKey(context, sessionId, threadId),
		composer.pendingComments,
	);
}

export async function clearThreadPendingComments(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	ensureThreadView(context, sessionId, threadId).composer.pendingComments = [];
	clearConversationComments(
		resolveCommentsStorageKey(context, sessionId, threadId),
	);
}

export async function setThreadPendingComments(
	context: Context,
	sessionId: string,
	threadId: string,
	comments: ConversationComment[],
): Promise<void> {
	ensureThreadView(context, sessionId, threadId).composer.pendingComments =
		comments;
	writeConversationComments(
		resolveCommentsStorageKey(context, sessionId, threadId),
		comments,
	);
}

export async function addThreadPendingComment(
	context: Context,
	sessionId: string,
	threadId: string,
	comment: Omit<ConversationComment, "id">,
): Promise<void> {
	const trimmedSnippet = comment.snippet.trim();
	const trimmedComment = comment.comment.trim();
	if (!trimmedSnippet || !trimmedComment) {
		return;
	}

	const composer = ensureThreadView(context, sessionId, threadId).composer;
	composer.pendingComments = [
		...composer.pendingComments,
		{
			id: crypto.randomUUID(),
			snippet: trimmedSnippet,
			comment: trimmedComment,
		},
	];
	writeConversationComments(
		resolveCommentsStorageKey(context, sessionId, threadId),
		composer.pendingComments,
	);
}

export async function setConversationScrollTop(
	context: Context,
	sessionId: string,
	threadId: string,
	scrollTop: number,
): Promise<void> {
	ensureThreadView(context, sessionId, threadId).conversation.scrollTop =
		scrollTop;
}

export async function setConversationStickToBottom(
	context: Context,
	sessionId: string,
	threadId: string,
	stickToBottom: boolean,
): Promise<void> {
	ensureThreadView(context, sessionId, threadId).conversation.stickToBottom =
		stickToBottom;
}

export async function submitThread(
	context: Context,
	sessionId: string,
	threadId: string,
	payload: SubmitThreadPayload,
): Promise<SubmitThreadResult | undefined> {
	const hasMessageContent = hasUserMessageContent(payload.parts);
	const hasSession = !isPendingSession(context, sessionId);
	if (
		!hasMessageContent &&
		!(payload.allowEmptyPendingMessage && !hasSession)
	) {
		return;
	}

	const content = ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	);
	content.error = null;
	const composerValues = getCurrentComposerValues(context, sessionId, threadId);
	const composer = ensureThreadView(context, sessionId, threadId).composer;
	const submitValues = resolveThreadComposerSubmitValues({
		modelId: composerValues.modelId,
		reasoning: composerValues.reasoning,
		serviceTier: composerValues.serviceTier,
		nextModelId: composer.nextModelId,
		nextReasoning: composer.nextReasoning,
		nextServiceTier: composer.nextServiceTier,
	});
	const nextModel = normalizeModelId(submitValues.modelId) ?? "";
	const userMessage = hasMessageContent
		? createUserMessageFromParts(payload.parts, { provisional: true })
		: null;
	const shouldOptimisticallyInsert =
		hasSession && userMessage && !content.isStreaming && !payload.runAfter;

	if (shouldOptimisticallyInsert) {
		setThreadMessages(context, sessionId, threadId, [
			...getThreadMessages(context, sessionId, threadId),
			userMessage,
		]);
	}

	try {
		if (!payload.runAfter) {
			content.isStreaming = true;
			content.pendingQuestionId = null;
			content.answeredApprovalIds = {};
		}
		const response = await api.startChat({
			sessionId,
			threadId,
			messages: userMessage ? getSubmitMessages(userMessage) : [],
			...(payload.workspaceId ? { workspaceId: payload.workspaceId } : {}),
			...(payload.providerId ? { providerId: payload.providerId } : {}),
			...(payload.workspaceType && payload.workspacePath
				? {
						workspaceType: payload.workspaceType,
						workspacePath: payload.workspacePath,
					}
				: {}),
			model: nextModel,
			reasoning: submitValues.reasoning ?? "",
			serviceTier: submitValues.serviceTier ?? "",
			...(payload.runAfter ? { runAfter: payload.runAfter } : {}),
		});
		const result = {
			sessionId: response.sessionId,
			threadId: response.threadId,
			queued: response.status === "queued",
		};
		if (response.status === "queued" && userMessage) {
			setThreadMessages(
				context,
				sessionId,
				threadId,
				removeProvisionalSubmitMessage(
					getThreadMessages(context, sessionId, threadId),
					userMessage.id,
				),
			);
			await loadThreadIntoCache(context, result.sessionId, result.threadId);
		}
		composer.nextModelId = undefined;
		composer.nextReasoning = undefined;
		composer.nextServiceTier = undefined;
		if (!hasSession) {
			await context.commands.sessions
				.refreshSession(result.sessionId)
				.catch(() => {});
		}
		if (!payload.runAfter) {
			await context.commands.threads.activateThread(
				result.sessionId,
				result.threadId,
			);
		}
		return result;
	} catch (error) {
		content.isStreaming = false;
		if (userMessage) {
			setThreadMessages(
				context,
				sessionId,
				threadId,
				removeProvisionalSubmitMessage(
					getThreadMessages(context, sessionId, threadId),
					userMessage.id,
				),
			);
		}
		content.pendingQuestionId = getPendingQuestionApprovalId(
			content.messages,
			content.answeredApprovalIds,
		);
		content.error =
			error instanceof Error ? error.message : "Failed to start chat";
		content.status = createErrorStatus(content.error);
		throw error;
	}
}

export async function refreshThread(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	await loadThreadIntoCache(context, sessionId, threadId);
}

export async function cancelThread(
	context: Context,
	sessionId: string,
	threadId: string,
): Promise<void> {
	await api.cancelThreadChat(sessionId, threadId);
	const content = ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	);
	content.isStreaming = false;
	content.status = createReadyStatus();
	await loadThreadIntoCache(context, sessionId, threadId);
}

export async function addToolApprovalResponse(
	context: Context,
	sessionId: string,
	threadId: string,
	payload: { id: string; approved: boolean; reason?: string },
): Promise<void> {
	const content = ensureThreadContentState(
		ensureSessionRecord(context.data.sessions, sessionId).threads,
		threadId,
	);
	content.answeredApprovalIds[payload.id] = {
		approved: payload.approved,
		...(payload.reason ? { reason: payload.reason } : {}),
	};
	content.pendingQuestionId = getPendingQuestionApprovalId(
		content.messages,
		content.answeredApprovalIds,
	);
}

export async function deleteQueuedPrompt(
	context: Context,
	sessionId: string,
	threadId: string,
	queueId: string,
): Promise<void> {
	await api.deleteQueuedPrompt(sessionId, threadId, queueId);
	await loadThreadIntoCache(context, sessionId, threadId);
}

export async function updateQueuedPrompt(
	context: Context,
	sessionId: string,
	threadId: string,
	queueId: string,
	payload: UpdateQueuedPromptRequest,
): Promise<void> {
	await api.updateQueuedPrompt(sessionId, threadId, queueId, payload);
	await loadThreadIntoCache(context, sessionId, threadId);
}
