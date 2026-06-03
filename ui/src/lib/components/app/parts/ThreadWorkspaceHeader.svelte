<script lang="ts">
	import type { Snippet } from "svelte";
	import type { ThreadState } from "$lib/api-types";
	import ThreadStateBadge from "$lib/components/app/parts/ThreadStateBadge.svelte";

	type Props = {
		reserveSidebarSpace?: boolean;
		title: string;
		state?: ThreadState;
		titleContent?: Snippet;
	};

	let {
		reserveSidebarSpace = false,
		title,
		state,
		titleContent,
	}: Props = $props();
</script>

<header class="flex h-10 min-w-0 items-center gap-1 bg-background px-3">
	{#if reserveSidebarSpace}
		<div class="w-[10.75rem] shrink-0" aria-hidden="true"></div>
	{/if}
	<div class="flex min-w-0 items-center gap-2">
		{#if titleContent}
			{@render titleContent()}
		{:else}
			<p class="truncate text-sm font-medium">
				{title}
			</p>
		{/if}
		<ThreadStateBadge {state} class="px-2 text-[11px]" />
	</div>
</header>
