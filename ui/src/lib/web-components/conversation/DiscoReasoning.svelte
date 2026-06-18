<svelte:options
	customElement={{
		tag: "disco-reasoning",
		props: {
			partId: { attribute: "part-id", type: "String" },
			state: { attribute: "state", type: "String" },
			open: { attribute: "open", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import { onMount, tick } from "svelte";
	import BrainIcon from "@lucide/svelte/icons/brain";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import Shimmer from "$lib/components/ai/shimmer.svelte";
	import "$lib/web-components/markdown/define";
	import {
		appendTextDelta,
		emitComposedEvent,
		getCustomElementHost,
	} from "./dom";
	import type { DiscoPartState } from "./types";

	type Props = {
		state?: DiscoPartState;
		partId?: string;
		open?: boolean;
	};

	const MAX_PREVIEW_LENGTH = 80;
	const MAX_PREVIEW_WORDS = 12;
	const SENTENCE_ENDING = /[.!?:…]$/;

	let {
		state: reasoningState = "complete",
		partId,
		open = false,
	}: Props = $props();
	type ReasoningHost = HTMLElement & {
		appendTextDelta?: (text: string) => void;
		open: boolean;
	};
	let root = $state<HTMLDivElement | null>(null);
	let sourceText = $state("");

	const isStreaming = $derived(reasoningState === "streaming");
	const previewText = $derived(extractPreviewText(sourceText));
	const triggerMessage = $derived(
		!isStreaming && previewText ? previewText : getThinkingMessage(isStreaming),
	);

	function getHost(): ReasoningHost | null {
		return root ? getCustomElementHost<ReasoningHost>(root) : null;
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

	function stripMarkdownHeader(line: string): string {
		return line
			.replace(/^#{1,6}\s+/, "")
			.replace(/^\*\*(.+)\*\*$/, "$1")
			.replace(/^__(.+)__$/, "$1")
			.replace(/^`(.+)`$/, "$1")
			.trim();
	}

	function extractPreviewText(text?: string): string | undefined {
		if (!text) {
			return undefined;
		}

		const lines = text.replace(/\r\n/g, "\n").split("\n");
		const firstNonEmptyLineIndex = lines.findIndex(
			(line) => line.trim().length > 0,
		);
		if (firstNonEmptyLineIndex === -1) {
			return undefined;
		}

		const firstLine = lines[firstNonEmptyLineIndex].trim();
		const nextLine = lines[firstNonEmptyLineIndex + 1]?.trim() ?? "";
		if (nextLine.length > 0) {
			return undefined;
		}

		const preview = stripMarkdownHeader(firstLine);
		if (!preview || preview.length > MAX_PREVIEW_LENGTH) {
			return undefined;
		}

		const wordCount = preview.split(/\s+/).filter(Boolean).length;
		if (wordCount === 0 || wordCount > MAX_PREVIEW_WORDS) {
			return undefined;
		}

		if (SENTENCE_ENDING.test(preview)) {
			return undefined;
		}

		return preview;
	}

	function getThinkingMessage(streaming: boolean): string {
		return streaming ? "Thinking..." : "Thought for a few seconds";
	}

	function toggle() {
		if (isStreaming) {
			return;
		}
		const host = getHost();
		if (!host) {
			return;
		}
		const nextOpen = !open;
		open = nextOpen;
		host.open = nextOpen;
		host.toggleAttribute("open", nextOpen);
		emitComposedEvent(host, "disco-expand-change", {
			messageId: host.closest("disco-message")?.id || undefined,
			partId,
			open: nextOpen,
		});
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

<div
	part="container"
	class="container"
	data-state={reasoningState}
	data-open={open}
	bind:this={root}
>
	<div part="header" class="header">
		{#if isStreaming}
			<div part="trigger" class="trigger">
				<BrainIcon class="icon" aria-hidden="true" />
				<span part="title" class="title">
					<Shimmer duration={1}>{triggerMessage}</Shimmer>
				</span>
			</div>
		{:else}
			<button
				part="trigger"
				class="trigger"
				type="button"
				aria-expanded={open}
				onclick={toggle}
			>
				<BrainIcon class="icon" aria-hidden="true" />
				<span part="title" class="title">{triggerMessage}</span>
			</button>
		{/if}

		{#if isStreaming}
			<div class="control-spacer" aria-hidden="true"></div>
		{:else}
			<button
				part="control"
				class="control"
				type="button"
				aria-label={open ? "Hide reasoning" : "Show reasoning"}
				aria-expanded={open}
				onclick={toggle}
			>
				<ChevronDownIcon
					class={open ? "chevron open" : "chevron"}
					aria-hidden="true"
				/>
			</button>
		{/if}
	</div>

	{#if open}
		<div part="content" class="content">
			<disco-markdown mode="block" is-animating={isStreaming}
				>{sourceText}</disco-markdown
			>
		</div>
	{/if}
	<span class="source"><slot></slot></span>
</div>

<style>
	:host {
		display: block;
		width: 100%;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-family: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
	}

	.container {
		width: 100%;
		min-width: 0;
		margin-bottom: var(--disco-reasoning-margin-bottom, 1rem);
		border-radius: 0.375rem;
	}

	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1rem 1rem 0;
	}

	.trigger {
		display: flex;
		min-width: 0;
		flex: 1 1 auto;
		align-items: center;
		gap: 0.5rem;
		border: 0;
		background: transparent;
		padding: 0;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font: inherit;
		text-align: left;
	}

	button.trigger {
		cursor: pointer;
	}

	button.trigger:hover {
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.icon) {
		width: 1rem;
		height: 1rem;
		flex: 0 0 auto;
		color: currentColor;
	}

	.title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 0.875rem;
		font-weight: 500;
		line-height: 1.25rem;
	}

	.control,
	.control-spacer {
		width: 1.75rem;
		height: 1.75rem;
		flex: 0 0 auto;
	}

	.control {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 0;
		border-radius: 0.375rem;
		background: transparent;
		padding: 0;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		opacity: 0;
		transition:
			opacity 150ms ease,
			background-color 150ms ease,
			color 150ms ease;
		cursor: pointer;
	}

	.container:hover .control,
	.container[data-open="true"] .control,
	.control:focus-visible {
		opacity: 1;
	}

	.control:hover {
		background: var(
			--disco-conversation-accent,
			var(--disco-accent, var(--accent, #f3f4f6))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.chevron) {
		width: 1rem;
		height: 1rem;
		transition: transform 150ms ease;
	}

	:global(.chevron.open) {
		transform: rotate(180deg);
	}

	.content {
		margin-top: 1rem;
		padding: 0 1rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.875rem;
		line-height: 1.25rem;
		outline: none;
		animation: disco-reasoning-open 150ms ease-out;
	}

	.source {
		display: none;
	}

	@keyframes disco-reasoning-open {
		from {
			opacity: 0;
			transform: translateY(-0.5rem);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
