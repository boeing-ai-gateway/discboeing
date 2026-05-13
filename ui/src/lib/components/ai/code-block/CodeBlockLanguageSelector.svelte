<script lang="ts">
	import type { Snippet } from "svelte";
	import type { ComponentProps } from "svelte";
	import { Select } from "$lib/components/ui/select";

	type Props = ComponentProps<typeof Select> & {
		children?: Snippet;
	};

	let { children, ...restProps }: Props = $props();
</script>

<Select {...restProps}>
	{@render children?.()}
</Select>
