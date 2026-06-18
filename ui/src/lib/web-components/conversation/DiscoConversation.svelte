<svelte:options
	customElement={{
		tag: "disco-conversation",
		props: {
			autoScroll: { attribute: "auto-scroll", type: "Boolean" },
			chatWidth: { attribute: "chat-width", type: "String" },
		},
	}}
/>

<script lang="ts">
	import { tick } from "svelte";
	import ArrowDownIcon from "@lucide/svelte/icons/arrow-down";
	import {
		createMessageElement,
		createPartElement,
		emitComposedEvent,
		getCustomElementHost,
	} from "./dom";
	import type {
		DiscoConversationChatWidth,
		DiscoConversationStatus,
		DiscoMessageElement,
		DiscoMessageInit,
		DiscoPartInit,
	} from "./types";

	type Props = {
		status?: DiscoConversationStatus;
		autoScroll?: boolean;
		chatWidth?: DiscoConversationChatWidth;
	};

	let {
		status = "ready",
		autoScroll = false,
		chatWidth = "full",
	}: Props = $props();

	type ScrollOptions = {
		behavior?: "auto" | "instant" | "smooth";
	};

	type ConversationHost = HTMLElement & {
		appendMessage?: (init: DiscoMessageInit) => DiscoMessageElement;
		replaceMessages?: (inits: DiscoMessageInit[]) => DiscoMessageElement[];
		clearMessages?: () => void;
		getMessages?: () => DiscoMessageInit[];
		getMessage?: (id: string) => DiscoMessageElement | null;
		appendPart?: (messageId: string, init: DiscoPartInit) => Element;
		scrollToBottom?: (options?: ScrollOptions) => void;
	};

	let viewport = $state<HTMLDivElement | null>(null);
	let content = $state<HTMLDivElement | null>(null);
	let isNearBottom = $state(true);
	let stickToBottom = $derived(autoScroll);
	let lastScrollDetail = "";

	function getHost(): ConversationHost | null {
		return viewport ? getCustomElementHost<ConversationHost>(viewport) : null;
	}

	function getScrollDetail() {
		const element = viewport;
		return {
			isNearBottom,
			stickToBottom,
			scrollTop: element?.scrollTop ?? 0,
			scrollHeight: element?.scrollHeight ?? 0,
			clientHeight: element?.clientHeight ?? 0,
		};
	}

	function updateScrollState() {
		const element = viewport;
		if (!element) {
			isNearBottom = true;
			return;
		}
		isNearBottom =
			element.scrollHeight - element.scrollTop - element.clientHeight <= 64;
		const detail = getScrollDetail();
		const serialized = JSON.stringify(detail);
		if (serialized !== lastScrollDetail) {
			lastScrollDetail = serialized;
			const host = getHost();
			if (host) {
				emitComposedEvent(host, "disco-scroll-state-change", detail);
			}
		}
	}

	function scrollToBottom(options: ScrollOptions = { behavior: "smooth" }) {
		const element = viewport;
		if (!element) {
			return;
		}
		element.scrollTo({
			top: element.scrollHeight,
			behavior: options.behavior ?? "smooth",
		});
		stickToBottom = true;
		void tick().then(updateScrollState);
	}

	function appendMessage(init: DiscoMessageInit): DiscoMessageElement {
		const host = getHost();
		if (!host) {
			throw new Error("disco-conversation is not mounted.");
		}
		const element = createMessageElement(init) as DiscoMessageElement;
		host.append(element);
		if (stickToBottom || autoScroll) {
			void tick().then(() => scrollToBottom({ behavior: "auto" }));
		}
		return element;
	}

	function replaceMessages(inits: DiscoMessageInit[]): DiscoMessageElement[] {
		const host = getHost();
		if (!host) {
			throw new Error("disco-conversation is not mounted.");
		}
		const elements = inits.map((init) =>
			createMessageElement(init),
		) as DiscoMessageElement[];
		host.replaceChildren(...elements);
		if (stickToBottom || autoScroll) {
			void tick().then(() => scrollToBottom({ behavior: "auto" }));
		}
		return elements;
	}

	function clearMessages() {
		getHost()?.replaceChildren();
		void tick().then(updateScrollState);
	}

	function getMessage(id: string): DiscoMessageElement | null {
		return (
			getHost()?.querySelector<DiscoMessageElement>(
				`disco-message#${CSS.escape(id)}`,
			) ?? null
		);
	}

	function appendPart(messageId: string, init: DiscoPartInit): Element {
		const message = getMessage(messageId);
		if (!message) {
			throw new Error(`No disco-message found for id "${messageId}".`);
		}
		const element = createPartElement(init);
		message.append(element);
		return element;
	}

	function getMessages(): DiscoMessageInit[] {
		const host = getHost();
		if (!host) {
			return [];
		}
		return Array.from(
			host.querySelectorAll<DiscoMessageElement>("disco-message"),
		)
			.map((message) => message.toMessageInit?.())
			.filter((message): message is DiscoMessageInit => Boolean(message));
	}

	function handleScroll() {
		const element = viewport;
		if (!element) {
			return;
		}
		updateScrollState();
		if (!isNearBottom) {
			stickToBottom = false;
		}
	}

	$effect(() => {
		const host = getHost();
		if (!host) {
			return;
		}
		host.appendMessage = appendMessage;
		host.replaceMessages = replaceMessages;
		host.clearMessages = clearMessages;
		host.getMessages = getMessages;
		host.getMessage = getMessage;
		host.appendPart = appendPart;
		host.scrollToBottom = scrollToBottom;
	});

	$effect(() => {
		const element = content;
		if (!element) {
			return;
		}
		const observer = new ResizeObserver(() => {
			if (stickToBottom || autoScroll) {
				scrollToBottom({ behavior: "auto" });
			} else {
				updateScrollState();
			}
		});
		observer.observe(element);
		return () => observer.disconnect();
	});
