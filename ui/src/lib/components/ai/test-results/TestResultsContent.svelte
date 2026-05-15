<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = { class?: string; children?: Snippet };
	let { class: className, children, ...restProps }: Props = $props();
</script>

<div class={cn("space-y-2 p-4", className)} {...restProps}>
	{@render children?.()}
</div>
