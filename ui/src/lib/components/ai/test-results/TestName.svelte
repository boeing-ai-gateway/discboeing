<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";
	import { useTestContext } from "./context";

	type Props = { class?: string; children?: Snippet };
	let { class: className, children, ...restProps }: Props = $props();
	const test = useTestContext();
</script>

<span class={cn("flex-1", className)} {...restProps}>
	{#if children}
		{@render children()}
	{:else}
		{test.name}
	{/if}
</span>
