<script lang="ts">
	import type { Snippet } from "svelte";
	import { HoverCard } from "$lib/components/ui/hover-card";

	type Props = {
		openDelay?: number;
		closeDelay?: number;
		children?: Snippet;
	};

	let {
		openDelay = 0,
		closeDelay = 0,
		children,
		...restProps
	}: Props = $props();
</script>

<HoverCard {openDelay} {closeDelay} {...restProps}>
	{@render children?.()}
</HoverCard>
