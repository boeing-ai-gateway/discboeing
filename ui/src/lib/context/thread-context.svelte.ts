import { getContext, hasContext, setContext } from "svelte";

import { api } from "$lib/api-client";
import { canLoadSessionThreads, SessionStatus } from "$lib/api-constants";
import type { Thread, ThreadActivityStatus } from "$lib/api-types";
import {
	clearComposerDraft,
	readComposerDraft,
	resolveComposerDraftStorageKey,
	writeComposerDraft,
} from "$lib/composer-draft-storage";
import {
	clearConversationComments,
	readConversationComments,
	resolveConversationCommentsStorageKey,
	serializeConversationComments,
	writeConversationComments,
} from "$lib/conversation-comment-storage";
import type { AppContext, StartChat } from "$lib/context/app-context.svelte";
import { createConversationDomain } from "$lib/thread/conversation.svelte";
import { createRetryScheduler } from "$lib/resource/create-resource.svelte";
import type {
	ConversationComment,
	SessionContextValue,
	ThreadContextValue,
} from "$lib/session/session-context.types";
import type { ThreadSummary } from "$lib/shell-types";

const THREAD_CONTEXT_KEY = Symbol.for("discobot-ui-thread-context");
const COMPOSER_DRAFT_PERSIST_DELAY_MS = 300;
const PENDING_COMMENTS_PERSIST_DELAY_MS = 300;

export function normalizeThreadComposerReasoning(
	reasoning: string | null | undefined,
): string | undefined {
	return reasoning && reasoning.length > 0 ? reasoning : undefined;
}

export function parseComposerModelSelection(
	modelId: string | null | undefined,
): { modelId: string | null } {
	return {
		modelId: modelId && modelId.length > 0 ? modelId : null,
	};
}

export function getThreadComposerValues(
	thread: ThreadSummary | null,
	defaultModel: string | null,
): {
	modelId: string | null;
	reasoning: string | undefined;
} {
	return {
		modelId: thread?.model ?? defaultModel,
		reasoning: normalizeThreadComposerReasoning(thread?.reasoning),
	};
}

export function getThreadComposerValuesKey(values: {
	modelId: string | null;
	reasoning: string | undefined;
}): string {
	return JSON.stringify(values);
}

export function resolveThreadComposerSubmitValues({
	modelId,
	reasoning,
	nextModelId,
	nextReasoning,
}: {
	modelId: string | null;
	reasoning: string | undefined;
	nextModelId: string | null | undefined;
	nextReasoning: string | undefined;
}): {
	modelId: string | null;
	reasoning: string | undefined;
} {
	const resolvedModelId = nextModelId !== undefined ? nextModelId : modelId;
	return {
		modelId: resolvedModelId,
		reasoning: resolvedModelId
			? normalizeThreadComposerReasoning(nextReasoning ?? reasoning)
			: undefined,
	};
}

export function clearComposerDraftState({
	cancelPersist,
	clearStoredDraft,
	clearInMemoryDraft,
}: {
	cancelPersist: () => void;
	clearStoredDraft: () => void;
	clearInMemoryDraft: () => void;
}): void {
	cancelPersist();
	clearStoredDraft();
	clearInMemoryDraft();
}

export function applyStreamedThreadUpdate({
	sessionId,
	sessionName,
	sessionDisplayName,
	previousThreadName,
	thread,
	upsertThread,
	syncSessionName,
	reloadSession,
}: {
	sessionId: string;
	sessionName: string;
	sessionDisplayName?: string | null;
	previousThreadName?: string | null;
	thread: Thread;
	upsertThread: (thread: Thread) => void;
	syncSessionName: (name: string) => void;
	reloadSession: () => void | Promise<void>;
}): void {
	upsertThread(thread);

	if (
		thread.id !== sessionId ||
		sessionDisplayName ||
		previousThreadName === thread.name ||
		thread.name.trim().length === 0
	) {
		return;
	}

	syncSessionName(thread.name);
	void reloadSession();
}

