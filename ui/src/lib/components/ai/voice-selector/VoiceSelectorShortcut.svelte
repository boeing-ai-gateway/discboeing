<script lang="ts">
	import type { Snippet } from "svelte";
	import { CommandShortcut } from "$lib/components/ui/command";
	type Props = { children?: Snippet };
	let { children, ...restProps }: Props = $props();
</script>

<CommandShortcut {...restProps}>
	{@render children?.()}
</CommandShortcut>
