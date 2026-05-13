<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";
	import { useStackTraceContext } from "./context";

	type Props = {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
	const stackTrace = useStackTraceContext();
</script>

<span class={cn("truncate text-foreground", className)} {...restProps}>
	{#if children}
		{@render children()}
	{:else}
		{stackTrace.trace.errorMessage}
	{/if}
</span>
