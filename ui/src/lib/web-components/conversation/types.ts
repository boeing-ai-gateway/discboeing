export type DiscoConversationStatus = "idle" | "loading" | "ready" | "error";
export type DiscoConversationChatWidth = "full" | "constrained";
export type DiscoMessageFrom = "user" | "assistant" | "system" | "tool";
export type DiscoMessageState = "pending" | "streaming" | "complete" | "error";
export type DiscoPartState = DiscoMessageState;
export type DiscoToolState =
	| "input-streaming"
	| "input-available"
	| "approval-requested"
	| "approval-responded"
	| "output-available"
	| "output-error"
	| "output-denied";
export type DiscoContentFormat = "text" | "markdown" | "json";
export type DiscoAttachmentKind = "file" | "image" | "source" | "artifact";

export type DiscoMetadataValue = Record<string, unknown>;

export type DiscoTextPartInit = {
	type: "text";
	partId?: string;
	text: string;
	format?: Exclude<DiscoContentFormat, "json">;
};

export type DiscoReasoningPartInit = {
	type: "reasoning";
	partId?: string;
	text: string;
	state?: DiscoPartState;
	open?: boolean;
};

export type DiscoToolPartInit = {
	type: "tool-call";
	partId?: string;
	callId: string;
	name: string;
	state: DiscoToolState;
	title?: string;
	input?: unknown;
	output?: unknown;
	errorText?: string;
	approvalId?: string;
	approved?: boolean;
	reason?: string;
	open?: boolean;
};

export type DiscoAttachmentPartInit = {
	type: "attachment";
	partId?: string;
	kind?: DiscoAttachmentKind;
	src?: string;
	filename?: string;
	mediaType?: string;
};

export type DiscoEventPartInit = {
	type: "event";
	partId?: string;
	kind: string;
	title?: string;
	summary?: string;
	data?: unknown;
	open?: boolean;
};

export type DiscoBrowserActivityPartInit = {
	type: "browser-activity";
	partId?: string;
	title?: string;
	summary?: string;
	stepCount?: number;
	data?: unknown;
	open?: boolean;
};

export type DiscoPartInit =
	| DiscoTextPartInit
	| DiscoReasoningPartInit
	| DiscoToolPartInit
	| DiscoAttachmentPartInit
	| DiscoEventPartInit
	| DiscoBrowserActivityPartInit;

export type DiscoMessageInit = {
	id?: string;
	from: DiscoMessageFrom;
	state?: DiscoMessageState;
	createdAt?: string;
	model?: string;
	provisional?: boolean;
	synthetic?: boolean;
	replacesMessageId?: string;
	replacedByMessageId?: string;
	metadata?: DiscoMetadataValue;
	parts?: DiscoPartInit[];
};

export type DiscoToolApprovalResponse = {
	approvalId?: string;
	approved: boolean;
	reason?: string;
};

export type DiscoLinkOpenRequestDetail = {
	url: string;
	messageId?: string;
	partId?: string;
};

export type DiscoAttachmentOpenRequestDetail = {
	messageId?: string;
	partId?: string;
	kind?: DiscoAttachmentKind;
	src?: string;
	filename?: string;
	mediaType?: string;
};

export type DiscoToolApprovalRequestDetail = {
	messageId?: string;
	partId?: string;
	callId: string;
	name: string;
	approvalId?: string;
	input?: unknown;
};

export type DiscoToolApprovalResponseDetail = DiscoToolApprovalRequestDetail &
	DiscoToolApprovalResponse;

export type DiscoMessageCopyRequestDetail = {
	messageId: string;
	text: string;
};

export type DiscoMessageRetryRequestDetail = {
	messageId: string;
};

export type DiscoSelectionCommentRequestDetail = {
	text: string;
	messageId?: string;
	partId?: string;
};

export type DiscoScrollStateChangeDetail = {
	isNearBottom: boolean;
	stickToBottom: boolean;
	scrollTop: number;
	scrollHeight: number;
	clientHeight: number;
};

export type DiscoExpandChangeDetail = {
	turnId?: string;
	messageId?: string;
	partId?: string;
	open: boolean;
};

export type DiscoPartActionDetail = {
	messageId?: string;
	partId?: string;
	partType: string;
	action: string;
	data?: unknown;
};

export type DiscoConversationEventMap = {
	"disco-link-open-request": CustomEvent<DiscoLinkOpenRequestDetail>;
	"disco-attachment-open-request": CustomEvent<DiscoAttachmentOpenRequestDetail>;
	"disco-tool-approval-request": CustomEvent<DiscoToolApprovalRequestDetail>;
	"disco-tool-approval-response": CustomEvent<DiscoToolApprovalResponseDetail>;
	"disco-message-copy-request": CustomEvent<DiscoMessageCopyRequestDetail>;
	"disco-message-retry-request": CustomEvent<DiscoMessageRetryRequestDetail>;
	"disco-selection-comment-request": CustomEvent<DiscoSelectionCommentRequestDetail>;
	"disco-scroll-state-change": CustomEvent<DiscoScrollStateChangeDetail>;
	"disco-expand-change": CustomEvent<DiscoExpandChangeDetail>;
	"disco-part-action": CustomEvent<DiscoPartActionDetail>;
};

