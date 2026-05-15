<script lang="ts">
	import type { Snippet } from "svelte";
	import { CommandDialog } from "$lib/components/ui/command";
	type Props = { children?: Snippet };
	let { children, ...restProps }: Props = $props();
</script>

<CommandDialog {...restProps}>
	{@render children?.()}
</CommandDialog>
