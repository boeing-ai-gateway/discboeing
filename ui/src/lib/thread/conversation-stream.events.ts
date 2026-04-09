import type { UIMessageChunk } from "ai";
import { safeValidateUIMessages, uiMessageChunkSchema } from "ai";

import type {
	ChatMessage,
	ChatMessageDataTypes,
	ChatMessageMetadata,
} from "$lib/api-types";

export type ChatStreamEvent =
	| {
			event: "history-start";
			data: string;
	  }
	| {
			event: "history-message";
			data: string;
	  }
	| {
			event: "history-end";
			data: string;
	  }
	| {
			event: "chunk";
			data: string;
	  }
	| {
			event: "ping";
			data: string;
	  };

export type ChatStreamEventName = ChatStreamEvent["event"];

export type ChatStreamEventSource = {
	addEventListener: (
		type: ChatStreamEventName,
		listener: (event: MessageEvent<string>) => void,
	) => void;
	removeEventListener: (
		type: ChatStreamEventName,
		listener: (event: MessageEvent<string>) => void,
	) => void;
};

export type ChatStreamEventSourceOptions = {
	onError?: (error: unknown) => void;
};

export type ChatStreamChunk = UIMessageChunk<
	ChatMessageMetadata,
	ChatMessageDataTypes
>;

function isObjectRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null;
}

function normalizeDynamicToolPart(
	part: Record<string, unknown>,
): Record<string, unknown> {
	const normalized = { ...part };
	const state = normalized.state;
	const toolCallId =
		typeof normalized.toolCallId === "string"
			? normalized.toolCallId
			: undefined;

	if (
		state === "input-available" ||
		state === "approval-requested" ||
		state === "approval-responded" ||
		state === "output-available" ||
		state === "output-denied"
	) {
		if (!("input" in normalized)) {
			normalized.input = null;
		}
	}

	if (state === "output-error" && !("input" in normalized)) {
		normalized.input =
			"rawInput" in normalized ? (normalized.rawInput ?? null) : null;
	}

	if (
		(state === "approval-requested" ||
			state === "approval-responded" ||
			state === "output-available" ||
			state === "output-error" ||
			state === "output-denied") &&
		toolCallId &&
		(!isObjectRecord(normalized.approval) ||
			typeof normalized.approval.id !== "string")
	) {
		normalized.approval = { id: toolCallId };
	}

	if (
		(state === "approval-responded" ||
			state === "output-available" ||
			state === "output-error") &&
		isObjectRecord(normalized.approval) &&
		typeof normalized.approval.id === "string" &&
		typeof normalized.approval.approved !== "boolean"
	) {
		normalized.approval = {
			...normalized.approval,
			approved: true,
		};
	}

	if (
		state === "output-denied" &&
		isObjectRecord(normalized.approval) &&
		typeof normalized.approval.id === "string" &&
		typeof normalized.approval.approved !== "boolean"
	) {
		normalized.approval = {
			...normalized.approval,
			approved: false,
		};
	}

	return normalized;
}

function normalizeChatStreamMessageValue(message: unknown): unknown {
	if (!isObjectRecord(message) || !Array.isArray(message.parts)) {
		return message;
	}

	return {
		...message,
		parts: message.parts.map((part) => {
			if (!isObjectRecord(part) || part.type !== "dynamic-tool") {
				return part;
			}
			return normalizeDynamicToolPart(part);
		}),
	};
}

export async function parseChatStreamMessageValue(
	message: unknown,
): Promise<ChatMessage> {
	const normalizedMessage = normalizeChatStreamMessageValue(message);
	const validation = await safeValidateUIMessages<ChatMessage>({
		messages: [normalizedMessage],
	});
	if (!validation.success) {
		throw validation.error;
	}

	return validation.data[0];
}

export async function parseChatStreamMessage(
	data: string,
): Promise<ChatMessage> {
	return parseChatStreamMessageValue(JSON.parse(data));
}

export async function parseChatStreamChunk(
	data: string,
): Promise<ChatStreamChunk> {
	const schema = uiMessageChunkSchema();
	if (!schema.validate) {
		throw new Error(
			"UIMessageChunk schema does not expose a validate function",
		);
	}

	const validation = await schema.validate(JSON.parse(data));
	if (!validation.success) {
		throw validation.error;
	}

	return validation.value as ChatStreamChunk;
}

export type ChatStreamEventListenerBinding = {
	type: ChatStreamEventName;
	listener: (event: MessageEvent<string>) => void;
};

export function createChatStreamEventListeners(
	streamState: {
		handleStreamEvent: (event: ChatStreamEvent) => Promise<unknown>;
	},
	options: ChatStreamEventSourceOptions = {},
): ChatStreamEventListenerBinding[] {
	const handleError = (error: unknown) => {
		if (options.onError) {
			options.onError(error);
			return;
		}

		console.error("Failed to process chat stream event", error);
	};

	const dispatchEvent = (event: ChatStreamEvent) => {
		void streamState.handleStreamEvent(event).catch(handleError);
	};

	return [
		{
			type: "history-start",
			listener: (event: MessageEvent<string>) => {
				dispatchEvent({ event: "history-start", data: event.data });
			},
		},
		{
			type: "history-message",
			listener: (event: MessageEvent<string>) => {
				dispatchEvent({ event: "history-message", data: event.data });
			},
		},
		{
			type: "history-end",
			listener: (event: MessageEvent<string>) => {
				dispatchEvent({ event: "history-end", data: event.data });
			},
		},
		{
			type: "chunk",
			listener: (event: MessageEvent<string>) => {
				dispatchEvent({ event: "chunk", data: event.data });
			},
		},
		{
			type: "ping",
			listener: (event: MessageEvent<string>) => {
				dispatchEvent({ event: "ping", data: event.data });
			},
		},
	];
}

export function bindChatStreamEventSource(
	eventSource: ChatStreamEventSource,
	streamState: {
		handleStreamEvent: (event: ChatStreamEvent) => Promise<unknown>;
	},
	options: ChatStreamEventSourceOptions = {},
) {
	const bindings = createChatStreamEventListeners(streamState, options);

	for (const { type, listener } of bindings) {
		eventSource.addEventListener(type, listener);
	}

	return () => {
		for (const { type, listener } of bindings) {
			eventSource.removeEventListener(type, listener);
		}
	};
}
