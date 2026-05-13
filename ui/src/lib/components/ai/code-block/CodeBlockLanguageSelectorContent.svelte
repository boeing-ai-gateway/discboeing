<script lang="ts">
	import type { Snippet } from "svelte";
	import type { ComponentProps } from "svelte";
	import { SelectContent } from "$lib/components/ui/select";

	type Props = ComponentProps<typeof SelectContent> & {
		children?: Snippet;
	};

	let { children, ...restProps }: Props = $props();
</script>

<SelectContent {...restProps}>
	{@render children?.()}
</SelectContent>
