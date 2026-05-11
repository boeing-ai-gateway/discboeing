<script lang="ts">
	import CircleCheckIcon from "@lucide/svelte/icons/circle-check";
	import CircleIcon from "@lucide/svelte/icons/circle";
	import GitCommitIcon from "@lucide/svelte/icons/git-commit";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import MessageCircleQuestionMarkIcon from "@lucide/svelte/icons/message-circle-question-mark";
	import type {
		SessionActivityStatusValue,
		SessionStatusValue,
	} from "$lib/shell-types";
	import { cn } from "$lib/utils";

	type DisplayStatusValue = SessionStatusValue | SessionActivityStatusValue;

	type Props = {
		status: DisplayStatusValue;
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

	function normalizedStatus(status: DisplayStatusValue): string {
		return status.toLowerCase();
	}

	function statusLabel(status: DisplayStatusValue): string {
		return status
			.replace(/_/g, " ")
			.replace(/\b\w/g, (char) => char.toUpperCase());
	}

	function statusTone(status: DisplayStatusValue): string {
		switch (normalizedStatus(status)) {
			case "error":
			case "create_failed":
				return "text-destructive";
			case "needs_attention":
				return "text-amber-500";
			case "running":
				return "text-blue-500";
			case "queued":
			case "unknown":
				return "text-yellow-500";
			case "ready":
			case "completed":
			case "committed":
				return "text-green-500";
			case "pending":
			case "committing":
			case "initializing":
			case "reinitializing":
			case "cloning":
			case "pulling_image":
			case "creating_sandbox":
				return "text-yellow-500";
			case "removing":
				return "text-orange-500";
			default:
				return "text-muted-foreground";
		}
	}

	function isSpinningStatus(status: DisplayStatusValue): boolean {
		switch (normalizedStatus(status)) {
			case "running":
			case "queued":
			case "unknown":
			case "pending":
			case "committing":
			case "initializing":
			case "reinitializing":
			case "cloning":
			case "pulling_image":
			case "creating_sandbox":
			case "removing":
				return true;
			default:
				return false;
		}
	}
</script>

<span
	class={cn("inline-flex items-center", showLabel && "gap-1.5", className)}
	title={statusLabel(status)}
	aria-label={statusLabel(status)}
>
	<span class={cn("inline-flex items-center", statusTone(status), iconClass)}>
		{#if isSpinningStatus(status)}
			<Loader2Icon class="size-3.5 animate-spin" />
		{:else if normalizedStatus(status) === "needs_attention"}
			<MessageCircleQuestionMarkIcon class="size-3.5" />
		{:else if normalizedStatus(status) === "committed"}
			<GitCommitIcon class="size-3.5" />
		{:else if ["ready", "completed"].includes(normalizedStatus(status))}
			<CircleCheckIcon class="size-3.5" />
		{:else}
			<CircleIcon class="size-2.5 fill-current" />
		{/if}
	</span>
	{#if showLabel}
		<span class={cn("text-sm text-muted-foreground", labelClass)}
			>{statusLabel(status)}</span
		>
	{/if}
</span>
