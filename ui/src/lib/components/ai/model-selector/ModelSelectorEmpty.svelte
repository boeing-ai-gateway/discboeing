<script lang="ts">
	import type { Snippet } from "svelte";
	import { CommandEmpty } from "$lib/components/ui/command";

	type Props = { children?: Snippet };

	let { children, ...restProps }: Props = $props();
</script>

<CommandEmpty {...restProps}>
	{@render children?.()}
</CommandEmpty>
