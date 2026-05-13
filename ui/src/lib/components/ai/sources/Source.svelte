<script lang="ts">
	import type { Snippet } from "svelte";
	import { resolve } from "$app/paths";
	import BookIcon from "@lucide/svelte/icons/book";
	import { cn } from "$lib/utils";

	type Props = {
		href: string;
		title?: string;
		class?: string;
		children?: Snippet;
	};

	let {
		href,
		title,
		class: className,
		children,
		...restProps
	}: Props = $props();
</script>

<a
	class={cn("flex items-center gap-2", className)}
	href={resolve(href as Parameters<typeof resolve>[0])}
	rel="noreferrer"
	target="_blank"
	{...restProps}
>
	{#if children}
		{@render children()}
	{:else}
		<BookIcon class="h-4 w-4" />
		<span class="block font-medium">{title}</span>
	{/if}
</a>
