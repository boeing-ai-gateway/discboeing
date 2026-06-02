<script lang="ts">
	import CircleCheckIcon from "@lucide/svelte/icons/circle-check";
	import CircleIcon from "@lucide/svelte/icons/circle";
	import GitCommitIcon from "@lucide/svelte/icons/git-commit";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import MessageCircleQuestionMarkIcon from "@lucide/svelte/icons/message-circle-question-mark";
	import type { SessionDisplayStatus } from "$lib/api-types";
	import { cn } from "$lib/utils";

	type Props = {
		status: SessionDisplayStatus;
		class?: string;
		iconClass?: string;
		labelClass?: string;
		showLabel?: boolean;
	};

	let {
		status,
		class: className,
		iconClass,
		labelClass,
		showLabel = true,
	}: Props = $props();

	const SPINNING_STATUSES = new Set<SessionDisplayStatus>([
		"running",
		"queued",
		"pending",
		"committing",
		"initializing",
		"reinitializing",
		"cloning",
		"pulling_image",
		"creating_sandbox",
		"removing",
	]);

	function normalizedStatus(
		status: SessionDisplayStatus,
	): SessionDisplayStatus {
		return status;
	}

	function statusTone(status: SessionDisplayStatus): string {
		switch (status) {
			case "error":
			case "create_failed":
				return "text-destructive";
			case "needs_attention":
				return "text-destructive";
			case "running":
				return "text-primary";
			case "queued":
				return "text-chart-5";
			case "idle":
			case "ready":
			case "completed":
			case "committed":
				return "text-diff-add-line";
			case "pending":
			case "committing":
			case "initializing":
			case "reinitializing":
			case "cloning":
			case "pulling_image":
			case "creating_sandbox":
				return "text-chart-5";
			case "removing":
				return "text-chart-3";
			default:
				return "text-muted-foreground";
		}
	}

	let label = $derived(
		status.replace(/_/g, " ").replace(/\b\w/g, (char) => char.toUpperCase()),
	);
	let tone = $derived(statusTone(status));
	let isSpinning = $derived(SPINNING_STATUSES.has(status));
</script>

<span
	class={cn("inline-flex items-center", showLabel && "gap-1.5", className)}
	title={label}
	aria-label={label}
>
	<span class={cn("inline-flex items-center", tone, iconClass)}>
		{#if isSpinning}
			<Loader2Icon class="size-3.5 animate-spin" />
		{:else if status === "needs_attention"}
			<MessageCircleQuestionMarkIcon class="size-3.5" />
		{:else if normalizedStatus(status) === "committed"}
			<GitCommitIcon class="size-3.5" />
		{:else if status === "ready" || status === "completed"}
			<CircleCheckIcon class="size-3.5" />
		{:else if status === "unknown"}
			<CircleIcon class="size-3.5" />
		{:else}
			<CircleIcon class="size-2.5 fill-current" />
		{/if}
	</span>
	{#if showLabel}
		<span class={cn("text-sm text-muted-foreground", labelClass)}>{label}</span>
	{/if}
</span>
