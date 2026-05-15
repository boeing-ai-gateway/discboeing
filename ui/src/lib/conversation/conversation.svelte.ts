import { api } from "$lib/api-client";
import type { AppChatRequest } from "$lib/app/app-context.types";
import type {
	BrowserEventChunkData,
	ChatMessage,
	HooksStatusResponse,
	StartChatResponse,
	Thread,
} from "$lib/api-types";
import {
	createChatStreamEventListeners,
	createChatStreamState,
} from "$lib/conversation/conversation-stream";
import type {
	ProjectStreamManager,
	ProjectStreamSubscription,
} from "$lib/project/project-stream-manager";
import {
	createUserMessageFromParts,
	hasUserMessageContent,
} from "$lib/session/domains/session-domain.helpers";

const RETRY_TOAST_ID_PREFIX = "thread-retry-status:";

type CreateConversationDomainArgs = {
	sessionId: string;
	threadId: string;
	startChat: (data: AppChatRequest) => Promise<StartChatResponse>;
	projectStreams: ProjectStreamManager;
	refreshThread: () => Promise<void>;
	applyThreadUpdate?: (thread: Thread) => void;
	applyHooksStatusUpdate?: (
		status: HooksStatusResponse,
	) => void | Promise<void>;
	refreshSessionState?: () => Promise<void>;
	afterTurn?: () => Promise<void>;
};

function getStreamErrorMessage(error: unknown): string {
	return error instanceof Error
		? error.message
		: "Failed to process chat stream";
}

function sortBrowserEvents(
	events: BrowserEventChunkData[],
): BrowserEventChunkData[] {
	return [...events].sort((left, right) => {
		if (left.stepIndex !== right.stepIndex) {
			return left.stepIndex - right.stepIndex;
		}
		const leftTime = left.event.recordedAt
			? Date.parse(left.event.recordedAt)
			: Number.NaN;
		const rightTime = right.event.recordedAt
			? Date.parse(right.event.recordedAt)
			: Number.NaN;
		if (
			!Number.isNaN(leftTime) &&
			!Number.isNaN(rightTime) &&
			leftTime !== rightTime
		) {
			return leftTime - rightTime;
		}
		return left.event.eventId.localeCompare(right.event.eventId);
	});
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
	const submittedMessage = { ...userMessage };
	delete submittedMessage.provisional;
	return [submittedMessage];
}

export function removeProvisionalSubmitMessage(
	messages: ChatMessage[],
	messageId: string,
): ChatMessage[] {
	return messages.filter((message) => message.id !== messageId);
}

