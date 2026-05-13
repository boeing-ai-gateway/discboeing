<script lang="ts">
	import type { Snippet } from "svelte";
	import { HoverCardContent } from "$lib/components/ui/hover-card";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
</script>

<HoverCardContent
	align="start"
	class={cn("w-auto p-2", className)}
	{...restProps}
>
	{@render children?.()}
</HoverCardContent>
