<script lang="ts">
	import type { Snippet } from "svelte";
	import { AlertDescription } from "$lib/components/ui/alert";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
</script>

<AlertDescription class={cn("inline", className)} {...restProps}>
	{@render children?.()}
</AlertDescription>
