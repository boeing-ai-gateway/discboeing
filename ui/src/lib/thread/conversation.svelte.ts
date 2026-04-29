import { api } from "$lib/api-client";
import { StartChatError } from "$lib/api-client";
import { useAppContext } from "$lib/context/app-context.svelte";
import type { ChatMessage, HooksStatusResponse, Thread } from "$lib/api-types";
import {
	createChatStreamEventListeners,
	createChatStreamState,
} from "$lib/thread/conversation-stream";
import type { ChatStreamSubscription } from "$lib/thread/chat-stream-manager";
import {
	addToolApprovalResponse,
	createUserMessageFromParts,
	getPendingQuestionApprovalId,
	hasUserMessageContent,
} from "$lib/session/domains/session-domain.helpers";

const RETRY_TOAST_ID_PREFIX = "thread-retry-status:";
const LOST_PROJECT_STREAM_CONNECTION_MESSAGE = "Lost project stream connection";

type CreateConversationDomainArgs = {
	sessionId: string;
	hasSession: () => boolean;
	threadId: string;
	initialMessages?: ChatMessage[];
	refreshThread: () => Promise<void>;
	applyThreadUpdate?: (thread: Thread) => void;
	applyHooksStatusUpdate?: (
		status: HooksStatusResponse,
	) => void | Promise<void>;
	refreshSessionState?: () => Promise<void>;
	shouldIgnoreClosedStreamError?: () => boolean;
	afterTurn?: () => Promise<void>;
};

function normalizeModelId(modelId: string | null): string | undefined {
	if (!modelId) {
		return undefined;
	}
	return modelId.endsWith(":thinking")
		? modelId.slice(0, -":thinking".length)
		: modelId;
}

function getStreamErrorMessage(error: unknown): string {
	return error instanceof Error
		? error.message
		: "Failed to process chat stream";
}

function isLostProjectStreamConnection(error: unknown): boolean {
	return (
		getStreamErrorMessage(error) === LOST_PROJECT_STREAM_CONNECTION_MESSAGE
	);
}

async function showRetryToast(
	threadId: string,
	message: string,
): Promise<void> {
	const { toast } = await import("svelte-sonner");
	toast.message(message, {
		id: `${RETRY_TOAST_ID_PREFIX}${threadId}`,
	});
}

async function dismissRetryToast(threadId: string): Promise<void> {
	const { toast } = await import("svelte-sonner");
	toast.dismiss(`${RETRY_TOAST_ID_PREFIX}${threadId}`);
}

export function getSubmitMessages(userMessage: ChatMessage): ChatMessage[] {
	const { provisional: _provisional, ...submittedMessage } = userMessage;
	return [submittedMessage];
}

export function removeProvisionalSubmitMessage(
	messages: ChatMessage[],
	messageId: string,
): ChatMessage[] {
	return messages.filter((message) => message.id !== messageId);
}

export function getPendingQuestionState(
	messages: ChatMessage[],
	pendingQuestionId: string | null,
): { hasPendingQuestion: boolean; pendingQuestionId: string | null } {
	const messagePendingQuestionId = getPendingQuestionApprovalId(messages);
	const resolvedPendingQuestionId =
		messagePendingQuestionId ?? pendingQuestionId;
	return {
		hasPendingQuestion: resolvedPendingQuestionId !== null,
		pendingQuestionId: resolvedPendingQuestionId,
	};
}

export function getStartChatErrorDetails(error: unknown): {
	message: string | null;
	pendingQuestionId: string | null;
	completionId: string | null;
} {
	if (error instanceof StartChatError) {
		return {
			message: error.message,
			pendingQuestionId:
				error.code === "pending_question_requires_answer"
					? (error.questionId ?? null)
					: null,
			completionId: error.completionId ?? null,
		};
	}
	return {
		message: error instanceof Error ? error.message : "Failed to start chat",
		pendingQuestionId: null,
		completionId: null,
	};
}

