<script lang="ts">
	import type { Snippet } from "svelte";
	type Props = {
		children?: Snippet;
	};

	let { children, ...restProps }: Props = $props();
</script>

<span {...restProps}>
	{#if children}
		{@render children()}
	{:else}
		•
	{/if}
</span>
