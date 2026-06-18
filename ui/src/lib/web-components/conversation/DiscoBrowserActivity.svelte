<svelte:options
	customElement={{
		tag: "disco-browser-activity",
		props: {
			partId: { attribute: "part-id", type: "String" },
			stepCount: { attribute: "step-count", type: "Number" },
			open: { attribute: "open", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import AppWindowIcon from "@lucide/svelte/icons/app-window";
	import {
		emitComposedEvent,
		getCustomElementHost,
		readJsonScript,
	} from "./dom";

	type Props = {
		partId?: string;
		title?: string;
		summary?: string;
		stepCount?: number;
		open?: boolean;
	};

	let {
		partId,
		title = "Browser activity",
		summary,
		stepCount,
		open = false,
	}: Props = $props();
	type BrowserActivityHost = HTMLElement & {
		data?: unknown;
		open: boolean;
	};
	let root = $state<HTMLDivElement | null>(null);
	const displaySummary = $derived(summary ?? getStepSummary(stepCount));

	function getHost(): BrowserActivityHost | null {
		return root ? getCustomElementHost<BrowserActivityHost>(root) : null;
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
			messageId: host.closest("disco-message")?.id || undefined,
			partId,
			open: nextOpen,
		});
	}

	function getStepSummary(value: number | undefined): string {
		if (value === undefined) {
			return "BROWSER ACTIVITY";
		}
		return value === 1 ? "1 BROWSER STEP" : `${value} BROWSER STEPS`;
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

<div part="container" class="container" bind:this={root}>
	<button
		part="header"
		class="header"
		type="button"
		aria-expanded={open}
		aria-label={`${open ? "Hide" : "Show"} ${title}`}
		onclick={toggle}
	>
		<span part="line" class="line"></span>
		<span part="pill" class="pill">
			<AppWindowIcon class="icon" aria-hidden="true" />
			<span part="summary" class="summary">{displaySummary}</span>
		</span>
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
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
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

	.header {
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

	.pill {
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
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.6875rem;
		font-weight: 500;
		letter-spacing: 0.14em;
		line-height: 1rem;
		text-transform: uppercase;
		transition:
			border-color 150ms ease,
			color 150ms ease;
	}

	.header:hover .pill {
		border-color: var(
			--disco-conversation-border,
			var(--disco-border, var(--border, #e5e7eb))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.icon) {
		width: 0.75rem;
		height: 0.75rem;
		flex: none;
	}

	.summary {
		white-space: nowrap;
	}

	.content {
		display: none;
		overflow: hidden;
		padding-top: 0.75rem;
	}

	.content.open {
		display: block;
		animation: disco-browser-activity-open 150ms ease-out;
	}

	@keyframes disco-browser-activity-open {
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
