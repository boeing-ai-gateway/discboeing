<script lang="ts">
	import type { Snippet } from "svelte";
	import { DropdownMenuItem } from "$lib/components/ui/dropdown-menu";
	import { cn } from "$lib/utils";

	type Props = { class?: string; children?: Snippet };
	let { class: className, children, ...restProps }: Props = $props();
</script>

<DropdownMenuItem class={cn(className)} {...restProps}>
	{@render children?.()}
</DropdownMenuItem>
