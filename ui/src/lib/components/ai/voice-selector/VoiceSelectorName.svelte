<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";
	type Props = { class?: string; children?: Snippet };
	let { class: className, children, ...restProps }: Props = $props();
</script>

<span
	class={cn("flex-1 truncate text-left font-medium", className)}
	{...restProps}
>
	{@render children?.()}
</span>
