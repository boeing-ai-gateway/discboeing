import type { ChatMessage } from "$lib/api-types";

export type ConversationMessage = ChatMessage & {
	readonly renderId: string;
};

export type ConversationTurn = {
	id: string;
	readonly renderId: string;
	userMessages: ConversationMessage[];
	assistantMessages: ConversationMessage[];
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
	const turnIdCounts = new Map<string, number>();
	const messageIdCounts = new Map<string, number>();

	for (const message of messages) {
		if (message.synthetic && !isCompactionMessage(message)) {
			continue;
		}
		const renderMessage = withRenderId(
			message,
			nextStableRenderId(message.id, messageIdCounts),
		);
		const stableTurnId = getStableTurnId(message);
		if (stableTurnId) {
			if (!currentTurn || currentTurn.id !== stableTurnId) {
				currentTurn = createConversationTurn(
					stableTurnId,
					nextStableRenderId(stableTurnId, turnIdCounts),
				);
				turns.push(currentTurn);
			}
			if (message.role === "user") {
				currentTurn.userMessages.push(renderMessage);
			} else {
				currentTurn.assistantMessages.push(renderMessage);
			}
			continue;
		}

		if (message.role === "user") {
			if (!currentTurn || currentTurn.assistantMessages.length > 0) {
				currentTurn = createConversationTurn(
					message.id,
					nextStableRenderId(message.id, turnIdCounts),
				);
				currentTurn.userMessages.push(renderMessage);
				turns.push(currentTurn);
				continue;
			}

			currentTurn.userMessages.push(renderMessage);
			continue;
		}

		if (!currentTurn) {
			currentTurn = createConversationTurn(
				message.id,
				nextStableRenderId(message.id, turnIdCounts),
			);
			currentTurn.assistantMessages.push(renderMessage);
			turns.push(currentTurn);
			continue;
		}

		currentTurn.assistantMessages.push(renderMessage);
	}

	return turns;
}

function nextStableRenderId(id: string, counts: Map<string, number>): string {
	const count = (counts.get(id) ?? 0) + 1;
	counts.set(id, count);
	return count === 1 ? id : `${id}#${count}`;
}

function createConversationTurn(
	id: string,
	renderId: string,
): ConversationTurn {
	const turn = {
		id,
		userMessages: [],
		assistantMessages: [],
	} as unknown as ConversationTurn;
	Object.defineProperty(turn, "renderId", {
		value: renderId,
		enumerable: false,
	});
	return turn;
}

function withRenderId(
	message: ChatMessage,
	renderId: string,
): ConversationMessage {
	const renderMessage = { ...message } as ConversationMessage;
	Object.defineProperty(renderMessage, "renderId", {
		value: renderId,
		enumerable: false,
	});
	return renderMessage;
}

export function isCompactionMessage(message: ChatMessage): boolean {
	const metadata =
		message.metadata && typeof message.metadata === "object"
			? (message.metadata as Record<string, unknown>)
			: null;
	const discobot =
		metadata?.discobot && typeof metadata.discobot === "object"
			? (metadata.discobot as Record<string, unknown>)
			: null;
	return discobot?.kind === "compaction";
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
