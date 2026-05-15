<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
</script>

<div
	class={cn(
		"flex items-center justify-between border-zinc-800 border-b px-4 py-2",
		className,
	)}
	{...restProps}
>
	{@render children?.()}
</div>
