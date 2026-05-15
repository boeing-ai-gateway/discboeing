<script lang="ts">
	import type { Snippet } from "svelte";
	import type { ComponentProps } from "svelte";
	import { Carousel } from "$lib/components/ui/carousel";
	import { cn } from "$lib/utils";

	type Props = ComponentProps<typeof Carousel> & {
		class?: string;
		children?: Snippet;
	};

	let { class: className, children, ...restProps }: Props = $props();
</script>

<Carousel class={cn("w-full", className)} {...restProps}>
	{@render children?.()}
</Carousel>
