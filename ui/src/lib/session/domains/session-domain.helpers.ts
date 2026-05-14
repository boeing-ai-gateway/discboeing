import { generateId } from "ai";

import type {
	ChatMessage,
	HookOutputResponse,
	HooksStatusResponse,
	Service,
	Session,
	Thread,
} from "$lib/api-types";
import type { DynamicToolPart } from "$lib/components/ai/types";
import type {
	ConversationComment,
	HookRunStatus,
	HooksStatus,
	HookOutputState,
	ServiceItem,
} from "$lib/session/session-context.types";
import type { PlanEntry } from "$lib/app/plan-entry";

type UserTextPart = Extract<ChatMessage["parts"][number], { type: "text" }>;
type UserFilePart = Extract<ChatMessage["parts"][number], { type: "file" }>;

export type UserMessageAttachment = {
	filename?: string;
	mediaType: string;
	url: string;
};

export function buildImplicitThread(session: Session | null): Thread[] {
	if (!session) {
		return [];
	}

	return [
		{
			id: session.id,
			name: session.displayName || session.name,
			model: session.model,
			reasoning: session.reasoning,
			promptQueue: [],
		},
	];
}

export function getNextSelectedThreadId(
	threads: Thread[],
	removedThreadId: string,
	currentSelectedId: string | null,
): string | null {
	const removedIndex = threads.findIndex(
		(thread) => thread.id === removedThreadId,
	);
	if (removedIndex === -1) {
		return currentSelectedId;
	}

	const remainingThreads = threads.filter(
		(thread) => thread.id !== removedThreadId,
	);
	if (currentSelectedId !== removedThreadId) {
		return currentSelectedId;
	}

	return (
		remainingThreads[removedIndex]?.id ??
		remainingThreads[removedIndex - 1]?.id ??
		null
	);
}

export function createUserMessage(
	text: string,
	options: {
		provisional?: boolean;
	} = {},
): ChatMessage {
	return createUserMessageFromParts(buildUserMessageParts(text), options);
}

export function createUserMessageFromParts(
	parts: ChatMessage["parts"],
	options: {
		provisional?: boolean;
	} = {},
): ChatMessage {
	return {
		id: generateId(),
		role: "user",
		parts,
		...(options.provisional ? { provisional: true } : {}),
	};
}

export function buildUserMessageParts(
	text: string,
	attachments: UserMessageAttachment[] = [],
): ChatMessage["parts"] {
	const parts: Array<UserTextPart | UserFilePart> = [];
	if (text.trim().length > 0) {
		parts.push({ type: "text", text });
	}
	parts.push(
		...attachments.map(({ filename, mediaType, url }) => ({
			type: "file" as const,
			filename,
			mediaType,
			url,
		})),
	);
	return parts;
}

export function quoteCommentSnippet(snippet: string): string {
	return snippet
		.split(/\r?\n/)
		.map((line) => `> ${line}`)
		.join("\n");
}

export function formatConversationComments(
	comments: Array<Omit<ConversationComment, "id">>,
): string {
	if (comments.length === 0) {
		return "";
	}
	return [
		"Comments on selected conversation text:",
		"",
		...comments.flatMap((comment, index) => [
			`${index + 1}. Selected text:`,
			quoteCommentSnippet(comment.snippet),
			"",
			"Comment:",
			comment.comment,
			"",
		]),
	]
		.join("\n")
		.trim();
}

export function hasUserMessageContent(parts: ChatMessage["parts"]): boolean {
	return parts.some((part) => {
		if (part.type === "text") {
			return part.text.trim().length > 0;
		}
		return part.type === "file";
	});
}

function bytesToBase64(bytes: Uint8Array): string {
	const bufferCtor = (
		globalThis as {
			Buffer?: {
				from: (input: Uint8Array) => {
					toString: (encoding: "base64") => string;
				};
			};
		}
	).Buffer;
	if (bufferCtor) {
		return bufferCtor.from(bytes).toString("base64");
	}

	let binary = "";
	const chunkSize = 0x8000;
	for (let index = 0; index < bytes.length; index += chunkSize) {
		binary += String.fromCharCode(...bytes.subarray(index, index + chunkSize));
	}
	return btoa(binary);
}

export async function createUserMessageAttachment(
	file: File,
): Promise<UserMessageAttachment> {
	const mediaType = file.type || "application/octet-stream";
	const base64 = bytesToBase64(new Uint8Array(await file.arrayBuffer()));
	return {
		filename: file.name,
		mediaType,
		url: `data:${mediaType};base64,${base64}`,
	};
}

export function getDynamicToolParts(message: ChatMessage): DynamicToolPart[] {
	return message.parts.filter(
		(part) => part.type === "dynamic-tool",
	) as unknown as DynamicToolPart[];
}

export function addToolApprovalResponse(
	messages: ChatMessage[],
	options: { id: string; approved: boolean; reason?: string },
): boolean {
	for (
		let messageIndex = messages.length - 1;
		messageIndex >= 0;
		messageIndex -= 1
	) {
		const message = messages[messageIndex];
		if (message.role !== "assistant") {
			continue;
		}

		for (const part of getDynamicToolParts(message)) {
			if (
				part.state === "approval-requested" &&
				part.approval?.id === options.id
			) {
				part.state = "approval-responded";
				part.approval = {
					id: options.id,
					approved: options.approved,
					...(options.reason ? { reason: options.reason } : {}),
				};
				return true;
			}
		}
	}

	return false;
}

