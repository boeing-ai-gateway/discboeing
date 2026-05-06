<script lang="ts">
	import { siCursor } from "simple-icons";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		title?: string;
	};

	let {
		class: className,
		title = siCursor.title,
		...restProps
	}: Props = $props();
</script>

<svg
	aria-label={title}
	class={cn("size-4", className)}
	role="img"
	viewBox="0 0 24 24"
	{...restProps}
>
	<title>{title}</title>
	<path fill="currentColor" d={siCursor.path} />
</svg>
