<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = { class?: string; children?: Snippet };
	let { class: className, children, ...restProps }: Props = $props();
</script>

<h3 class={cn("font-medium text-sm", className)} {...restProps}>
	{#if children}
		{@render children()}
	{:else}
		Environment Variables
	{/if}
</h3>
