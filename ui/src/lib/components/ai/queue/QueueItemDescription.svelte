<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = {
		completed?: boolean;
		class?: string;
		children?: Snippet;
	};

	let {
		completed = false,
		class: className,
		children,
		...restProps
	}: Props = $props();
</script>

<div
	class={cn(
		"ml-6 text-xs",
		completed
			? "text-muted-foreground/40 line-through"
			: "text-muted-foreground",
		className,
	)}
	{...restProps}
>
	{@render children?.()}
</div>
