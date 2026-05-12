<script lang="ts">
	import type { ThreadState } from "$lib/api-types";
	import {
		getThreadStateLabel,
		getThreadStateTone,
	} from "$lib/app/thread-status";
	import { cn } from "$lib/utils";

	type Props = {
		state?: ThreadState;
		class?: string;
	};

	let { state, class: className }: Props = $props();
	const label = $derived(getThreadStateLabel(state));
</script>

{#if label}
	<span
		class={cn(
			"inline-flex shrink-0 items-center rounded-full border px-1.5 py-0.5 text-[10px] font-medium",
			getThreadStateTone(state),
			className,
		)}
	>
		{label}
	</span>
{/if}
