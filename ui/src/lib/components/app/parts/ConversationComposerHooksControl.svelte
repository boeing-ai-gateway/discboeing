<script lang="ts">
	import AlertTriangleIcon from "@lucide/svelte/icons/alert-triangle";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import PauseCircleIcon from "@lucide/svelte/icons/pause-circle";
	import ZapIcon from "@lucide/svelte/icons/zap";
	import type {
		HookRunStatus as ApiHookRunStatus,
		ThreadPhase,
	} from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import { getHookDisplayState } from "$lib/conversation-helpers";

	type ComposerHookRunStatus = Pick<
		ApiHookRunStatus,
		"hookId" | "lastResult" | "phase" | "executionPaused"
	>;

	type ComposerHooksStatus = {
		hooks: ComposerHookRunStatus[];
		pendingHookIds: string[];
		executionPaused: boolean;
	};

	type Props = {
		expanded?: boolean;
		hooksStatus: ComposerHooksStatus;
		threadPhase?: ThreadPhase | "";
		onExpandedChange?: (expanded: boolean) => void;
	};

	let {
		expanded = $bindable(false),
		hooksStatus,
		threadPhase = "",
		onExpandedChange,
	}: Props = $props();

	let hooks = $derived(hooksStatus.hooks);
	let pendingHookSet = $derived(new Set(hooksStatus.pendingHookIds));
	let hookDisplayStates = $derived(
		hooks.map((hook) => getHookDisplayState(hook, pendingHookSet)),
	);
	let hookPassedCount = $derived(
		hookDisplayStates.filter((state) => state === "success").length,
	);
	let hookHasRunning = $derived(
		hookDisplayStates.some((state) => state === "running"),
	);
	let hookHasFailures = $derived(
		hookDisplayStates.some((state) => state === "failure"),
	);
	let hookHasPausedExecution = $derived(
		hooks.some((hook) => hook.executionPaused),
	);
	let hasReviewPhaseHooks = $derived(
		hooks.some((hook) => hook.phase === "review"),
	);
</script>

{#if hooks.length > 0}
	<Button
		variant="ghost"
		size="xs"
		class="h-8 gap-1.5 px-2"
		onclick={() => {
			expanded = !expanded;
			onExpandedChange?.(expanded);
		}}
	>
		{#if hooksStatus.executionPaused || hookHasPausedExecution}
			<PauseCircleIcon class="size-3.5 text-amber-500" />
		{:else if hookHasRunning}
			<Loader2Icon class="size-3.5 animate-spin text-blue-500" />
		{:else if hookHasFailures}
			<AlertTriangleIcon class="size-3.5 text-yellow-500" />
		{:else}
			<ZapIcon class="size-3.5 text-green-500" />
		{/if}
		<span class="text-xs font-medium">{hookPassedCount}</span>
		{#if hasReviewPhaseHooks}
			<span class="text-xs text-muted-foreground">
				{threadPhase === "review" ? "Review" : "Draft"}
			</span>
		{/if}
	</Button>
{/if}