export function createThreadContext(
	app: AppContext,
	startChat: StartChat,
	session: SessionContextValue,
	threadId: string,
): ThreadContextValue {
	const hasSession = $derived.by(() =>
		canLoadSessionThreads(session.current?.status),
	);
	const retryScheduler = createRetryScheduler({
		owner: "ThreadContext",
		enabled: () => hasSession,
		retry: { mode: "background" },
	});
	const shouldIgnoreClosedStreamError = () => {
		switch (session.current?.status) {
			case SessionStatus.INITIALIZING:
			case SessionStatus.REINITIALIZING:
			case SessionStatus.CLONING:
			case SessionStatus.PULLING_IMAGE:
			case SessionStatus.CREATING_SANDBOX:
			case SessionStatus.REMOVING:
				return true;
			default:
				return false;
		}
	};
	const refreshSessionState = async () => {
		if (!hasSession) {
			return;
		}
		session.services.invalidate();
		session.hooks.invalidate();
		await Promise.all([
			retryScheduler.run("files", () => session.files.refresh()),
			retryScheduler.run("session", () =>
				app.sessions.reloadSession(session.sessionId),
			),
		]);
	};
	const applyLocalActivityStatus = (
		activityStatus: ThreadActivityStatus | null,
	) => {
		const currentThread = session.stores.threads.get(threadId);
		if (!currentThread) {
			return;
		}
		session.stores.threads.upsert({
			...currentThread,
			activityStatus: activityStatus ?? undefined,
		});
	};

	const conversation = createConversationDomain({
		sessionId: session.sessionId,
		hasSession: () => hasSession,
		threadId,
		startChat,
		chatStreams: app.chatStreams,
		initialMessages: app.sessions.takeOptimisticMessages(
			session.sessionId,
			threadId,
		),
		refreshThread: async () => {
			await session.threads.refreshThread(threadId);
			const refreshedThread = session.stores.threads.get(threadId);
			if (refreshedThread) {
				conversation.reconcileThreadSnapshot(refreshedThread);
			}
		},
		applyThreadUpdate: (thread) => {
			const previousThread = session.stores.threads.get(thread.id);
			applyStreamedThreadUpdate({
				sessionId: session.sessionId,
				sessionName:
					session.current?.displayName ||
					session.current?.name ||
					"New Session",
				sessionDisplayName: session.current?.displayName ?? null,
				previousThreadName: previousThread?.name ?? null,
				thread,
				upsertThread: (nextThread) => {
					session.stores.threads.upsert(nextThread);
				},
				syncSessionName: (name) => {
					const currentSession = session.current;
					if (!currentSession || currentSession.displayName) {
						return;
					}
					app.stores.sessions.upsert({
						...currentSession,
						name,
					});
				},
				reloadSession: () => app.sessions.reloadSession(session.sessionId),
			});
			if (thread.id === threadId) {
				conversation.reconcileThreadSnapshot(thread);
			}
		},
		applyHooksStatusUpdate: (status) => {
			return session.hooks.applyStatusUpdate(status);
		},
		refreshSessionState,
		shouldIgnoreClosedStreamError,
		onActivityStatusChange: applyLocalActivityStatus,
		afterTurn: async () => {
			if (!hasSession) {
				return;
			}
			await session.threads.refreshThread(threadId);
			await refreshSessionState();
		},
	});

	$effect(() => {
		if (hasSession) {
			return;
		}
		retryScheduler.dispose();
		conversation.disconnect();
	});

	const threadSummary = $derived.by(
		() => session.threads.list.find((t) => t.id === threadId) ?? null,
	);
	const sourceComposerValues = $derived.by(() =>
		getThreadComposerValues(
			threadSummary,
			app.preferences.defaultModel || null,
		),
	);
	const initialComposerValues = getThreadComposerValues(
		session.threads.list.find((t) => t.id === threadId) ?? null,
		app.preferences.defaultModel || null,
	);
	let sourceComposerValuesKey = $state(
		getThreadComposerValuesKey(initialComposerValues),
	);
	let modelId = $state<string | null>(initialComposerValues.modelId);
	let reasoning = $state<string | undefined>(initialComposerValues.reasoning);
	let nextModelId = $state<string | null | undefined>(undefined);
	let nextReasoning = $state<string | undefined>(undefined);
	let loadedComposerDraftStorageKey = $state<string | null>(null);
	let lastStoredComposerDraft = $state("");
	let pendingComments = $state<ConversationComment[]>([]);
	let loadedPendingCommentsStorageKey = $state<string | null>(null);
	let lastStoredPendingComments = $state("");
	let composerDraftPersistTimer: ReturnType<typeof setTimeout> | null = null;
	let pendingCommentsPersistTimer: ReturnType<typeof setTimeout> | null = null;

	$effect(() => {
		const nextSourceComposerValues = sourceComposerValues;
		const nextSourceComposerValuesKey = getThreadComposerValuesKey(
			nextSourceComposerValues,
		);
		if (nextSourceComposerValuesKey === sourceComposerValuesKey) {
			return;
		}
		sourceComposerValuesKey = nextSourceComposerValuesKey;
		modelId = nextSourceComposerValues.modelId;
		reasoning = nextSourceComposerValues.reasoning;
	});

	const composerDraftStorageKey = $derived.by(() =>
		resolveComposerDraftStorageKey({
			isPending: session.isPending,
			threadId,
			sessionId: session.sessionId,
		}),
	);
	const pendingCommentsStorageKey = $derived.by(() =>
		resolveConversationCommentsStorageKey({
			isPending: session.isPending,
			threadId,
			sessionId: session.sessionId,
		}),
	);

	$effect(() => {
		const storageKey = composerDraftStorageKey;
		if (loadedComposerDraftStorageKey !== storageKey) {
			loadedComposerDraftStorageKey = storageKey;
			lastStoredComposerDraft = readComposerDraft(storageKey);
			if (session.ui.composerDraft !== lastStoredComposerDraft) {
				session.ui.setComposerDraft(lastStoredComposerDraft);
			}
			return;
		}

		const draft = session.ui.composerDraft;
		if (draft === lastStoredComposerDraft) {
			return;
		}

		if (composerDraftPersistTimer !== null) {
			clearTimeout(composerDraftPersistTimer);
		}

		composerDraftPersistTimer = setTimeout(() => {
			writeComposerDraft(storageKey, draft);
			lastStoredComposerDraft = draft;
			composerDraftPersistTimer = null;
		}, COMPOSER_DRAFT_PERSIST_DELAY_MS);

		return () => {
			if (composerDraftPersistTimer !== null) {
				clearTimeout(composerDraftPersistTimer);
				composerDraftPersistTimer = null;
			}
		};
	});

	const clearStoredComposerDraft = (storageKey = composerDraftStorageKey) => {
		clearComposerDraftState({
			cancelPersist: () => {
				if (composerDraftPersistTimer !== null) {
					clearTimeout(composerDraftPersistTimer);
					composerDraftPersistTimer = null;
				}
			},
			clearStoredDraft: () => {
				clearComposerDraft(storageKey);
				if (storageKey === composerDraftStorageKey) {
					lastStoredComposerDraft = "";
				}
			},
			clearInMemoryDraft: () => {
				session.ui.setComposerDraft("");
			},
		});
	};

	$effect(() => {
		const storageKey = pendingCommentsStorageKey;
		if (loadedPendingCommentsStorageKey !== storageKey) {
			loadedPendingCommentsStorageKey = storageKey;
			pendingComments = readConversationComments(storageKey);
			lastStoredPendingComments =
				serializeConversationComments(pendingComments);
			return;
		}

		const serializedComments = serializeConversationComments(pendingComments);
		if (serializedComments === lastStoredPendingComments) {
			return;
		}

		if (pendingCommentsPersistTimer !== null) {
			clearTimeout(pendingCommentsPersistTimer);
		}

		pendingCommentsPersistTimer = setTimeout(() => {
			writeConversationComments(storageKey, pendingComments);
			lastStoredPendingComments = serializedComments;
			pendingCommentsPersistTimer = null;
		}, PENDING_COMMENTS_PERSIST_DELAY_MS);

		return () => {
			if (pendingCommentsPersistTimer !== null) {
				clearTimeout(pendingCommentsPersistTimer);
				pendingCommentsPersistTimer = null;
			}
		};
	});

	const clearStoredPendingComments = (
		storageKey = pendingCommentsStorageKey,
	) => {
		if (pendingCommentsPersistTimer !== null) {
			clearTimeout(pendingCommentsPersistTimer);
			pendingCommentsPersistTimer = null;
		}
		clearConversationComments(storageKey);
		if (storageKey === pendingCommentsStorageKey) {
			lastStoredPendingComments = "[]";
			pendingComments = [];
		}
	};

	const clearNextComposerValues = () => {
		nextModelId = undefined;
		nextReasoning = undefined;
	};

	const submit: ThreadContextValue["submit"] = async ({
		parts,
		workspaceId,
		providerId,
		workspaceType,
		workspacePath,
		allowEmptyPendingMessage,
		runAfter,
	}) => {
		const submitValues = resolveThreadComposerSubmitValues({
			modelId,
			reasoning,
			nextModelId,
			nextReasoning,
		});
		const result = await conversation.submit({
			parts,
			modelId: submitValues.modelId,
			reasoning: submitValues.reasoning,
			workspaceId,
			providerId,
			workspaceType,
			workspacePath,
			allowEmptyPendingMessage,
			runAfter,
		});
		modelId = submitValues.modelId;
		reasoning = submitValues.reasoning;
		clearNextComposerValues();
		return result;
	};

	return {
		get threadId() {
			return threadId;
		},
		get thread() {
			return threadSummary;
		},
		get modelId() {
			return modelId;
		},
		get reasoning() {
			return reasoning;
		},
		get nextModelId() {
			return nextModelId;
		},
		get nextReasoning() {
			return nextReasoning;
		},
		setNextModelId: (value) => {
			nextModelId = value;
		},
		setNextReasoning: (value) => {
			nextReasoning = value;
		},
		clearNextComposerValues,
		get messages() {
			return conversation.messages;
		},
		get browserEventsByTurnId() {
			return conversation.browserEventsByTurnId;
		},
		get promptQueue() {
			return threadSummary?.promptQueue ?? [];
		},
		get status() {
			return conversation.status;
		},
		get error() {
			return conversation.error ?? threadSummary?.errorMessage ?? null;
		},
		get hasPendingQuestion() {
			return conversation.hasPendingQuestion;
		},
		get pendingQuestionId() {
			return conversation.pendingQuestionId;
		},
		get pendingComments() {
			return pendingComments;
		},
		addPendingComment: (comment) => {
			const trimmedSnippet = comment.snippet.trim();
			const trimmedComment = comment.comment.trim();
			if (!trimmedSnippet || !trimmedComment) {
				return;
			}
			pendingComments = [
				...pendingComments,
				{
					id: crypto.randomUUID(),
					snippet: trimmedSnippet,
					comment: trimmedComment,
				},
			];
		},
		removePendingComment: (id) => {
			pendingComments = pendingComments.filter((comment) => comment.id !== id);
		},
		clearPendingComments: () => {
			clearStoredPendingComments();
		},
		clearComposerDraft: clearStoredComposerDraft,
		submit,
		cancel: conversation.cancel,
		connect: conversation.connect,
		disconnect: conversation.disconnect,
		refresh: conversation.refresh,
		addToolApprovalResponse: conversation.addToolApprovalResponse,
		deleteQueuedPrompt: async (queueId) => {
			await api.deleteQueuedPrompt(session.sessionId, threadId, queueId);
			await session.threads.refreshThread(threadId);
		},
		updateQueuedPrompt: async (queueId, payload) => {
			await api.updateQueuedPrompt(
				session.sessionId,
				threadId,
				queueId,
				payload,
			);
			await session.threads.refreshThread(threadId);
		},
		dispose: () => {
			retryScheduler.dispose();
			if (composerDraftPersistTimer !== null) {
				clearTimeout(composerDraftPersistTimer);
				composerDraftPersistTimer = null;
			}
			if (pendingCommentsPersistTimer !== null) {
				clearTimeout(pendingCommentsPersistTimer);
				pendingCommentsPersistTimer = null;
			}
			conversation.dispose();
		},
		get editorFiles() {
			return session.files.list;
		},
		get fileContents() {
			return session.files.contents;
		},
	};
}

export function setThreadContext(
	context: ThreadContextValue,
): ThreadContextValue {
	setContext(THREAD_CONTEXT_KEY, context);
	return context;
}

export function useThreadContext(): ThreadContextValue {
	const context = getContext<ThreadContextValue | undefined>(
		THREAD_CONTEXT_KEY,
	);
	if (!context) {
		throw new Error(
			"useThreadContext must be used within a ThreadContext provider",
		);
	}
	return context;
}

export function getThreadContextIfPresent(): ThreadContextValue | undefined {
	if (!hasContext(THREAD_CONTEXT_KEY)) {
		return undefined;
	}
	return getContext<ThreadContextValue | undefined>(THREAD_CONTEXT_KEY);
}
