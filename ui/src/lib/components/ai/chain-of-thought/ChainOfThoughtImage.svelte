<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = {
		caption?: string;
		class?: string;
		children?: Snippet;
	};

	let { caption, class: className, children, ...restProps }: Props = $props();
</script>

<div class={cn("mt-2 space-y-2", className)} {...restProps}>
	<div
		class="relative flex max-h-[22rem] items-center justify-center overflow-hidden rounded-lg bg-muted p-3"
	>
		{@render children?.()}
	</div>
	{#if caption}
		<p class="text-muted-foreground text-xs">{caption}</p>
	{/if}
</div>
