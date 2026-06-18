<svelte:options
	customElement={{
		tag: "disco-step-group",
		props: {
			open: { attribute: "open", type: "Boolean" },
			label: { attribute: "label", type: "String" },
		},
	}}
/>

<script lang="ts">
	import { emitComposedEvent, getCustomElementHost } from "./dom";

	type Props = {
		open?: boolean;
		label?: string;
	};

	let { open = false, label = "Steps" }: Props = $props();
	type StepGroupHost = HTMLElement & { open: boolean };
	let root = $state<HTMLDivElement | null>(null);

	function getHost(): StepGroupHost | null {
		return root ? getCustomElementHost<StepGroupHost>(root) : null;
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
			turnId: host.closest("disco-turn")?.id || undefined,
			open: nextOpen,
		});
	}
</script>

<div
	part="container"
	class="container"
	bind:this={root}
	data-state={open ? "open" : "closed"}
>
	<button
		part="trigger"
		class="trigger"
		type="button"
		aria-expanded={open}
		aria-label={`${open ? "Hide" : "Show"} ${label}`}
		onclick={toggle}
	>
		<span part="line" class="line"></span>
		<span part="label" class="label">{label}</span>
		<span part="line" class="line"></span>
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
	}

	.trigger {
		display: flex;
		width: 100%;
		align-items: center;
		gap: 0.75rem;
		border: 0;
		background: transparent;
		padding: 0.25rem 0;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.line {
		height: 1px;
		min-width: 0;
		flex: 1 1 auto;
		background: var(
			--disco-conversation-border,
			var(--disco-border, var(--border, #e5e7eb))
		);
	}

	.label {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		flex: 0 0 auto;
		border: 1px solid
			color-mix(
				in srgb,
				var(
						--disco-conversation-border,
						var(--disco-border, var(--border, #e5e7eb))
					)
					70%,
				transparent
			);
		border-radius: 9999px;
		background: var(
			--disco-conversation-background,
			var(--disco-background, var(--background, #ffffff))
		);
		padding: 0.25rem 0.75rem;
		font-size: 0.6875rem;
		font-weight: 500;
		letter-spacing: 0.14em;
		line-height: 1rem;
		text-transform: uppercase;
		transition:
			border-color 150ms ease,
			color 150ms ease;
	}

	.trigger:hover .label {
		border-color: var(
			--disco-conversation-border,
			var(--disco-border, var(--border, #e5e7eb))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	.content {
		display: none;
		min-width: 0;
		overflow: hidden;
		padding-top: 0;
	}

	.content.open {
		display: block;
		animation: disco-step-group-open 150ms ease-out;
	}

	::slotted(disco-message) {
		display: block;
	}

	@keyframes disco-step-group-open {
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