export function getPendingQuestionApprovalId(
	messages: ChatMessage[],
): string | null {
	for (
		let messageIndex = messages.length - 1;
		messageIndex >= 0;
		messageIndex -= 1
	) {
		const message = messages[messageIndex];
		if (message.role !== "assistant") {
			continue;
		}

		for (const part of [...getDynamicToolParts(message)].reverse()) {
			if (part.state !== "approval-requested") {
				continue;
			}
			if (typeof part.approval?.id === "string") {
				return part.approval.id;
			}
			return part.toolCallId || null;
		}
	}

	return null;
}

export function getTodoWriteEntries(input: unknown): PlanEntry[] | null {
	if (!input || typeof input !== "object") {
		return null;
	}

	const todos = (input as { todos?: unknown }).todos;
	if (!Array.isArray(todos)) {
		return [];
	}

	return todos.flatMap((todo) => {
		if (!todo || typeof todo !== "object") {
			return [];
		}

		const candidate = todo as {
			content?: unknown;
			activeForm?: unknown;
			status?: unknown;
		};
		if (
			typeof candidate.content !== "string" ||
			typeof candidate.activeForm !== "string" ||
			(candidate.status !== "pending" &&
				candidate.status !== "in_progress" &&
				candidate.status !== "completed")
		) {
			return [];
		}

		return [
			{
				content: candidate.content,
				activeForm: candidate.activeForm,
				status: candidate.status,
			},
		];
	});
}

export function getPreviousTodoWriteEntries(
	messages: ChatMessage[],
	currentToolCallId: string,
): PlanEntry[] {
	let previousEntries: PlanEntry[] = [];

	for (const message of messages) {
		for (const part of getDynamicToolParts(message)) {
			if (part.toolName !== "TodoWrite") {
				continue;
			}

			if (part.toolCallId === currentToolCallId) {
				return previousEntries;
			}

			if (part.state !== "output-available") {
				continue;
			}

			const entries = getTodoWriteEntries(part.input);
			if (entries) {
				previousEntries = entries;
			}
		}
	}

	return previousEntries;
}

export function toServiceItem(service: Service): ServiceItem {
	const target =
		typeof service.https === "number"
			? `https://localhost:${service.https}${service.urlPath ?? ""}`
			: typeof service.http === "number"
				? `http://localhost:${service.http}${service.urlPath ?? ""}`
				: service.path;

	return {
		id: service.id,
		label: service.name,
		target,
		description: service.description,
		order: service.order,
		http: service.http,
		https: service.https,
		urlPath: service.urlPath,
		status: service.status,
		passive: service.passive,
		exitCode: service.exitCode,
	};
}

export function sortServiceItems(
	services: readonly ServiceItem[],
): ServiceItem[] {
	return [...services].sort((left, right) => {
		const leftOrder = left.order;
		const rightOrder = right.order;

		if (leftOrder !== undefined && rightOrder === undefined) {
			return -1;
		}
		if (leftOrder === undefined && rightOrder !== undefined) {
			return 1;
		}
		if (
			leftOrder !== undefined &&
			rightOrder !== undefined &&
			leftOrder !== rightOrder
		) {
			return leftOrder - rightOrder;
		}

		const labelCompare = left.label.localeCompare(right.label);
		if (labelCompare !== 0) {
			return labelCompare;
		}

		return left.id.localeCompare(right.id);
	});
}

export function toHooksStatus(
	response: HooksStatusResponse | null | undefined,
): HooksStatus {
	if (!response) {
		return { hooks: [], pendingHookIds: [] };
	}

	return {
		hooks: Object.values(response.hooks).map((hook) => ({
			hookId: hook.hookId,
			hookName: hook.hookName,
			type: hook.type,
			lastResult: hook.lastResult,
			lastRunAt: hook.lastRunAt,
			lastExitCode: hook.lastExitCode,
			runCount: hook.runCount,
			failCount: hook.failCount,
			command: undefined,
		})),
		pendingHookIds: response.pendingHooks,
	};
}

export function getHookDisplayState(
	hook: HookRunStatus,
	pendingHookIds: ReadonlySet<string>,
): HookRunStatus["lastResult"] {
	if (hook.lastResult === "running" || hook.lastResult === "failure") {
		return hook.lastResult;
	}

	if (pendingHookIds.has(hook.hookId)) {
		return "pending";
	}

	return hook.lastResult;
}

export function mergeHookOutput(
	outputById: Record<string, HookOutputState>,
	hookId: string,
	response: HookOutputResponse,
): Record<string, HookOutputState> {
	return {
		...outputById,
		[hookId]: {
			output: response.output,
			sizeBytes: response.sizeBytes,
			displayedBytes: response.displayedBytes,
			tooLarge: response.tooLarge,
		},
	};
}
