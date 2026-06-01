import { getContext, hasContext, setContext } from "svelte";

import { api } from "$lib/api-client";
import { isThreadSnapshotRunning } from "$lib/app/thread-status";
import {
	canLoadSessionThreads,
	isSessionTransitioningStatus,
	SessionStatus,
} from "$lib/api-constants";
import type {
	SessionThreadStatus,
	Thread,
	ThreadActivityStatus,
} from "$lib/api-types";
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

const THREAD_CONTEXT_KEY = Symbol.for("discobot-ui-thread-context");
const COMPOSER_DRAFT_PERSIST_DELAY_MS = 300;
const PENDING_COMMENTS_PERSIST_DELAY_MS = 300;

export function getThreadIsStreaming(
	thread: Thread | null,
	isStreaming: boolean,
): boolean {
	return isStreaming || isThreadSnapshotRunning(thread);
}

export function isSessionThreadStatusRunningForThread(
	status: SessionThreadStatus | null | undefined,
	threadId: string,
): boolean {
	return status?.status === "running" && status.threadId === threadId;
}

export function getThreadConversationStatus(
	status: ThreadContextValue["status"],
): ThreadContextValue["status"] {
	return status;
}

export function normalizeThreadComposerReasoning(
	reasoning: string | null | undefined,
): string | undefined {
	return reasoning && reasoning.length > 0 ? reasoning : undefined;
}

export function normalizeThreadComposerServiceTier(
	serviceTier: string | null | undefined,
): string | undefined {
	const normalized = serviceTier?.trim().toLowerCase();
	if (!normalized) {
		return undefined;
	}
	return normalized === "fast" ? "priority" : normalized;
}

export function parseComposerModelSelection(
	modelId: string | null | undefined,
): { modelId: string | null } {
	return {
		modelId: modelId && modelId.length > 0 ? modelId : null,
	};
}

export function getThreadComposerValues(
	thread: Thread | null,
	defaultModel: string | null,
): {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
} {
	return {
		modelId: thread?.model ?? defaultModel,
		reasoning: normalizeThreadComposerReasoning(thread?.reasoning),
		serviceTier: normalizeThreadComposerServiceTier(thread?.serviceTier),
	};
}

export function getThreadComposerValuesKey(values: {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
}): string {
	return JSON.stringify(values);
}

export function resolveThreadComposerSubmitValues({
	modelId,
	reasoning,
	serviceTier,
	nextModelId,
	nextReasoning,
	nextServiceTier,
}: {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
	nextModelId: string | null | undefined;
	nextReasoning: string | undefined;
	nextServiceTier: string | null | undefined;
}): {
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
} {
	const resolvedModelId = nextModelId !== undefined ? nextModelId : modelId;
	return {
		modelId: resolvedModelId,
		reasoning: resolvedModelId
			? normalizeThreadComposerReasoning(nextReasoning ?? reasoning)
			: undefined,
		serviceTier: resolvedModelId
			? normalizeThreadComposerServiceTier(
					nextServiceTier !== undefined ? nextServiceTier : serviceTier,
				)
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
	void sessionName;
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
		canLoadSessionThreads(session.current?.sandboxStatus),
	);
	const retryScheduler = createRetryScheduler({
		owner: "ThreadContext",
		enabled: () => hasSession,
		retry: { mode: "background" },
	});
	const shouldIgnoreClosedStreamError = () => {
		switch (session.current?.sandboxStatus) {
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

	function getThread() {
		return (
			session.threads.list.find((thread) => thread.id === threadId) ?? null
		);
	}

	const sourceComposerValues = $derived.by(() =>
		getThreadComposerValues(getThread(), app.preferences.defaultModel || null),
	);
	const initialComposerValues = getThreadComposerValues(
		getThread(),
		app.preferences.defaultModel || null,
	);
	let sourceComposerValuesKey = $state(
		getThreadComposerValuesKey(initialComposerValues),
	);
	let modelId = $state<string | null>(initialComposerValues.modelId);
	let reasoning = $state<string | undefined>(initialComposerValues.reasoning);
	let serviceTier = $state<string | undefined>(
		initialComposerValues.serviceTier,
	);
	let nextModelId = $state<string | null | undefined>(undefined);
	let nextReasoning = $state<string | undefined>(undefined);
	let nextServiceTier = $state<string | null | undefined>(undefined);
	let loadedComposerDraftStorageKey = $state<string | null>(null);
	let lastStoredComposerDraft = $state("");
	let pendingComments = $state<ConversationComment[]>([]);
	let loadedPendingCommentsStorageKey = $state<string | null>(null);
	let lastStoredPendingComments = $state("");
	let composerDraftPersistTimer: ReturnType<typeof setTimeout> | null = null;
	let pendingCommentsPersistTimer: ReturnType<typeof setTimeout> | null = null;
	let disposed = false;

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
		serviceTier = nextSourceComposerValues.serviceTier;
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
		nextServiceTier = undefined;
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
			serviceTier,
			nextModelId,
			nextReasoning,
			nextServiceTier,
		});
		const result = await conversation.submit({
			parts,
			modelId: submitValues.modelId,
			reasoning: submitValues.reasoning,
			serviceTier: submitValues.serviceTier,
			workspaceId,
			providerId,
			workspaceType,
			workspacePath,
			allowEmptyPendingMessage,
			runAfter,
		});
		if (result) {
			const submittedThread = session.threads.list.find(
				(thread) => thread.id === result.threadId,
			);
			app.stores.recentThreads.recordSelection({
				sessionId: result.sessionId,
				threadId: result.threadId,
				name:
					submittedThread?.name ||
					session.current?.displayName ||
					session.current?.name ||
					"New Thread",
			});
		}
		modelId = submitValues.modelId;
		reasoning = submitValues.reasoning;
		serviceTier = submitValues.serviceTier;
		clearNextComposerValues();
		return result;
	};

	return {
		get disposed() {
			return disposed;
		},
		get threadId() {
			return threadId;
		},
		get thread() {
			return getThread();
		},
		get modelId() {
			return modelId;
		},
		get reasoning() {
			return reasoning;
		},
		get serviceTier() {
			return serviceTier;
		},
		get nextModelId() {
			return nextModelId;
		},
		get nextReasoning() {
			return nextReasoning;
		},
		get nextServiceTier() {
			return nextServiceTier;
		},
		setNextModelId: (value) => {
			nextModelId = value;
		},
		setNextReasoning: (value) => {
			nextReasoning = value;
		},
		setNextServiceTier: (value) => {
			nextServiceTier = value;
		},
		clearNextComposerValues,
		get messages() {
			return conversation.messages;
		},
		get browserEventsByTurnId() {
			return conversation.browserEventsByTurnId;
		},
		get promptQueue() {
			return getThread()?.promptQueue ?? [];
		},
		get status() {
			return getThreadConversationStatus(conversation.status);
		},
		get isStreaming() {
			return (
				getThreadIsStreaming(getThread(), conversation.isStreaming) ||
				isSessionThreadStatusRunningForThread(
					session.current?.threadStatus,
					threadId,
				) ||
				(!hasSession &&
					(isSessionTransitioningStatus(session.current?.sandboxStatus) ||
						conversation.messages.some(
							(message) => message.provisional === true,
						)))
			);
		},
		get error() {
			return conversation.error ?? getThread()?.errorMessage ?? null;
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
			disposed = true;
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
