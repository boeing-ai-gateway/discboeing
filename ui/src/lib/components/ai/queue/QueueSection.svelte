<script lang="ts">
	import type { Snippet } from "svelte";
	import { Collapsible } from "$lib/components/ui/collapsible";
	import { cn } from "$lib/utils";

	type Props = {
		open?: boolean;
		defaultOpen?: boolean;
		class?: string;
		children?: Snippet;
	};

	let {
		defaultOpen = true,
		open = $bindable(defaultOpen),
		class: className,
		children,
		...restProps
	}: Props = $props();
</script>

<Collapsible bind:open class={cn(className)} {...restProps}>
	{@render children?.()}
</Collapsible>
