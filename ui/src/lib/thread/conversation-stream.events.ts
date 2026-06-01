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

export type SourceURLChunk = {
	type: "source-url";
	sourceType?: string;
	sourceId?: string;
	url: string;
	title?: string;
};

export type ChatStreamChunk =
	| UIMessageChunk<ChatMessageMetadata, ChatMessageDataTypes>
	| SourceURLChunk;

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

function appendWebSearchResult(
	toolPart: Record<string, unknown>,
	source: Record<string, unknown>,
) {
	if (toolPart.type !== "dynamic-tool" || toolPart.toolName !== "WebSearch") {
		return;
	}
	if (typeof source.url !== "string" || source.url.trim() === "") {
		return;
	}

	const output = isObjectRecord(toolPart.output) ? { ...toolPart.output } : {};
	const results = Array.isArray(output.results) ? [...output.results] : [];
	if (
		results.some(
			(result) => isObjectRecord(result) && result.url === source.url,
		)
	) {
		return;
	}

	results.push({
		title: typeof source.title === "string" ? source.title : source.url,
		url: source.url,
	});
	toolPart.output = { ...output, results };
}

function extractURLResults(text: string) {
	const urls = new Set<string>();
	for (const match of text.matchAll(/https?:\/\/[^\s<>)\]]+/g)) {
		const url = match[0]?.replace(/[.,;:!?]+$/, "");
		if (url) {
			urls.add(url);
		}
	}
	return [...urls].map((url) => ({ title: url, url }));
}

function normalizeChatStreamMessageValue(message: unknown): unknown {
	if (!isObjectRecord(message) || !Array.isArray(message.parts)) {
		return message;
	}

	let latestWebSearchTool: Record<string, unknown> | null = null;
	return {
		...message,
		parts: message.parts.map((part) => {
			if (!isObjectRecord(part)) {
				return part;
			}
			if (part.type === "dynamic-tool") {
				const normalized = normalizeDynamicToolPart(part);
				if (normalized.toolName === "WebSearch") {
					latestWebSearchTool = normalized;
				}
				return normalized;
			}
			if (part.type === "source-url" && latestWebSearchTool) {
				appendWebSearchResult(latestWebSearchTool, part);
			}
			if (
				part.type === "text" &&
				latestWebSearchTool &&
				typeof part.text === "string"
			) {
				for (const result of extractURLResults(part.text)) {
					appendWebSearchResult(latestWebSearchTool, result);
				}
			}
			return part;
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
	const value = JSON.parse(data);
	if (
		isObjectRecord(value) &&
		value.type === "source-url" &&
		typeof value.url === "string"
	) {
		return {
			type: "source-url",
			sourceType:
				typeof value.sourceType === "string" ? value.sourceType : undefined,
			sourceId: typeof value.sourceId === "string" ? value.sourceId : undefined,
			url: value.url,
			title: typeof value.title === "string" ? value.title : undefined,
		};
	}

	const schema = uiMessageChunkSchema();
	if (!schema.validate) {
		throw new Error(
			"UIMessageChunk schema does not expose a validate function",
		);
	}

	const validation = await schema.validate(value);
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
