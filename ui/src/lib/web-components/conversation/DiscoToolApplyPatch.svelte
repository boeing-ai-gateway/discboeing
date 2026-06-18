<svelte:options
	customElement={{
		tag: "disco-tool-apply-patch",
		props: {
			partId: { attribute: "part-id", type: "String" },
			callId: { attribute: "call-id", type: "String" },
			state: { attribute: "state", type: "String" },
			title: { attribute: "title", type: "String" },
			input: { attribute: "input", type: "String" },
			output: { attribute: "output", type: "String" },
			errorText: { attribute: "error-text", type: "String" },
			defaultOpen: { attribute: "default-open", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import OptimizedToolView from "./OptimizedToolView.svelte";

	type Props = {
		partId?: string;
		callId?: string;
		state?: string;
		title?: string;
		input?: string;
		output?: string;
		errorText?: string;
		defaultOpen?: boolean;
	};

	let {
		partId,
		callId,
		state = "input-available",
		title,
		input = "",
		output = "",
		errorText = "",
		defaultOpen = false,
	}: Props = $props();
</script>

<OptimizedToolView
	toolKind="apply_patch"
	{partId}
	{callId}
	{state}
	{title}
	{input}
	{output}
	{errorText}
	{defaultOpen}
/>

<style>
	:host {
		display: block;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-family: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
	}

	:global(.container) {
		display: flex;
		width: 100%;
		min-width: 0;
		flex-direction: column;
		gap: 0;
		margin-bottom: 1rem;
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.25rem);
	}

	:global(.header) {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		min-width: 0;
		padding: 1rem 1rem 0;
	}

	:global(.trigger) {
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
		cursor: pointer;
	}

	:global(.trigger:hover) {
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.title) {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 0.875rem;
		font-weight: 500;
		line-height: 1.25rem;
	}

	:global(.status) {
		display: inline-flex;
		flex: 0 0 auto;
		align-items: center;
		gap: 0.25rem;
		border-radius: 999px;
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
		padding: 0.125rem 0.5rem;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-size: 0.75rem;
		font-weight: 500;
		line-height: 1rem;
	}

	:global(.status.success) {
		background: color-mix(in oklab, #16a34a 12%, transparent);
		color: #15803d;
	}

	:global(.status.running) {
		background: color-mix(in oklab, #2563eb 12%, transparent);
		color: #1d4ed8;
	}

	:global(.status.approval) {
		background: color-mix(in oklab, #d97706 14%, transparent);
		color: #b45309;
	}

	:global(.status.error) {
		background: color-mix(in oklab, #dc2626 12%, transparent);
		color: #b91c1c;
	}

	:global(.raw-toggle) {
		display: inline-flex;
		width: 1.75rem;
		height: 1.75rem;
		align-items: center;
		justify-content: center;
		border: 0;
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.375rem);
		background: transparent;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		cursor: pointer;
	}

	:global(.raw-toggle:hover),
	:global(.raw-toggle:focus-visible),
	:global(.raw-toggle.active) {
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.chevron),
	:global(.tool-icon),
	:global(.status-icon),
	:global(.raw-icon),
	:global(.copy-icon) {
		width: 1rem;
		height: 1rem;
		flex: 0 0 auto;
	}

	:global(.chevron) {
		transition: transform 150ms ease;
	}

	:global(.chevron.open) {
		transform: rotate(180deg);
	}

	:global(.tool-icon) {
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
	}

	:global(.status-icon.spinning) {
		animation: disco-tool-spin 1s linear infinite;
	}

	:global(.content) {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		margin: 0;
		padding: 0.75rem 1rem 1rem;
	}

	:global(.summary) {
		display: grid;
		gap: 0.5rem;
	}

	:global(.summary-row) {
		display: flex;
		min-width: 0;
		align-items: baseline;
		gap: 0.5rem;
		font-size: 0.875rem;
		line-height: 1.25rem;
	}

	:global(.summary-label) {
		flex: 0 0 auto;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.75rem;
	}

	:global(code),
	:global(.body),
	:global(.raw) {
		font-family: var(
			--disco-conversation-font-mono,
			var(--disco-font-mono, var(--font-mono, monospace))
		);
	}

	:global(code) {
		min-width: 0;
		overflow-wrap: anywhere;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-size: 0.8125rem;
		white-space: pre-wrap;
	}

	:global(.copy-button) {
		display: inline-flex;
		height: 1.5rem;
		align-items: center;
		justify-content: center;
		gap: 0.25rem;
		border: 0;
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.5rem);
		background: transparent;
		padding: 0 0.375rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-family: var(
			--disco-conversation-font-sans,
			var(--disco-font-sans, var(--font-sans, system-ui, sans-serif))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		cursor: pointer;
	}

	:global(.copy-button:hover),
	:global(.copy-button:focus-visible) {
		background: var(
			--disco-conversation-background,
			var(--disco-background, var(--background, #fff))
		);
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
	}

	:global(.copy-button.inline) {
		width: 1.5rem;
		flex: 0 0 auto;
		padding: 0;
		opacity: 0;
		transition: opacity 120ms ease;
	}

	:global(.summary-row:hover .copy-button.inline),
	:global(.copy-button.inline:focus-visible) {
		opacity: 1;
	}

	:global(.raw-panel),
	:global(.body-panel) {
		overflow: hidden;
		border: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.375rem);
		background: var(
			--disco-conversation-muted,
			var(--disco-muted, var(--muted, #f3f4f6))
		);
	}

	:global(.raw-panel) {
		margin: 0.75rem 1rem 1rem;
	}

	:global(.panel-header),
	:global(.body-header) {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		border-bottom: 1px solid
			var(
				--disco-conversation-border,
				var(--disco-border, var(--border, #e5e7eb))
			);
		padding: 0.5rem 0.75rem;
		color: var(
			--disco-conversation-muted-foreground,
			var(--disco-muted-foreground, var(--muted-foreground, #6b7280))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	:global(.panel-actions) {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
	}

	:global(.raw),
	:global(.body) {
		max-height: 24rem;
		overflow: auto;
		margin: 0;
		padding: 0.75rem;
		border: 0;
		background: transparent;
		color: var(
			--disco-conversation-foreground,
			var(--disco-foreground, var(--foreground, #111827))
		);
		font-size: 0.75rem;
		line-height: 1rem;
		white-space: pre-wrap;
	}

	:global(.error) {
		border: 1px solid color-mix(in oklab, #dc2626 35%, transparent);
		border-radius: calc(var(--disco-radius, 0.75rem) - 0.375rem);
		background: color-mix(in oklab, #dc2626 10%, transparent);
		padding: 0.75rem;
		color: #dc2626;
		font-size: 0.875rem;
		line-height: 1.25rem;
	}

	:global(.sr-only) {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}

	@keyframes disco-tool-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