</script>

<div
	part="viewport"
	class="viewport"
	data-status={status}
	bind:this={viewport}
	onscroll={handleScroll}
>
	<div
		part="content"
		class:constrained={chatWidth === "constrained"}
		class="content"
		bind:this={content}
	>
		<slot></slot>
	</div>
	{#if !isNearBottom}
		<div class="scroll-button-wrap">
			<button
				part="scroll-button"
				class="scroll-button"
				type="button"
				aria-label="Scroll to bottom"
				onclick={() => scrollToBottom({ behavior: "smooth" })}
			>
				<ArrowDownIcon aria-hidden="true" class="icon" />
			</button>
		</div>
	{/if}
</div>

<style>
	:host {
		--disco-conversation-background: var(
			--disco-background,
			var(--background, #ffffff)
		);
		--disco-conversation-foreground: var(
			--disco-foreground,
			var(--foreground, #111827)
		);
		--disco-conversation-muted: var(--disco-muted, var(--muted, #f9fafb));
		--disco-conversation-muted-foreground: var(
			--disco-muted-foreground,
			var(--muted-foreground, #6b7280)
		);
		--disco-conversation-border: var(--disco-border, var(--border, #e5e7eb));
		--disco-conversation-card: var(--disco-card, var(--card, #ffffff));
		--disco-conversation-accent: var(--disco-accent, var(--accent, #f3f4f6));
		--disco-conversation-secondary: var(
			--disco-secondary,
			var(--secondary, #f3f4f6)
		);
		--disco-conversation-destructive: var(
			--disco-destructive,
			var(--destructive, #dc2626)
		);
		--disco-conversation-primary: var(--disco-primary, var(--primary, #2563eb));
		--disco-conversation-radius: var(--disco-radius, 0.75rem);
		--disco-conversation-font-sans: var(
			--disco-font-sans,
			var(--font-sans, system-ui, sans-serif)
		);
		--disco-conversation-font-mono: var(
			--disco-font-mono,
			var(
				--font-mono,
				ui-monospace,
				SFMono-Regular,
				Menlo,
				Monaco,
				Consolas,
				monospace
			)
		);
		--disco-conversation-padding: 1rem;
		--disco-conversation-gap: 1rem;
		--disco-conversation-max-width: 48rem;
		display: block;
		height: 100%;
		min-height: 0;
		background: var(--disco-conversation-background);
		color: var(--disco-conversation-foreground);
		font-family: var(--disco-conversation-font-sans);
	}

	.viewport {
		position: relative;
		height: 100%;
		min-width: 0;
		overflow: auto;
		padding: var(--disco-conversation-padding);
		scrollbar-gutter: stable;
	}

	.content {
		width: 100%;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: var(--disco-conversation-gap);
	}

	.content.constrained {
		max-width: var(--disco-conversation-max-width);
		margin-inline: auto;
	}

	::slotted(disco-turn),
	::slotted(disco-message) {
		display: block;
		min-width: 0;
	}

	.scroll-button-wrap {
		position: absolute;
		inset-inline: 0;
		bottom: 1rem;
		display: flex;
		justify-content: center;
		pointer-events: none;
	}

	.scroll-button {
		pointer-events: auto;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 9999px;
		border: 1px solid var(--disco-conversation-border);
		background: var(--disco-conversation-background);
		color: var(--disco-conversation-foreground);
		box-shadow: 0 1px 2px rgb(0 0 0 / 0.08);
		cursor: pointer;
	}

	.scroll-button:hover {
		background: var(--disco-conversation-accent);
	}

	:global(.icon) {
		width: 1rem;
		height: 1rem;
	}
</style>
