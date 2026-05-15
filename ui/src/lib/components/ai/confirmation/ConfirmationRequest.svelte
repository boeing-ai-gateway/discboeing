<script lang="ts">
	import type { Snippet } from "svelte";
	import { useConfirmationContext } from "./context";

	type Props = { children?: Snippet };
	let { children }: Props = $props();
	const confirmation = useConfirmationContext();
</script>

{#if confirmation.state === "approval-requested"}
	{@render children?.()}
{/if}
