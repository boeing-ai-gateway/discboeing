<svelte:options
	customElement={{
		tag: "disco-event",
		props: {
			partId: { attribute: "part-id", type: "String" },
			open: { attribute: "open", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import {
		emitComposedEvent,
		getCustomElementHost,
		readJsonScript,
	} from "./dom";

	type Props = {
		partId?: string;
		kind?: string;
		title?: string;
		summary?: string;
		open?: boolean;
	};

	let {
		partId,
		kind = "event",
		title,
		summary,
		open = false,
	}: Props = $props();
	type EventHost = HTMLElement & { data?: unknown; open: boolean };
	let root = $state<HTMLDivElement | null>(null);
	const displayTitle = $derived(title || kind);

	function getHost(): EventHost | null {
		return root ? getCustomElementHost<EventHost>(root) : null;
	}

	function toggle() {
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
		const metadata = host.querySelector("disco-metadata");
		host.data = metadata ? readJsonScript(metadata) : undefined;
	});
</script>

<div part="container" class="container" data-kind={kind} bind:this={root}>
	<button
		part="header"
		class="header"
		type="button"
		aria-expanded={open}
		onclick={toggle}
	>
		<ChevronRightIcon
			class={open ? "chevron open" : "chevron"}
			aria-hidden="true"
		/>
		<span class="title">{displayTitle}</span>
		{#if summary}
			<span class="summary">{summary}</span>
		{/if}
	</button>
	<div part="content" class:open class="content">
		<slot></slot>
	</div>
</div>

<style>
	:host {
		display: block;
		width: 100%;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-family: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
	}

	.container {
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: 0.75rem;
		background: var(
			--disco-conversation-card,
			var(--disco-card, var(--card, #fff))
		);
		overflow: hidden;
	}

	.header {
		display: flex;
		width: 100%;
		align-items: center;
		gap: 0.5rem;
		border: 0;
		background: var(
			--disco-conversation-background,
			var(--disco-background, var(--background, #fff))
		);
		padding: 0.75rem 1rem;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	:global(.chevron) {
		width: 1rem;
		height: 1rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		transition: transform 120ms ease;
	}

	:global(.chevron.open) {
		transform: rotate(90deg);
	}

	.title {
		font-size: 0.875rem;
		font-weight: 500;
	}

	.summary {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.8125rem;
	}

	.content {
		display: none;
		border-top: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		padding: 0.75rem 1rem;
	}

	.content.open {
		display: block;
	}
</style>
