<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = { class?: string; children?: Snippet };
	let { class: className, children, ...restProps }: Props = $props();
</script>

<div class={cn("contents", className)} {...restProps}>
	{@render children?.()}
</div>
