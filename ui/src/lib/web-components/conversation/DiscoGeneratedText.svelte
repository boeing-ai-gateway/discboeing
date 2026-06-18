<svelte:options
	customElement={{
		tag: "disco-generated-text",
		props: {
			partId: { attribute: "part-id", type: "String" },
			label: { attribute: "label", type: "String" },
			open: { attribute: "open", type: "Boolean" },
			contentOnly: { attribute: "content-only", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import { onMount, tick } from "svelte";
	import { emitComposedEvent, getCustomElementHost } from "./dom";
	import "$lib/web-components/markdown/define";

	type Props = {
		partId?: string;
		label?: string;
		open?: boolean;
		contentOnly?: boolean;
	};

	let {
		partId,
		label = "Generated text",
		open = false,
		contentOnly = false,
	}: Props = $props();
	type GeneratedTextHost = HTMLElement & { open: boolean };
	let root = $state<HTMLDivElement | null>(null);
	let sourceText = $state("");
	const contentOpen = $derived(open || contentOnly);
	const toggleLabel = $derived(
		`${contentOpen ? "Hide" : "Show"} ${label.toLowerCase()}`,
	);

	function getHost(): GeneratedTextHost | null {
		return root ? getCustomElementHost<GeneratedTextHost>(root) : null;
	}

	function refreshSourceText() {
		sourceText = getHost()?.textContent ?? "";
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
	data-open={contentOpen}
	data-content-only={contentOnly}
	bind:this={root}
>
	{#if !contentOnly}
		<button part="trigger" class="trigger" type="button" onclick={toggle}>
			{toggleLabel}
		</button>
	{/if}
	{#if contentOpen}
		<div part="content" class="content">
			<p part="label" class="label">{label}</p>
			<disco-markdown mode="block">{sourceText}</disco-markdown>
		</div>
	{/if}
	<span class="source"><slot></slot></span>
</div>

<style>
	:host {
		display: block;
		width: fit-content;
		max-width: 100%;
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
		display: flex;
		width: fit-content;
		max-width: 100%;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.25rem;
	}

	.trigger {
		border: 0;
		background: transparent;
		padding: 0;
		color: inherit;
		font: inherit;
		font-size: 0.6875rem;
		font-weight: 400;
		line-height: 1rem;
		text-align: left;
		cursor: pointer;
		transition: color 150ms ease;
	}

	.trigger:hover,
	.trigger:focus-visible {
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	.content {
		width: 100%;
		max-width: min(30rem, 100%);
		border: 1px solid
			color-mix(
				in oklab,
				var(
						--disco-conversation-border,
						var(--disco-border, var(--border, #e5e7eb))
					)
					60%,
				transparent
			);
		border-radius: 0.375rem;
		background: color-mix(
			in oklab,
			var(--disco-conversation-muted, var(--disco-muted, var(--muted, #f3f4f6)))
				30%,
			transparent
		);
		padding: 0.75rem;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		animation: disco-generated-text-open 150ms ease-out;
	}

	.label {
		margin: 0 0 0.5rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		letter-spacing: 0.14em;
		text-transform: uppercase;
	}

	.source {
		display: none;
	}

	@keyframes disco-generated-text-open {
		from {
			opacity: 0;
			transform: translateY(-0.25rem);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
