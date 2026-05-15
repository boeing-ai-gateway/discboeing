import { getContext, hasContext, setContext } from "svelte";

import { api } from "$lib/api-client";
import { isThreadSnapshotRunning } from "$lib/app/thread-status";
import type { Thread } from "$lib/api-types";
import { isSessionTransitioningStatus } from "$lib/session/session-status";
import type { AppContext, StartChat } from "$lib/context/app-context.svelte";
import { createConversationDomain } from "$lib/conversation/conversation.svelte";
import { addToolApprovalResponse } from "$lib/session/domains/session-domain.helpers";
import { createThreadComposerState } from "$lib/thread/domains/thread-composer.svelte";
import { createThreadPendingQuestionState } from "$lib/thread/domains/thread-pending-question.svelte";
import type {
	SessionContextValue,
	ThreadContextValue,
} from "$lib/session/session-context.types";

const THREAD_CONTEXT_KEY = Symbol.for("discobot-ui-thread-context");

export function getThreadIsStreaming(
	thread: Thread | null,
	isStreaming: boolean,
): boolean {
	return isStreaming || isThreadSnapshotRunning(thread);
}

export function applyStreamedThreadUpdate({
	sessionId,
	sessionDisplayName,
	previousThreadName,
	thread,
	upsertThread,
	syncSessionName,
	reloadSession,
}: {
	sessionId: string;
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
	const refreshSessionState = async () => {
		session.services.invalidate();
		session.hooks.invalidate();
		await Promise.all([
			session.files.refresh().catch(() => {}),
			app.sessions.reloadSession(session.sessionId),
		]);
	};
	const createConversation = () =>
		createConversationDomain({
			sessionId: session.sessionId,
			threadId,
			startChat,
			projectStreams: app.projectStreams,
			refreshThread: async () => {
				await session.threads.refreshThread(threadId);
			},
			applyThreadUpdate: (thread) => {
				const previousThread = session.threads.get(thread.id);
				applyStreamedThreadUpdate({
					sessionId: session.sessionId,
					sessionDisplayName: session.current?.displayName ?? null,
					previousThreadName: previousThread?.name ?? null,
					thread,
					upsertThread: (nextThread) => {
						session.threads.upsert(nextThread);
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
			},
			applyHooksStatusUpdate: (status) => {
				return session.hooks.applyStatusUpdate(status);
			},
			refreshSessionState,
			afterTurn: async () => {
				await session.threads.refreshThread(threadId);
				await refreshSessionState();
			},
		});
	let conversation = $state(createConversation());
	let conversationStarted = $state(false);

	function getThread() {
		return session.threads.get(threadId);
	}

	const composer = createThreadComposerState({
		sessionId: session.sessionId,
		threadId,
		isPending: () => session.isPending,
		getThread,
		getDefaultModel: () => app.preferences.defaultModel || null,
		getDraft: () => session.ui.composerDraft,
		setDraft: session.ui.setComposerDraft,
	});
	const pendingQuestion = createThreadPendingQuestionState({
		sessionId: session.sessionId,
		threadId,
		getMessages: () => conversation.messages,
		getThreadPendingQuestion: () => Boolean(getThread()?.pendingQuestion),
	});

	const start = () => {
		conversationStarted = true;
		return conversation.connect();
	};

	const refreshAfterSubmitError = () => {
		conversation.dispose();
		conversation = createConversation();
		if (!conversationStarted) {
			return Promise.resolve();
		}
		return conversation.connect();
	};

	const submit: ThreadContextValue["submit"] = async ({
		parts,
		workspaceId,
		providerId,
		workspaceType,
		workspacePath,
		runAfter,
	}) => {
		const submitValues = composer.resolveSubmitValues();
		pendingQuestion.clearSubmitError();
		let result: Awaited<ReturnType<ThreadContextValue["submit"]>>;
		try {
			result = await conversation.submit({
				parts,
				modelId: submitValues.modelId,
				reasoning: submitValues.reasoning,
				serviceTier: submitValues.serviceTier,
				workspaceId,
				providerId,
				workspaceType,
				workspacePath,
				runAfter,
			});
		} catch (error) {
			pendingQuestion.applySubmitError(error);
			void refreshAfterSubmitError();
			throw error;
		}
		if (result) {
			const submittedThread =
				result.threadId === threadId
					? getThread()
					: session.threads.get(result.threadId);
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
		composer.applySubmitValues(submitValues);
		return result;
	};

	return {
		get threadId() {
			return threadId;
		},
		get thread() {
			return getThread();
		},
		get modelId() {
			return composer.modelId;
		},
		get reasoning() {
			return composer.reasoning;
		},
		get serviceTier() {
			return composer.serviceTier;
		},
		get nextModelId() {
			return composer.nextModelId;
		},
		get nextReasoning() {
			return composer.nextReasoning;
		},
		get nextServiceTier() {
			return composer.nextServiceTier;
		},
		setNextModelId: (value) => {
			composer.setNextModelId(value);
		},
		setNextReasoning: (value) => {
			composer.setNextReasoning(value);
		},
		setNextServiceTier: (value) => {
			composer.setNextServiceTier(value);
		},
		clearNextComposerValues: composer.clearNextValues,
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
			return conversation.status;
		},
		get isStreaming() {
			return (
				getThreadIsStreaming(getThread(), conversation.isStreaming) ||
				isSessionTransitioningStatus(session.current?.sandboxStatus) ||
				conversation.messages.some((message) => message.provisional === true)
			);
		},
		get error() {
			return conversation.error ?? getThread()?.errorMessage ?? null;
		},
		get hasPendingQuestion() {
			return pendingQuestion.hasPendingQuestion;
		},
		get pendingQuestionToolPart() {
			return pendingQuestion.pendingQuestionToolPart;
		},
		get pendingQuestionLoading() {
			return pendingQuestion.pendingQuestionLoading;
		},
		get pendingQuestionError() {
			return pendingQuestion.pendingQuestionError;
		},
		clearComposerDraft: composer.clearDraft,
		submit,
		cancel: () => conversation.cancel(),
		start,
		refresh: refreshAfterSubmitError,
		addToolApprovalResponse: (payload) => {
			addToolApprovalResponse(conversation.messages, payload);
			pendingQuestion.clearSubmitError();
		},
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
			composer.dispose();
			pendingQuestion.clearSubmitError();
			conversation.dispose();
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
