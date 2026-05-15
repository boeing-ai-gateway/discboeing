<script lang="ts">
	import type { Snippet } from "svelte";
	import type { ComponentProps } from "svelte";
	import { HoverCard } from "$lib/components/ui/hover-card";

	type Props = ComponentProps<typeof HoverCard> & {
		children?: Snippet;
	};

	let { children, ...restProps }: Props = $props();
</script>

<HoverCard closeDelay={0} openDelay={0} {...restProps}>
	{@render children?.()}
</HoverCard>
