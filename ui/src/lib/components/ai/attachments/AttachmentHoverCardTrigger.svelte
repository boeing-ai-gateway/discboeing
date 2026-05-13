<script lang="ts">
	import type { Snippet } from "svelte";
	import { HoverCardTrigger } from "$lib/components/ui/hover-card";

	type Props = {
		children?: Snippet;
	};

	let { children, ...restProps }: Props = $props();
</script>

<HoverCardTrigger {...restProps}>
	{@render children?.()}
</HoverCardTrigger>