export function createConversationDomain(args: CreateConversationDomainArgs) {
	const app = useAppContext();
	let messages = $state<ChatMessage[]>(args.initialMessages ?? []);
	let historyReplayVersion = $state(0);
	let streamError = $state<string | null>(null);
	let fatalStreamError = $state(false);
	let completionRunning = $state(false);
	let afterTurnPending = $state(false);
	let pendingQuestionId = $state<string | null>(null);
	let loadStatus = $state<"idle" | "loading" | "ready" | "error">("idle");
	let activeSubscription: ChatStreamSubscription | null = null;
	let activeStreamKey: string | null = null;
	let pendingLoadPromise: Promise<void> | null = null;
	let resolvePendingLoad: (() => void) | null = null;
	let rejectPendingLoad: ((error?: unknown) => void) | null = null;

	const status = $derived.by(() => {
		if (completionRunning) {
			return "streaming" as const;
		}
		if (loadStatus === "loading") {
			return "loading" as const;
		}
		if (!args.hasSession()) {
			return "idle" as const;
		}
		if (streamError || loadStatus === "error") {
			return "error" as const;
		}
		return "ready" as const;
	});
	const error = $derived.by(() => streamError);
	const pendingQuestionState = $derived.by(() =>
		getPendingQuestionState(messages, pendingQuestionId),
	);

	const handleCompletionStart = () => {
		if (completionRunning) {
			return;
		}
		streamError = null;
		afterTurnPending = true;
		completionRunning = true;
		pendingQuestionId = null;
	};

	const handleCompletionFinish = () => {
		if (!completionRunning) {
			return;
		}
		completionRunning = false;
		void dismissRetryToast(args.threadId);
		return runAfterTurnIfNeeded();
	};

	const streamState = createChatStreamState({
		getMessages: () => messages,
		setMessages: (nextMessages) => {
			messages = nextMessages;
		},
		onStart: () => {
			handleCompletionStart();
		},
		onCompletionStatus: ({ isRunning }) => {
			if (isRunning) {
				handleCompletionStart();
				return;
			}
			return handleCompletionFinish();
		},
		onFinish: () => {
			return handleCompletionFinish();
		},
		onHistoryReplayEnd: () => {
			fatalStreamError = false;
			historyReplayVersion += 1;
			pendingQuestionId = getPendingQuestionApprovalId(messages);
			if (loadStatus === "loading") {
				loadStatus = "ready";
			}
			resolvePendingLoad?.();
			pendingLoadPromise = null;
			resolvePendingLoad = null;
			rejectPendingLoad = null;
		},
		onChunkError: (errorText) => {
			fatalStreamError = true;
			completionRunning = false;
			streamError = errorText;
			void dismissRetryToast(args.threadId);
			if (loadStatus === "loading") {
				loadStatus = "error";
			}
			rejectPendingLoad?.(new Error(errorText));
			pendingLoadPromise = null;
			resolvePendingLoad = null;
			rejectPendingLoad = null;
		},
		onRetryStatus: (message) => {
			return showRetryToast(args.threadId, message);
		},
		onThreadUpdate: (thread) => {
			args.applyThreadUpdate?.(thread);
		},
		onHooksStatusUpdate: (status) => {
			return args.applyHooksStatusUpdate?.(status);
		},
	});

	function disconnectStream() {
		activeSubscription?.unsubscribe();
		activeSubscription = null;
		activeStreamKey = null;
	}

	function runAfterTurnIfNeeded() {
		if (!afterTurnPending) {
			return;
		}
		afterTurnPending = false;
		return args.afterTurn?.();
	}

	function resetLoadPromise() {
		pendingLoadPromise = null;
		resolvePendingLoad = null;
		rejectPendingLoad = null;
	}

	function beginLoadPromise() {
		if (pendingLoadPromise) {
			return pendingLoadPromise;
		}
		pendingLoadPromise = new Promise<void>((resolve, reject) => {
			resolvePendingLoad = resolve;
			rejectPendingLoad = reject;
		});
		return pendingLoadPromise;
	}

	function rejectLoad(error: unknown, fallbackMessage: string) {
		loadStatus = "error";
		streamError = error instanceof Error ? error.message : fallbackMessage;
		rejectPendingLoad?.(error);
		resetLoadPromise();
	}

	function streamKey(sessionId: string) {
		return `${sessionId}:${args.threadId}`;
	}

	function ensureStream(forceResubscribe = false) {
		if (typeof window === "undefined") {
			return;
		}
		if (fatalStreamError && !forceResubscribe) {
			console.warn(
				"[WS] Skipping chat stream subscribe because the stream is in a fatal state",
				{
					threadId: args.threadId,
					sessionId: args.sessionId,
					streamError,
				},
			);
			return;
		}
		if (activeStreamKey === streamKey(args.sessionId)) {
			if (forceResubscribe) {
				console.debug("[WS] Forcing chat stream resubscribe", {
					threadId: args.threadId,
					sessionId: args.sessionId,
					streamError,
					fatalStreamError,
				});
				fatalStreamError = false;
				activeSubscription?.resubscribe();
			}
			return;
		}

		disconnectStream();
		streamError = null;
		fatalStreamError = false;
		activeStreamKey = streamKey(args.sessionId);
		console.debug("[WS] Creating chat stream subscription", {
			threadId: args.threadId,
			sessionId: args.sessionId,
			forceResubscribe,
		});
		const listeners = createChatStreamEventListeners(streamState, {
			onError: (error) => {
				streamError = getStreamErrorMessage(error);
				fatalStreamError = true;
				disconnectStream();
				if (loadStatus === "loading") {
					rejectLoad(error, "Failed to load messages");
				}
			},
		});
		const subscription = app.chatStreams.subscribe({
			sessionId: args.sessionId,
			threadId: args.threadId,
			replay: true,
			listeners,
			onOpen: () => {
				console.debug("[WS] Chat stream opened", {
					threadId: args.threadId,
					sessionId: args.sessionId,
				});
				streamError = null;
				void Promise.all([
					args.refreshThread(),
					args.refreshSessionState?.(),
				]).catch((error) => {
					console.error(
						"Failed to refresh thread state after chat stream connected",
						error,
					);
				});
			},
			onError: (error) => {
				const errorMessage = getStreamErrorMessage(error);
				console.warn("[WS] Chat stream subscription error", {
					threadId: args.threadId,
					sessionId: args.sessionId,
					error: errorMessage,
					loadStatus,
					fatalStreamError,
				});
				if (isLostProjectStreamConnection(error)) {
					if (loadStatus !== "loading") {
						streamError = errorMessage;
					}
					return;
				}
				fatalStreamError = true;
				streamError = errorMessage;
				disconnectStream();
				if (loadStatus === "loading") {
					rejectLoad(error, "Failed to load messages");
				}
			},
		});
		activeSubscription = subscription;
	}

	async function loadFromStream(forceReconnect = false) {
		if (!forceReconnect && pendingLoadPromise) {
			return pendingLoadPromise;
		}
		if (fatalStreamError && !forceReconnect) {
			throw new Error(streamError ?? "Failed to process chat stream");
		}
		if (forceReconnect) {
			fatalStreamError = false;
		}
		if (forceReconnect || activeStreamKey === streamKey(args.sessionId)) {
			disconnectStream();
		}
		loadStatus = "loading";
		streamError = null;
		ensureStream();
		return beginLoadPromise();
	}

	function disconnect() {
		completionRunning = false;
		void dismissRetryToast(args.threadId);
		disconnectStream();
	}

	async function load() {
		if (!args.hasSession()) {
			loadStatus = "idle";
			streamError = null;
			completionRunning = false;
			pendingQuestionId = null;
			void dismissRetryToast(args.threadId);
			disconnectStream();
			resetLoadPromise();
			return;
		}
		if (loadStatus === "ready") {
			ensureStream();
			return;
		}
		await loadFromStream();
	}

	async function refresh() {
		if (!args.hasSession()) {
			loadStatus = "idle";
			streamError = null;
			completionRunning = false;
			pendingQuestionId = null;
			void dismissRetryToast(args.threadId);
			disconnectStream();
			resetLoadPromise();
			return;
		}
		await loadFromStream(true);
	}

	return {
		get messages() {
			return messages;
		},
		get historyReplayVersion() {
			return historyReplayVersion;
		},
		get status() {
			return status;
		},
		get error() {
			return error;
		},
		get hasPendingQuestion() {
			return pendingQuestionState.hasPendingQuestion;
		},
		get pendingQuestionId() {
			return pendingQuestionState.pendingQuestionId;
		},
		connect: load,
		disconnect,
		load,
		submit: async ({
			parts,
			mode,
			modelId,
			reasoning,
			workspaceId,
			workspaceType,
			workspacePath,
			allowEmptyPendingMessage,
		}: {
			parts: ChatMessage["parts"];
			mode: "build" | "plan";
			modelId: string | null;
			reasoning: string | undefined;
			workspaceId?: string;
			workspaceType?: "local" | "git" | null;
			workspacePath?: string | null;
			allowEmptyPendingMessage?: boolean;
		}) => {
			const hasMessageContent = hasUserMessageContent(parts);
			if (
				!hasMessageContent &&
				!(allowEmptyPendingMessage && !args.hasSession())
			) {
				return;
			}

			streamError = null;
			const nextModel = normalizeModelId(modelId ?? null) ?? "";
			const nextReasoning = reasoning ?? "";
			const nextMode = mode === "plan" ? "plan" : "build";
			const submittingWhileGenerating =
				args.hasSession() && (status === "streaming" || status === "loading");
			const userMessage = hasMessageContent
				? createUserMessageFromParts(parts, {
						provisional: true,
					})
				: null;

			if (userMessage && !submittingWhileGenerating) {
				messages = [...messages, userMessage];
			}

			try {
				if (!args.hasSession()) {
					const response = await app.chat({
						sessionId: args.sessionId,
						threadId: args.threadId,
						messages: userMessage ? getSubmitMessages(userMessage) : [],
						...(workspaceId ? { workspaceId } : {}),
						...(workspaceType && workspacePath
							? {
									workspaceType,
									workspacePath,
								}
							: {}),
						model: nextModel,
						reasoning: nextReasoning,
						mode: nextMode,
					});
					return {
						sessionId: response.sessionId,
						threadId: response.threadId,
						materialized: true,
						queued: response.status === "queued",
					};
				}

				console.debug("[WS] Preparing chat submit", {
					threadId: args.threadId,
					sessionId: args.sessionId,
					status,
					loadStatus,
					fatalStreamError,
					activeSubscriptionState: activeSubscription?.getState() ?? null,
				});
				ensureStream(true);
				const response = await app.chat({
					sessionId: args.sessionId,
					threadId: args.threadId,
					messages: userMessage ? getSubmitMessages(userMessage) : [],
					model: nextModel,
					reasoning: nextReasoning,
					mode: nextMode,
				});
				if (response.status === "queued" && userMessage) {
					messages = removeProvisionalSubmitMessage(messages, userMessage.id);
				}
				return {
					sessionId: args.sessionId,
					threadId: args.threadId,
					materialized: false,
					queued: response.status === "queued",
				};
			} catch (error) {
				completionRunning = false;
				const errorDetails = getStartChatErrorDetails(error);
				if (args.hasSession()) {
					await refresh();
				} else if (userMessage) {
					messages = removeProvisionalSubmitMessage(messages, userMessage.id);
				}
				pendingQuestionId =
					getPendingQuestionApprovalId(messages) ??
					errorDetails.pendingQuestionId;
				streamError = errorDetails.message;
				throw error;
			}
		},
		cancel: async () => {
			if (!args.hasSession()) {
				return;
			}
			await api.cancelThreadChat(args.sessionId, args.threadId);
			completionRunning = false;
			await refresh();
		},
		refresh,
		addToolApprovalResponse: ({
			id,
			approved,
			reason,
		}: {
			id: string;
			approved: boolean;
			reason?: string;
		}) => {
			addToolApprovalResponse(messages, { id, approved, reason });
		},
		dispose: () => {
			loadStatus = "idle";
			completionRunning = false;
			pendingQuestionId = null;
			void dismissRetryToast(args.threadId);
			resetLoadPromise();
			disconnectStream();
		},
	};
}
