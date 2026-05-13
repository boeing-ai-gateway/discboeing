<script lang="ts">
	import type { Snippet } from "svelte";
	import { DialogTrigger } from "$lib/components/ui/dialog";

	type Props = { children?: Snippet };
	let { children, ...restProps }: Props = $props();
</script>

<DialogTrigger {...restProps}>
	{@render children?.()}
</DialogTrigger>
