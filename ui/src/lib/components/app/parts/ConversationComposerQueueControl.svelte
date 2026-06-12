<script lang="ts">
	import CheckCircleIcon from "@lucide/svelte/icons/check-circle";
	import { Button } from "$lib/components/ui/button";
	import type { PlanEntry } from "$lib/plan-entry";

	type Props = {
		expanded?: boolean;
		entries: PlanEntry[];
	};

	let { expanded = $bindable(false), entries }: Props = $props();

	let queueCompletedCount = $derived(
		entries.filter((entry) => entry.status === "completed").length,
	);
	let queueTotalCount = $derived(entries.length);
	let queueToggleLabel = $derived(
		`Toggle queued plan entries, ${queueCompletedCount} of ${queueTotalCount} complete`,
	);
</script>

{#if queueTotalCount > 0}
	<Button
		variant="ghost"
		size="xs"
		class="h-8 gap-1.5 px-2"
		aria-label={queueToggleLabel}
		aria-expanded={expanded}
		onclick={() => {
			expanded = !expanded;
		}}
	>
		<CheckCircleIcon class="size-3.5" aria-hidden="true" />
		<span class="text-xs font-medium"
			>{queueCompletedCount}/{queueTotalCount}</span
		>
	</Button>
{/if}
