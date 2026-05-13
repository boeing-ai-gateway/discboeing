<script lang="ts">
	import type { Snippet } from "svelte";
	import { CommandList } from "$lib/components/ui/command";

	type Props = { children?: Snippet };

	let { children, ...restProps }: Props = $props();
</script>

<CommandList {...restProps}>
	{@render children?.()}
</CommandList>
