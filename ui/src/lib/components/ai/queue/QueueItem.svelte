<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
</script>

<li
	class={cn(
		"group flex flex-col gap-1 rounded-md px-3 py-1 text-sm transition-colors hover:bg-muted",
		className,
	)}
	{...restProps}
>
	{@render children?.()}
</li>
