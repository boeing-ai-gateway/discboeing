<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
</script>

<div class={cn("space-y-2", className)} {...restProps}>
	<span
		class="font-medium text-muted-foreground text-xs uppercase tracking-wide"
		>Dependencies</span
	>
	<div class="space-y-1">
		{@render children?.()}
	</div>
</div>
