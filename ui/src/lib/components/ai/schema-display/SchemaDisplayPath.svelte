<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";
	import { useSchemaDisplayContext } from "./context";

	type PathPart = { value: string; isParam: boolean };
	type Props = { class?: string; children?: Snippet };
	let { class: className, children, ...restProps }: Props = $props();
	const schemaDisplay = useSchemaDisplayContext();

	const pathParts = $derived.by(() => {
		const source = schemaDisplay.path;
		return source
			.split(/(\{[^}]+\})/g)
			.filter(Boolean)
			.map(
				(part): PathPart => ({
					value: part,
					isParam: /^\{[^}]+\}$/.test(part),
				}),
			);
	});
</script>

<span class={cn("font-mono text-sm", className)} {...restProps}>
	{#if children}
		{@render children()}
	{:else}
		{#each pathParts as part, __key0 (__key0)}
			{#if part.isParam}
				<span class="text-blue-600 dark:text-blue-400"
					>{part.value.slice(1, -1)}</span
				>
			{:else}
				{part.value}
			{/if}
		{/each}
	{/if}
</span>
