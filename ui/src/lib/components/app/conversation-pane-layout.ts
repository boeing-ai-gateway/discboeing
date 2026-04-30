import type { ChatMessage } from "$lib/api-types";

export type ConversationTurn = {
	id: string;
	userMessages: ChatMessage[];
	assistantMessages: ChatMessage[];
};

export type ReservedTurnMinHeightArgs = {
	currentTurnHeight: number;
	contentTopPadding: number;
	turnTopPadding: number;
	viewportClientHeight: number;
	viewportPaddingBottom: number;
	viewportPaddingTop: number;
};

export function groupMessagesIntoTurns(
	messages: ChatMessage[],
): ConversationTurn[] {
	const turns: ConversationTurn[] = [];
	let currentTurn: ConversationTurn | null = null;

	for (const message of messages) {
		const stableTurnId = getStableTurnId(message);
		if (stableTurnId) {
			if (!currentTurn || currentTurn.id !== stableTurnId) {
				currentTurn = {
					id: stableTurnId,
					userMessages: [],
					assistantMessages: [],
				};
				turns.push(currentTurn);
			}
			if (message.role === "user") {
				currentTurn.userMessages = [...currentTurn.userMessages, message];
			} else {
				currentTurn.assistantMessages = [
					...currentTurn.assistantMessages,
					message,
				];
			}
			continue;
		}

		if (message.role === "user") {
			if (!currentTurn || currentTurn.assistantMessages.length > 0) {
				currentTurn = {
					id: message.id,
					userMessages: [message],
					assistantMessages: [],
				};
				turns.push(currentTurn);
				continue;
			}

			currentTurn.userMessages = [...currentTurn.userMessages, message];
			continue;
		}

		if (!currentTurn) {
			currentTurn = {
				id: message.id,
				userMessages: [],
				assistantMessages: [message],
			};
			turns.push(currentTurn);
			continue;
		}

		currentTurn.assistantMessages = [...currentTurn.assistantMessages, message];
	}

	return turns;
}

function getStableTurnId(message: ChatMessage): string | null {
	const metadata =
		message.metadata && typeof message.metadata === "object"
			? (message.metadata as Record<string, unknown>)
			: null;
	const discobot =
		metadata?.discobot && typeof metadata.discobot === "object"
			? (metadata.discobot as Record<string, unknown>)
			: null;
	return typeof discobot?.turnId === "string" && discobot.turnId.trim() !== ""
		? discobot.turnId
		: null;
}

export function getReservedTurnMinHeight({
	currentTurnHeight,
	contentTopPadding,
	turnTopPadding,
	viewportClientHeight,
	viewportPaddingBottom,
	viewportPaddingTop,
}: ReservedTurnMinHeightArgs): number {
	const viewportInnerHeight = Math.max(
		0,
		viewportClientHeight - viewportPaddingTop - viewportPaddingBottom,
	);
	const availableTurnHeight = Math.max(
		0,
		viewportInnerHeight - Math.max(0, contentTopPadding),
	);
	const compensatedAvailableTurnHeight =
		availableTurnHeight + Math.max(0, turnTopPadding);

	return Math.max(
		Math.ceil(currentTurnHeight),
		Math.floor(compensatedAvailableTurnHeight),
	);
}
