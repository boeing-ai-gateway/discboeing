<script lang="ts">
	import type { Snippet } from "svelte";
	import { DropdownMenu } from "$lib/components/ui/dropdown-menu";

	type Props = { children?: Snippet };
	let { children, ...restProps }: Props = $props();
</script>

<DropdownMenu {...restProps}>
	{@render children?.()}
</DropdownMenu>
