<script lang="ts">
	import type { Snippet } from "svelte";
	import { Dialog } from "$lib/components/ui/dialog";

	type Props = {
		open?: boolean;
		children?: Snippet;
	};

	let { open = $bindable(false), children, ...restProps }: Props = $props();
</script>

<Dialog bind:open {...restProps}>
	{@render children?.()}
</Dialog>
