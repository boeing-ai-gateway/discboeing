<svelte:options
	customElement={{
		tag: "disco-turn",
		props: {
			open: { attribute: "open", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import { emitComposedEvent, getCustomElementHost } from "./dom";

	type Props = {
		open?: boolean;
	};

	let { open = true }: Props = $props();
	type TurnHost = HTMLElement & { open: boolean };
	let root = $state<HTMLDivElement | null>(null);

	function getHost(): TurnHost | null {
		return root ? getCustomElementHost<TurnHost>(root) : null;
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
			turnId: host.id || undefined,
			open: nextOpen,
		});
	}
</script>

<div
	part="container"
	class="container"
	data-state={open ? "open" : "closed"}
	bind:this={root}
>
	{#if !open}
		<button
			part="trigger"
			class="trigger"
			type="button"
			aria-expanded={open}
			onclick={toggle}
		>
			<span part="state-line" class="state-line"></span>
			<span part="state-label" class="state-label">Closed turn</span>
			<span part="state-line" class="state-line"></span>
		</button>
	{/if}
	{#if open}
		<div part="content" class="content">
			<slot></slot>
		</div>
	{/if}
</div>

<style>
	:host {
		display: block;
		min-width: 0;
		color: var(--disco-foreground, #111827);
		font-family: var(--disco-font-sans, system-ui, sans-serif);
	}

	:host(:not(:first-child)) {
		padding-top: var(--disco-turn-spacing, 5rem);
	}

	.container {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 0.75rem;
	}

	.trigger {
		display: flex;
		width: 100%;
		align-items: center;
		gap: 0.5rem;
		border: 0;
		background: transparent;
		padding: 0;
		color: var(--disco-muted-foreground, #6b7280);
		font: inherit;
		font-size: 0.75rem;
		cursor: pointer;
	}

	.trigger:hover {
		color: var(--disco-foreground, #111827);
	}

	.state-line {
		height: 1px;
		flex: 1 1 auto;
		background: var(--disco-border, #e5e7eb);
	}

	.state-label {
		flex: 0 0 auto;
	}

	.content {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 1rem;
	}

	::slotted(disco-message) {
		display: block;
	}
</style>
