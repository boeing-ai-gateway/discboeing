import type {
	DiscoAttachmentPartInit,
	DiscoBrowserActivityPartInit,
	DiscoEventPartInit,
	DiscoMessageFrom,
	DiscoMessageInit,
	DiscoMessageState,
	DiscoPartInit,
	DiscoToolPartInit,
} from "./types";

export function getCustomElementHost<T extends HTMLElement = HTMLElement>(
	node: Node,
): T {
	const root = node.getRootNode();
	if (root instanceof ShadowRoot) {
		return root.host as T;
	}
	return node as T;
}

export function booleanAttribute(value: boolean | undefined): boolean {
	return value === true;
}

export function setBooleanAttribute(
	element: Element,
	name: string,
	value: boolean | undefined,
) {
	if (value) {
		element.setAttribute(name, "");
	} else {
		element.removeAttribute(name);
	}
}

export function readJsonScript(element: Element): unknown {
	const script = element.querySelector<HTMLScriptElement>(
		':scope > script[type="application/json"]',
	);
	const text = script?.textContent?.trim();
	if (!text) {
		return undefined;
	}
	return JSON.parse(text);
}

export function writeJsonScript(element: Element, value: unknown) {
	let script = element.querySelector<HTMLScriptElement>(
		':scope > script[type="application/json"]',
	);
	if (!script) {
		script = document.createElement("script");
		script.type = "application/json";
		element.replaceChildren(script);
	}
	script.textContent = `\n${JSON.stringify(value, null, "\t")}\n`;
}

export function appendTextDelta(element: Element, text: string) {
	const lastChild = element.lastChild;
	if (lastChild?.nodeType === Node.TEXT_NODE) {
		lastChild.textContent = `${lastChild.textContent ?? ""}${text}`;
		return;
	}
	element.append(document.createTextNode(text));
}

export function createMessageElement(init: DiscoMessageInit): HTMLElement {
	const element = document.createElement("disco-message");
	if (init.id) {
		element.id = init.id;
	}
	element.setAttribute("from", init.from);
	if (init.state) {
		element.setAttribute("state", init.state);
	}
	if (init.createdAt) {
		element.setAttribute("created-at", init.createdAt);
	}
	if (init.model) {
		element.setAttribute("model", init.model);
	}
	setBooleanAttribute(element, "provisional", init.provisional);
	setBooleanAttribute(element, "synthetic", init.synthetic);
	if (init.replacesMessageId) {
		element.setAttribute("replaces-message-id", init.replacesMessageId);
	}
	if (init.replacedByMessageId) {
		element.setAttribute("replaced-by-message-id", init.replacedByMessageId);
	}
	if (init.metadata) {
		const metadata = document.createElement("disco-metadata");
		writeJsonScript(metadata, init.metadata);
		element.append(metadata);
	}
	for (const part of init.parts ?? []) {
		element.append(createPartElement(part));
	}
	return element;
}

export function createPartElement(init: DiscoPartInit): Element {
	switch (init.type) {
		case "text": {
			const element = document.createElement("disco-message-content");
			setOptionalAttribute(element, "part-id", init.partId);
			element.setAttribute("format", init.format ?? "markdown");
			element.textContent = init.text;
			return element;
		}
		case "reasoning": {
			const element = document.createElement("disco-reasoning");
			setOptionalAttribute(element, "part-id", init.partId);
			setOptionalAttribute(element, "state", init.state);
			setBooleanAttribute(element, "open", init.open);
			element.textContent = init.text;
			return element;
		}
		case "tool-call":
			return createToolCallElement(init);
		case "attachment":
			return createAttachmentElement(init);
		case "event":
			return createEventElement(init);
		case "browser-activity":
			return createBrowserActivityElement(init);
	}
}

function createToolCallElement(init: DiscoToolPartInit): Element {
	const element = document.createElement("disco-tool-call");
	setOptionalAttribute(element, "part-id", init.partId);
	element.setAttribute("call-id", init.callId);
	element.setAttribute("name", init.name);
	element.setAttribute("state", init.state);
	setOptionalAttribute(element, "title", init.title);
	setOptionalAttribute(element, "approval-id", init.approvalId);
	setBooleanAttribute(element, "open", init.open);
	if (init.approved !== undefined) {
		element.setAttribute("approved", String(init.approved));
	}
	setOptionalAttribute(element, "reason", init.reason);
	if (init.input !== undefined) {
		const input = document.createElement("disco-tool-input");
		input.setAttribute("format", "json");
		writeJsonScript(input, init.input);
		element.append(input);
	}
	if (init.output !== undefined) {
		const output = document.createElement("disco-tool-output");
		output.setAttribute("format", "json");
		writeJsonScript(output, init.output);
		element.append(output);
	}
	if (init.errorText) {
		const output = document.createElement("disco-tool-output");
		output.setAttribute("format", "text");
		output.textContent = init.errorText;
		element.append(output);
	}
	return element;
}

function createAttachmentElement(init: DiscoAttachmentPartInit): Element {
	const element = document.createElement("disco-attachment");
	setOptionalAttribute(element, "part-id", init.partId);
	setOptionalAttribute(element, "kind", init.kind);
	setOptionalAttribute(element, "src", init.src);
	setOptionalAttribute(element, "filename", init.filename);
	setOptionalAttribute(element, "media-type", init.mediaType);
	return element;
}

function createEventElement(init: DiscoEventPartInit): Element {
	const element = document.createElement("disco-event");
	setOptionalAttribute(element, "part-id", init.partId);
	element.setAttribute("kind", init.kind);
	setOptionalAttribute(element, "title", init.title);
	setOptionalAttribute(element, "summary", init.summary);
	setBooleanAttribute(element, "open", init.open);
	if (init.data !== undefined) {
		const metadata = document.createElement("disco-metadata");
		writeJsonScript(metadata, init.data);
		element.append(metadata);
	}
	return element;
}

function createBrowserActivityElement(
	init: DiscoBrowserActivityPartInit,
): Element {
	const element = document.createElement("disco-browser-activity");
	setOptionalAttribute(element, "part-id", init.partId);
	setOptionalAttribute(element, "title", init.title);
	setOptionalAttribute(element, "summary", init.summary);
	if (init.stepCount !== undefined) {
		element.setAttribute("step-count", String(init.stepCount));
	}
	setBooleanAttribute(element, "open", init.open);
	if (init.data !== undefined) {
		const metadata = document.createElement("disco-metadata");
		writeJsonScript(metadata, init.data);
		element.append(metadata);
	}
	return element;
}

export function setOptionalAttribute(
	element: Element,
	name: string,
	value: string | undefined,
) {
	if (value === undefined || value === "") {
		element.removeAttribute(name);
		return;
	}
	element.setAttribute(name, value);
}

export function getMessageFrom(element: Element): DiscoMessageFrom {
	const from = element.getAttribute("from");
	if (
		from === "user" ||
		from === "assistant" ||
		from === "system" ||
		from === "tool"
	) {
		return from;
	}
	return "assistant";
}

export function getMessageState(element: Element): DiscoMessageState {
	const state = element.getAttribute("state");
	if (
		state === "pending" ||
		state === "streaming" ||
		state === "complete" ||
		state === "error"
	) {
		return state;
	}
	return "complete";
}

export function emitComposedEvent<T>(
	element: Element,
	type: string,
	detail: T,
	options: { cancelable?: boolean } = {},
): boolean {
	return element.dispatchEvent(
		new CustomEvent(type, {
			detail,
			bubbles: true,
			composed: true,
			cancelable: options.cancelable ?? false,
		}),
	);
}