export interface DiscoConversationElement extends HTMLElement {
	status: DiscoConversationStatus;
	autoScroll: boolean;
	chatWidth: DiscoConversationChatWidth;
	appendMessage(init: DiscoMessageInit): DiscoMessageElement;
	replaceMessages(inits: DiscoMessageInit[]): DiscoMessageElement[];
	clearMessages(): void;
	getMessages(): DiscoMessageInit[];
	getMessage(id: string): DiscoMessageElement | null;
	appendPart(messageId: string, init: DiscoPartInit): Element;
	scrollToBottom(options?: ScrollIntoViewOptions): void;
	addEventListener<K extends keyof DiscoConversationEventMap>(
		type: K,
		listener: (
			this: DiscoConversationElement,
			event: DiscoConversationEventMap[K],
		) => void,
		options?: boolean | AddEventListenerOptions,
	): void;
	addEventListener(
		type: string,
		listener: EventListenerOrEventListenerObject | null,
		options?: boolean | AddEventListenerOptions,
	): void;
	removeEventListener<K extends keyof DiscoConversationEventMap>(
		type: K,
		listener: (
			this: DiscoConversationElement,
			event: DiscoConversationEventMap[K],
		) => void,
		options?: boolean | EventListenerOptions,
	): void;
	removeEventListener(
		type: string,
		listener: EventListenerOrEventListenerObject | null,
		options?: boolean | EventListenerOptions,
	): void;
}

export interface DiscoTurnElement extends HTMLElement {
	id: string;
	open: boolean;
}

export interface DiscoStepGroupElement extends HTMLElement {
	open: boolean;
	label?: string;
}

export interface DiscoMessageElement extends HTMLElement {
	from: DiscoMessageFrom;
	state: DiscoMessageState;
	provisional: boolean;
	synthetic: boolean;
	appendPart(init: DiscoPartInit): Element;
	setState(state: DiscoMessageState): void;
	toMessageInit(): DiscoMessageInit;
}

export interface DiscoMessageContentElement extends HTMLElement {
	format: Exclude<DiscoContentFormat, "json">;
	partId?: string;
	appendTextDelta(text: string): void;
}

export interface DiscoReasoningElement extends HTMLElement {
	state: DiscoPartState;
	partId?: string;
	open: boolean;
	appendTextDelta(text: string): void;
}

export interface DiscoToolCallElement extends HTMLElement {
	partId?: string;
	callId: string;
	name: string;
	state: DiscoToolState;
	open: boolean;
	respond(response: DiscoToolApprovalResponse): void;
	setInput(value: unknown): void;
	setOutput(value: unknown): void;
}

export interface DiscoToolInputElement extends HTMLElement {
	format: "json" | "text";
	value: unknown;
	setValue(value: unknown): void;
}

export interface DiscoToolOutputElement extends HTMLElement {
	format: "json" | "text";
	value: unknown;
	setValue(value: unknown): void;
}

export interface DiscoAttachmentElement extends HTMLElement {
	partId?: string;
	kind: DiscoAttachmentKind;
	src?: string;
	filename?: string;
	mediaType?: string;
}

export interface DiscoEventElement extends HTMLElement {
	partId?: string;
	kind: string;
	open: boolean;
	data?: unknown;
}

export interface DiscoBrowserActivityElement extends HTMLElement {
	partId?: string;
	open: boolean;
	stepCount?: number;
	data?: unknown;
}

export interface DiscoMetadataElement extends HTMLElement {
	value: DiscoMetadataValue;
	setValue(value: DiscoMetadataValue): void;
}

declare global {
	interface HTMLElementTagNameMap {
		"disco-conversation": DiscoConversationElement;
		"disco-turn": DiscoTurnElement;
		"disco-step-group": DiscoStepGroupElement;
		"disco-message": DiscoMessageElement;
		"disco-message-content": DiscoMessageContentElement;
		"disco-reasoning": DiscoReasoningElement;
		"disco-tool-call": DiscoToolCallElement;
		"disco-tool-input": DiscoToolInputElement;
		"disco-tool-output": DiscoToolOutputElement;
		"disco-attachment": DiscoAttachmentElement;
		"disco-event": DiscoEventElement;
		"disco-browser-activity": DiscoBrowserActivityElement;
		"disco-metadata": DiscoMetadataElement;
	}
}
