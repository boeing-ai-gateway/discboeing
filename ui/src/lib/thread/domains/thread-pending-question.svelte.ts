import { StartChatError } from "$lib/api-client";
import type { ChatMessage } from "$lib/api-types";
import { getPendingQuestionApprovalId } from "$lib/session/domains/session-domain.helpers";

export function getHasPendingQuestion(
	messages: ChatMessage[],
	hasPendingQuestionFromSubmitError: boolean,
): boolean {
	return (
		getPendingQuestionApprovalId(messages) !== null ||
		hasPendingQuestionFromSubmitError
	);
}

export function isStartChatPendingQuestionError(error: unknown): boolean {
	return (
		error instanceof StartChatError &&
		error.code === "pending_question_requires_answer"
	);
}

export function createThreadPendingQuestionState(args: {
	getMessages: () => ChatMessage[];
}) {
	let hasPendingQuestionFromSubmitError = $state(false);
	const clearSubmitError = () => {
		hasPendingQuestionFromSubmitError = false;
	};

	return {
		get hasPendingQuestion() {
			return getHasPendingQuestion(
				args.getMessages(),
				hasPendingQuestionFromSubmitError,
			);
		},
		clearSubmitError,
		applySubmitError: (error: unknown) => {
			hasPendingQuestionFromSubmitError =
				isStartChatPendingQuestionError(error);
		},
	};
}
