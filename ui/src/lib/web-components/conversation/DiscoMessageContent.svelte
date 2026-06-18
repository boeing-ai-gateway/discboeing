<svelte:options
	customElement={{
		tag: "disco-message-content",
		props: {
			format: { attribute: "format", type: "String" },
			partId: { attribute: "part-id", type: "String" },
		},
	}}
/>

<script lang="ts">
	import { onMount, tick } from "svelte";
	import "../markdown/define";
	import {
		appendTextDelta,
		emitComposedEvent,
		getCustomElementHost,
	} from "./dom";
	import type { MarkdownLinkClickDetail } from "../markdown";
	import type { DiscoContentFormat } from "./types";

	type Props = {
		format?: Exclude<DiscoContentFormat, "json">;
		partId?: string;
	};

	let { format = "text", partId }: Props = $props();
	type ContentHost = HTMLElement & {
		appendTextDelta?: (text: string) => void;
	};

	let root = $state<HTMLDivElement | null>(null);
	let sourceText = $state("");

	function getHost(): ContentHost | null {
		return root ? getCustomElementHost<ContentHost>(root) : null;
	}

	function refreshSourceText() {
		sourceText = getHost()?.textContent ?? "";
	}

	function appendDelta(text: string) {
		const host = getHost();
		if (!host) {
			return;
		}
		appendTextDelta(host, text);
		refreshSourceText();
	}

	function handleClick(event: MouseEvent) {
		const host = getHost();
		if (!host) {
			return;
		}
		const target = event.composedPath()[0];
		if (!(target instanceof HTMLAnchorElement)) {
			return;
		}
		const href = target.href;
		if (!href) {
			return;
		}
		const allowed = emitComposedEvent(
			host,
			"disco-link-open-request",
			{
				url: href,
				messageId: host.closest("disco-message")?.id || undefined,
				partId,
			},
			{ cancelable: true },
		);
		if (!allowed) {
			event.preventDefault();
		}
	}

	function handleMarkdownLinkClick(event: Event) {
		const host = getHost();
		if (!host) {
			return;
		}
		event.stopPropagation();
		const linkEvent = event as CustomEvent<MarkdownLinkClickDetail>;
		const allowed = emitComposedEvent(
			host,
			"disco-link-open-request",
			{
				url: linkEvent.detail.href,
				messageId: host.closest("disco-message")?.id || undefined,
				partId,
			},
			{ cancelable: true },
		);
		if (!allowed) {
			linkEvent.preventDefault();
		}
	}

	$effect(() => {
		const host = getHost();
		if (!host) {
			return;
		}
		host.appendTextDelta = appendDelta;
	});

	onMount(() => {
		const host = getHost();
		if (!host) {
			return;
		}
		refreshSourceText();
		const observer = new MutationObserver(() => {
			void tick().then(refreshSourceText);
		});
		observer.observe(host, {
			childList: true,
			characterData: true,
			subtree: true,
		});
		return () => observer.disconnect();
	});
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div
	part="container"
	bind:this={root}
	class="container"
	data-format={format}
	onclick={handleClick}
>
	{#if format === "markdown"}
		<div part="markdown" class="markdown-rendered">
			<disco-markdown
				class="streamdown"
				markdown={sourceText}
				mode="streaming"
				ondisco-link-click={handleMarkdownLinkClick}
				>{sourceText}</disco-markdown
			>
		</div>
	{:else if format === "text"}
		<div part="text" class="plain-text">{sourceText}</div>
	{/if}
	<div class="source">
		<slot></slot>
	</div>
</div>

<style>
	:host {
		--disco-message-content-foreground: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		--disco-message-content-font-sans: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
		--disco-message-content-font-mono: var(
			--disco-conversation-font-mono,
			var(
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
			)
		);
		--disco-message-content-link: var(
			--disco-conversation-primary,
			var(--disco-primary, var(--primary, #2563eb))
		);
		--disco-background: var(
			--disco-conversation-background,
			var(--background, #ffffff)
		);
		--disco-foreground: var(--disco-message-content-foreground);
		--disco-muted: var(--disco-conversation-muted, var(--muted, #f3f4f6));
		--disco-muted-foreground: var(
			--disco-conversation-muted-foreground,
			var(--muted-foreground, #6b7280)
		);
		--disco-border: var(--disco-conversation-border, var(--border, #e5e7eb));
		--disco-primary: var(--disco-message-content-link);
		--disco-font-sans: var(--disco-message-content-font-sans);
		--disco-font-mono: var(--disco-message-content-font-mono);
		display: block;
		width: var(--disco-message-content-width, 100%);
		max-width: var(--disco-message-content-max-width, 100%);
		min-width: 0;
		border-radius: var(--disco-message-content-radius, 0);
		background: var(--disco-message-content-background, transparent);
		color: var(--disco-message-content-foreground);
		font-family: var(--disco-message-content-font-sans);
	}

	.container {
		width: 100%;
		min-width: 0;
		padding: var(--disco-message-content-padding, 0);
		border-radius: inherit;
		overflow-wrap: anywhere;
	}

	.markdown-rendered,
	.plain-text,
	.markdown-rendered,
	.plain-text {
		width: 100%;
		min-width: 0;
	}

	.plain-text {
		white-space: pre-wrap;
	}

	.source {
		display: none;
	}

	:global(.streamdown > *:first-child) {
		margin-top: 0;
	}

	:global(.streamdown > *:last-child) {
		margin-bottom: 0;
	}
</style>
