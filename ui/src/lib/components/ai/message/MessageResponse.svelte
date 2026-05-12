<script lang="ts">
	import { SvelteStreamdown } from "$lib/components/ai/streamdown";
	import type { MarkdownMode } from "$lib/markdown/types";
	import { cn } from "$lib/utils";

	type Props = {
		text?: string;
		class?: string;
		mode?: MarkdownMode;
		isAnimating?: boolean;
		children?: () => any;
	};

	let {
		text,
		class: className,
		mode = "streaming",
		isAnimating = false,
		children,
		...restProps
	}: Props = $props();
</script>

<div class={cn("size-full break-words", className)} {...restProps}>
	{#if text !== undefined}
		<SvelteStreamdown
			{text}
			{mode}
			{isAnimating}
			class="size-full [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
		/>
	{:else}
		{@render children?.()}
	{/if}
</div>
