<script lang="ts">
	import type { Snippet } from "svelte";
	import Markdown from "$lib/components/ai/streamdown/Markdown.svelte";
	import type { MarkdownMode } from "$lib/web-components/markdown";
	import { cn } from "$lib/utils";

	type Props = {
		text?: string;
		class?: string;
		mode?: MarkdownMode;
		isAnimating?: boolean;
		children?: Snippet;
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
		<Markdown
			{text}
			{mode}
			{isAnimating}
			class="size-full [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
		/>
	{:else}
		{@render children?.()}
	{/if}
</div>
