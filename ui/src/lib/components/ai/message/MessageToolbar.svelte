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
	class={cn("mt-4 flex w-full items-center justify-between gap-4", className)}
	{...restProps}
>
	{@render children?.()}
</div>
