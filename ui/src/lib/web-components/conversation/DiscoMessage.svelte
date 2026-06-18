<svelte:options
	customElement={{
		tag: "disco-message",
		props: {
			from: { attribute: "from", type: "String" },
			state: { attribute: "state", type: "String" },
			provisional: { attribute: "provisional", type: "Boolean" },
			synthetic: { attribute: "synthetic", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import {
		booleanAttribute,
		createPartElement,
		getMessageFrom,
		getMessageState,
		getCustomElementHost,
		readJsonScript,
	} from "./dom";
	import type {
		DiscoMessageFrom,
		DiscoMessageInit,
		DiscoMessageState,
		DiscoPartInit,
	} from "./types";

	type Props = {
		from?: DiscoMessageFrom;
		state?: DiscoMessageState;
		provisional?: boolean;
		synthetic?: boolean;
	};

	let {
		from = "assistant",
		state: messageState = "complete",
		provisional = false,
		synthetic = false,
	}: Props = $props();

	type MessageHost = HTMLElement & {
		appendPart?: (init: DiscoPartInit) => Element;
		setState?: (state: DiscoMessageState) => void;
		toMessageInit?: () => DiscoMessageInit;
	};

	let root = $state<HTMLDivElement | null>(null);

	function getHost(): MessageHost | null {
		return root ? getCustomElementHost<MessageHost>(root) : null;
	}

	function appendPart(init: DiscoPartInit): Element {
		const host = getHost();
		if (!host) {
			throw new Error("disco-message is not mounted.");
		}
		const element = createPartElement(init);
		host.append(element);
		return element;
	}

	function setState(nextState: DiscoMessageState) {
		getHost()?.setAttribute("state", nextState);
	}

	function toMessageInit(): DiscoMessageInit {
		const host = getHost();
		if (!host) {
			return { from: "assistant", parts: [] };
		}
		const metadataElement = host.querySelector("disco-metadata");
		const metadata = metadataElement
			? (readJsonScript(metadataElement) as Record<string, unknown> | undefined)
			: undefined;
		return {
			id: host.id || undefined,
			from: getMessageFrom(host),
			state: getMessageState(host),
			createdAt: host.getAttribute("created-at") ?? undefined,
			model: host.getAttribute("model") ?? undefined,
			provisional: host.hasAttribute("provisional"),
			synthetic: host.hasAttribute("synthetic"),
			replacesMessageId: host.getAttribute("replaces-message-id") ?? undefined,
			replacedByMessageId:
				host.getAttribute("replaced-by-message-id") ?? undefined,
			metadata,
			parts: [],
		};
	}

	$effect(() => {
		const host = getHost();
		if (!host) {
			return;
		}
		host.setAttribute("from", from);
		host.setAttribute("state", messageState);
		host.toggleAttribute("provisional", provisional);
		host.toggleAttribute("synthetic", synthetic);
		host.appendPart = appendPart;
		host.setState = setState;
		host.toMessageInit = toMessageInit;
	});
</script>

<div
	part="container"
	bind:this={root}
	class="container"
	class:is-user={from === "user"}
	class:is-assistant={from !== "user"}
	data-from={from}
	data-state={messageState}
	data-provisional={booleanAttribute(provisional)}
	data-synthetic={booleanAttribute(synthetic)}
>
	<div part="content" class="content">
		<slot></slot>
	</div>
</div>

<style>
	:host {
		--disco-message-foreground: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		--disco-message-secondary: var(
			--disco-conversation-secondary,
			var(--disco-secondary, var(--secondary, #f3f4f6))
		);
		--disco-message-font-sans: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
		--disco-message-user-background: var(--disco-message-secondary, #f3f4f6);
		--disco-message-user-padding: 0.75rem 1rem;
		--disco-message-user-radius: 0.5rem;
		--disco-message-user-max-width: min(30rem, 92%);
		--disco-message-gap: 0.75rem;
		display: block;
		min-width: 0;
		color: var(--disco-message-foreground);
		font-family: var(--disco-message-font-sans);
	}

	:host([from="user"]) {
		--disco-message-content-padding: 0;
		--disco-message-content-radius: 0;
		--disco-message-content-width: 100%;
		--disco-message-content-max-width: 100%;
		--disco-message-content-background: transparent;
	}

	:host(:not([from="user"])) {
		--disco-message-content-padding: 0;
		--disco-message-content-radius: 0;
		--disco-message-content-width: 100%;
		--disco-message-content-max-width: 100%;
		--disco-message-content-background: transparent;
	}

	.container {
		display: flex;
		width: 100%;
		min-width: 0;
		flex-direction: column;
		gap: 0.5rem;
	}

	.container.is-user {
		--disco-attachment-background: transparent;
		--disco-attachment-hover-background: var(
			--disco-conversation-accent,
			var(--disco-accent, var(--accent, #f3f4f6))
		);
		--disco-attachment-border: color-mix(
			in oklab,
			var(
					--disco-conversation-border,
					var(--disco-border, var(--border, #e5e7eb))
				)
				60%,
			transparent
		);
		--disco-attachment-radius: 0.375rem;
		--disco-attachment-padding: 0.375rem 0.5rem;
		--disco-attachment-gap: 0.5rem;
		--disco-attachment-preview-size: 1.25rem;
		--disco-attachment-preview-background: color-mix(
			in oklab,
			var(--disco-conversation-muted, var(--disco-muted, var(--muted, #f3f4f6)))
				35%,
			transparent
		);
		--disco-attachment-filename-size: 0.8125rem;
		--disco-attachment-media-display: none;
		margin-left: auto;
		align-items: flex-end;
	}

	.content {
		display: flex;
		min-width: 0;
		width: 100%;
		flex-direction: column;
		gap: var(--disco-message-gap);
		font-size: 0.875rem;
		line-height: 1.25rem;
	}

	.container.is-user .content {
		width: fit-content;
		max-width: var(--disco-message-user-max-width);
		align-items: flex-start;
		border-radius: var(--disco-message-user-radius);
		background: var(--disco-message-user-background);
		padding: var(--disco-message-user-padding);
	}

	::slotted(disco-message-content),
	::slotted(disco-generated-text),
	::slotted(disco-reasoning),
	::slotted(disco-step-group),
	::slotted(disco-tool-call),
	::slotted(disco-attachment),
	::slotted(disco-browser-activity),
	::slotted(disco-event) {
		display: block;
		max-width: 100%;
	}
</style>
