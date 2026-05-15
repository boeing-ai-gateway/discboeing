import { api, StartChatError } from "$lib/api-client";
import type {
	ChatMessage,
	PendingQuestion,
	PendingQuestionResponse,
} from "$lib/api-types";
import type { DynamicToolPart } from "$lib/components/ai/types";
import { getPendingQuestionApprovalId } from "$lib/session/domains/session-domain.helpers";

export function getHasPendingQuestion(
	messages: ChatMessage[],
	hasPendingQuestionFromSubmitError: boolean,
	hasPendingQuestionFromThread = false,
): boolean {
	return (
		getPendingQuestionApprovalId(messages) !== null ||
		hasPendingQuestionFromSubmitError ||
		hasPendingQuestionFromThread
	);
}

export function resolvePendingQuestionToolName(
	question: Pick<PendingQuestion, "context" | "credentials">,
): DynamicToolPart["toolName"] {
	if (question.context === "request_commit_pull") {
		return "RequestCommitPull";
	}
	if ((question.credentials?.length ?? 0) > 0) {
		return "RequestUserCredential";
	}
	return "AskUserQuestion";
}

export function buildPendingQuestionFallbackToolPart(
	question: PendingQuestion,
): DynamicToolPart {
	const toolName = resolvePendingQuestionToolName(question);
	const input =
		toolName === "RequestUserCredential"
			? { credentials: question.credentials ?? [] }
			: { questions: question.questions ?? [] };

	return {
		type: "dynamic-tool",
		toolCallId: question.toolUseID,
		toolName,
		state: "approval-requested",
		input,
		approval: { id: question.toolUseID },
	};
}

export function isStartChatPendingQuestionError(error: unknown): boolean {
	return (
		error instanceof StartChatError &&
		error.code === "pending_question_requires_answer"
	);
}

export function createThreadPendingQuestionState(args: {
	sessionId: string;
	threadId: string;
	getMessages: () => ChatMessage[];
	getThreadPendingQuestion?: () => boolean;
}) {
	let hasPendingQuestionFromSubmitError = $state(false);
	let pendingQuestionToolPart = $state<DynamicToolPart | null>(null);
	let pendingQuestionLoading = $state(false);
	let pendingQuestionError = $state<string | null>(null);
	let lastFetchedPendingQuestionKey = $state<string | null>(null);
	const clearSubmitError = () => {
		hasPendingQuestionFromSubmitError = false;
	};

	$effect(() => {
		const hasPendingQuestionFromThread =
			args.getThreadPendingQuestion?.() ?? false;
		const hasPendingQuestionFromMessages =
			getPendingQuestionApprovalId(args.getMessages()) !== null;
		if (!hasPendingQuestionFromThread || hasPendingQuestionFromMessages) {
			pendingQuestionToolPart = null;
			pendingQuestionLoading = false;
			pendingQuestionError = null;
			lastFetchedPendingQuestionKey = null;
			return;
		}

		const fetchKey = `${args.sessionId}:${args.threadId}`;
		if (lastFetchedPendingQuestionKey === fetchKey) {
			return;
		}

		lastFetchedPendingQuestionKey = fetchKey;
		pendingQuestionToolPart = null;
		pendingQuestionLoading = true;
		pendingQuestionError = null;

		let cancelled = false;
		void api
			.getCurrentThreadChatQuestion(args.sessionId, args.threadId)
			.then((result: PendingQuestionResponse) => {
				if (cancelled) {
					return;
				}
				pendingQuestionToolPart =
					result.status === "pending" && result.question
						? buildPendingQuestionFallbackToolPart(result.question)
						: null;
			})
			.catch((error) => {
				if (cancelled) {
					return;
				}
				pendingQuestionError =
					error instanceof Error
						? error.message
						: "Failed to load the pending question.";
			})
			.finally(() => {
				if (cancelled) {
					return;
				}
				pendingQuestionLoading = false;
			});

		return () => {
			cancelled = true;
		};
	});

	return {
		get hasPendingQuestion() {
			return getHasPendingQuestion(
				args.getMessages(),
				hasPendingQuestionFromSubmitError,
				args.getThreadPendingQuestion?.() ?? false,
			);
		},
		get pendingQuestionToolPart() {
			return pendingQuestionToolPart;
		},
		get pendingQuestionLoading() {
			return pendingQuestionLoading;
		},
		get pendingQuestionError() {
			return pendingQuestionError;
		},
		clearSubmitError,
		applySubmitError: (error: unknown) => {
			hasPendingQuestionFromSubmitError =
				isStartChatPendingQuestionError(error);
		},
	};
}