export function createConversationDomain(args: CreateConversationDomainArgs) {
	let messages = $state<ChatMessage[]>([]);
	let streamError = $state<string | null>(null);
	let completionRunning = $state(false);
	let browserEventsByTurnId = $state<Record<string, BrowserEventChunkData[]>>(
		{},
	);
	let activeSubscription: ProjectStreamSubscription | null = null;
	let activeStreamKey: string | null = null;

	function getStatus() {
		return streamError ? "error" : "ready";
	}

	const handleCompletionStart = () => {
		if (completionRunning) {
			return;
		}
		streamError = null;
		completionRunning = true;
	};

	const handleCompletionFinish = () => {
		if (!completionRunning) {
			return;
		}
		completionRunning = false;
		void dismissRetryToast(args.threadId);
		return args.afterTurn?.();
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
		onHistoryReplayStart: () => {
			browserEventsByTurnId = {};
		},
		onChunkError: (errorText) => {
			completionRunning = false;
			streamError = errorText;
			void dismissRetryToast(args.threadId);
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
		onBrowserEvent: (event) => {
			const turnId = event.turnId?.trim();
			if (!turnId) {
				return;
			}
			const current = browserEventsByTurnId[turnId] ?? [];
			const existingIndex = current.findIndex(
				(candidate) => candidate.event.eventId === event.event.eventId,
			);
			if (existingIndex === -1) {
				browserEventsByTurnId = {
					...browserEventsByTurnId,
					[turnId]: sortBrowserEvents([...current, event]),
				};
				return;
			}
			const next = [...current];
			next[existingIndex] = event;
			browserEventsByTurnId = {
				...browserEventsByTurnId,
				[turnId]: sortBrowserEvents(next),
			};
		},
	});

	function disconnectStream() {
		activeSubscription?.unsubscribe();
		activeSubscription = null;
		activeStreamKey = null;
	}

	function streamKey(sessionId: string) {
		return `${sessionId}:${args.threadId}`;
	}

	function ensureStream() {
		if (typeof window === "undefined") {
			return;
		}
		if (activeStreamKey === streamKey(args.sessionId)) {
			return;
		}

		disconnectStream();
		streamError = null;
		activeStreamKey = streamKey(args.sessionId);
		console.debug("[WS] Creating chat stream subscription", {
			threadId: args.threadId,
			sessionId: args.sessionId,
		});
		const listeners = createChatStreamEventListeners(streamState, {
			onError: (error) => {
				streamError = getStreamErrorMessage(error);
			},
		});
		const subscription = args.projectStreams.subscribe({
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
				});
				streamError = errorMessage;
			},
		});
		activeSubscription = subscription;
	}

	function connect() {
		streamError = null;
		ensureStream();
		return Promise.resolve();
	}

	return {
		get messages() {
			return messages;
		},
		get browserEventsByTurnId() {
			return browserEventsByTurnId;
		},
		get status() {
			return getStatus();
		},
		get isStreaming() {
			return completionRunning;
		},
		get error() {
			return streamError;
		},
		connect,
		submit: async ({
			parts,
			modelId,
			reasoning,
			serviceTier,
			workspaceId,
			providerId,
			workspaceType,
			workspacePath,
			runAfter,
		}: {
			parts: ChatMessage["parts"];
			modelId: string | null;
			reasoning: string | undefined;
			serviceTier: string | undefined;
			workspaceId?: string;
			providerId?: string;
			workspaceType?: "local" | "git" | null;
			workspacePath?: string | null;
			runAfter?: string;
		}) => {
			const hasMessageContent = hasUserMessageContent(parts);

			streamError = null;
			const nextModel = modelId ?? "";
			const nextReasoning = reasoning ?? "";
			const nextServiceTier = serviceTier ?? "";
			const submittingWhileGenerating = completionRunning;
			const shouldOptimisticallyInsert =
				!submittingWhileGenerating && !runAfter;
			const userMessage = hasMessageContent
				? createUserMessageFromParts(parts, {
						provisional: true,
					})
				: null;

			if (userMessage && shouldOptimisticallyInsert) {
				messages = [...messages, userMessage];
			}

			try {
				console.debug("[WS] Preparing chat submit", {
					threadId: args.threadId,
					sessionId: args.sessionId,
					status: getStatus(),
				});
				const response = await args.startChat({
					sessionId: args.sessionId,
					threadId: args.threadId,
					messages: userMessage ? getSubmitMessages(userMessage) : [],
					...(workspaceId ? { workspaceId } : {}),
					...(providerId ? { providerId } : {}),
					...(workspaceType && workspacePath
						? {
								workspaceType,
								workspacePath,
							}
						: {}),
					model: nextModel,
					reasoning: nextReasoning,
					serviceTier: nextServiceTier,
					...(runAfter ? { runAfter } : {}),
				});
				if (response.status === "queued" && userMessage) {
					messages = removeProvisionalSubmitMessage(messages, userMessage.id);
					await args.refreshThread();
				}
				return {
					sessionId: response.sessionId,
					threadId: response.threadId,
					queued: response.status === "queued",
				};
			} catch (error) {
				completionRunning = false;
				if (userMessage) {
					messages = removeProvisionalSubmitMessage(messages, userMessage.id);
				}
				streamError =
					error instanceof Error ? error.message : "Failed to start chat";
				throw error;
			}
		},
		cancel: () => {
			completionRunning = false;
			void api.cancelThreadChat(args.sessionId, args.threadId).then(
				() => {
					void Promise.all([
						args.refreshThread(),
						args.refreshSessionState?.(),
					]).catch((error) => {
						console.error(
							"Failed to refresh thread state after chat cancellation",
							error,
						);
					});
				},
				(error) => {
					streamError =
						error instanceof Error ? error.message : "Failed to cancel chat";
				},
			);
			return Promise.resolve();
		},
		dispose: () => {
			completionRunning = false;
			browserEventsByTurnId = {};
			void dismissRetryToast(args.threadId);
			disconnectStream();
		},
	};
}
