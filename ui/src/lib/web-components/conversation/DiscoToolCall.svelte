<svelte:options
	customElement={{
		tag: "disco-tool-call",
		props: {
			partId: { attribute: "part-id", type: "String" },
			callId: { attribute: "call-id", type: "String" },
			name: { attribute: "name", type: "String" },
			state: { attribute: "state", type: "String" },
			title: { attribute: "title", type: "String" },
			approvalId: { attribute: "approval-id", type: "String" },
			open: { attribute: "open", type: "Boolean" },
		},
	}}
/>

<script lang="ts">
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import CircleIcon from "@lucide/svelte/icons/circle";
	import {
		emitComposedEvent,
		getCustomElementHost,
		readJsonScript,
		writeJsonScript,
	} from "./dom";
	import type { DiscoToolApprovalResponse, DiscoToolState } from "./types";

	type Props = {
		partId?: string;
		callId?: string;
		name?: string;
		state?: DiscoToolState;
		title?: string;
		approvalId?: string;
		open?: boolean;
	};

	let {
		partId,
		callId = "",
		name = "Tool",
		state: toolState = "input-available",
		title,
		approvalId,
		open = false,
	}: Props = $props();
	type ToolCallHost = HTMLElement & {
		respond?: (response: DiscoToolApprovalResponse) => void;
		setInput?: (value: unknown) => void;
		setOutput?: (value: unknown) => void;
		open: boolean;
	};
	let root = $state<HTMLDivElement | null>(null);
	const displayTitle = $derived(title || name);
	const statusLabel = $derived(getStatusLabel(toolState));

	function getHost(): ToolCallHost | null {
		return root ? getCustomElementHost<ToolCallHost>(root) : null;
	}

	function requireHost(): ToolCallHost {
		const host = getHost();
		if (!host) {
			throw new Error("disco-tool-call is not mounted.");
		}
		return host;
	}

	function getInputValue() {
		const host = getHost();
		const input = host?.querySelector("disco-tool-input");
		return input ? readJsonScript(input) : undefined;
	}

	function respond(response: DiscoToolApprovalResponse) {
		const host = requireHost();
		if (response.approved) {
			host.setAttribute("state", "approval-responded");
			host.setAttribute("approved", "true");
		} else {
			host.setAttribute("state", "output-denied");
			host.setAttribute("approved", "false");
		}
		if (response.reason) {
			host.setAttribute("reason", response.reason);
		}
		emitComposedEvent(host, "disco-tool-approval-response", {
			messageId: host.closest("disco-message")?.id || undefined,
			partId,
			callId,
			name,
			approvalId: response.approvalId ?? approvalId,
			input: getInputValue(),
			...response,
		});
	}

	function requestApproval() {
		const host = requireHost();
		emitComposedEvent(
			host,
			"disco-tool-approval-request",
			{
				messageId: host.closest("disco-message")?.id || undefined,
				partId,
				callId,
				name,
				approvalId,
				input: getInputValue(),
			},
			{ cancelable: true },
		);
	}

	function setInput(value: unknown) {
		const host = requireHost();
		let input = host.querySelector("disco-tool-input");
		if (!input) {
			input = document.createElement("disco-tool-input");
			input.setAttribute("format", "json");
			host.prepend(input);
		}
		writeJsonScript(input, value);
	}

	function setOutput(value: unknown) {
		const host = requireHost();
		let output = host.querySelector("disco-tool-output");
		if (!output) {
			output = document.createElement("disco-tool-output");
			output.setAttribute("format", "json");
			host.append(output);
		}
		writeJsonScript(output, value);
	}

	function toggle() {
		const host = requireHost();
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

	function getStatusLabel(value: DiscoToolState): string {
		switch (value) {
			case "input-streaming":
				return "Preparing";
			case "input-available":
				return "Ready";
			case "approval-requested":
				return "Approval requested";
			case "approval-responded":
				return "Approved";
			case "output-available":
				return "Completed";
			case "output-error":
				return "Error";
			case "output-denied":
				return "Denied";
		}
	}

	$effect(() => {
		const host = getHost();
		if (!host) {
			return;
		}
		host.respond = respond;
		host.setInput = setInput;
		host.setOutput = setOutput;
	});
</script>

<div part="container" class="container" data-state={toolState} bind:this={root}>
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
		<CircleIcon class="status-dot" aria-hidden="true" />
		<span part="title" class="title">{displayTitle}</span>
		<span part="status" class="status">{statusLabel}</span>
	</button>
	{#if toolState === "approval-requested"}
		<div part="actions" class="actions">
			<button class="action" type="button" onclick={requestApproval}
				>Review</button
			>
			<button
				class="action primary"
				type="button"
				onclick={() => respond({ approvalId, approved: true })}
			>
				Approve
			</button>
			<button
				class="action"
				type="button"
				onclick={() => respond({ approvalId, approved: false })}
			>
				Deny
			</button>
		</div>
	{/if}
	<div part="content" class:open class="content">
		<slot></slot>
	</div>
</div>

<style>
	:host {
		display: block;
		width: 100%;
		min-width: 0;
		color: var(--disco-foreground, #111827);
		font-family: var(--disco-font-sans, system-ui, sans-serif);
	}

	.container {
		width: 100%;
		min-width: 0;
		border: 1px solid var(--disco-border, #e5e7eb);
		border-radius: 0.5rem;
		background: var(--disco-card, #fff);
		overflow: hidden;
	}

	.header {
		display: flex;
		width: 100%;
		align-items: center;
		gap: 0.5rem;
		border: 0;
		background: color-mix(
			in srgb,
			var(--disco-muted, #f9fafb) 45%,
			transparent
		);
		padding: 0.625rem 0.75rem;
		color: inherit;
		text-align: left;
		cursor: pointer;
	}

	.header:hover {
		background: var(--disco-muted, #f9fafb);
	}

	:global(.chevron) {
		width: 1rem;
		height: 1rem;
		color: var(--disco-muted-foreground, #6b7280);
		transition: transform 120ms ease;
	}

	:global(.chevron.open) {
		transform: rotate(90deg);
	}

	:global(.status-dot) {
		width: 0.5rem;
		height: 0.5rem;
		fill: currentColor;
		color: var(--disco-muted-foreground, #6b7280);
	}

	:host([state="output-available"]) :global(.status-dot),
	:host([state="approval-responded"]) :global(.status-dot) {
		color: #22c55e;
	}

	:host([state="output-error"]) :global(.status-dot),
	:host([state="output-denied"]) :global(.status-dot) {
		color: var(--disco-destructive, #dc2626);
	}

	.title {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 500;
		font-size: 0.875rem;
	}

	.status {
		flex: none;
		color: var(--disco-muted-foreground, #6b7280);
		font-size: 0.75rem;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		border-top: 1px solid var(--disco-border, #e5e7eb);
		padding: 0.75rem 1rem;
	}

	.action {
		border: 1px solid var(--disco-border, #e5e7eb);
		border-radius: 0.375rem;
		background: var(--disco-background, #fff);
		padding: 0.375rem 0.75rem;
		color: var(--disco-foreground, #111827);
		font: inherit;
		font-size: 0.8125rem;
		cursor: pointer;
	}

	.action.primary {
		background: var(--disco-foreground, #111827);
		color: var(--disco-background, #fff);
	}

	.content {
		display: none;
		min-width: 0;
		flex-direction: column;
		gap: 0.75rem;
		border-top: 1px solid var(--disco-border, #e5e7eb);
		padding: 0.75rem;
	}

	.content.open {
		display: flex;
	}
</style>
