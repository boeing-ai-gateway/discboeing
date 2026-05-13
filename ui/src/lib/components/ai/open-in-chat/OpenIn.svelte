<script lang="ts">
	import type { Snippet } from "svelte";
	import { DropdownMenu } from "$lib/components/ui/dropdown-menu";
	import { setOpenInChatContext } from "./context";

	type Props = {
		query: string;
		children?: Snippet;
	};

	let { query, children, ...restProps }: Props = $props();
	const openIn = $state({ query: "" });
	$effect(() => {
		openIn.query = query;
	});
	setOpenInChatContext(openIn);
</script>

<DropdownMenu {...restProps}>
	{@render children?.()}
</DropdownMenu>
