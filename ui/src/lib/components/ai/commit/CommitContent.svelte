<script lang="ts">
	import type { Snippet } from "svelte";
	import { CollapsibleContent } from "$lib/components/ui/collapsible";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
</script>

<CollapsibleContent class={cn("border-t p-3", className)} {...restProps}>
	{@render children?.()}
</CollapsibleContent>
