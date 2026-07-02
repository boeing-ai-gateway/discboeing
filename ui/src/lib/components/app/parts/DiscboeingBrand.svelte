<script lang="ts">
	import brandAssetUrl from "./branding-assets/discboeing-brand-gradient.svg";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		heightClass?: string;
		title?: string;
	};

	let {
		class: className,
		heightClass = "h-5",
		title = "Discboeing brand",
	}: Props = $props();
</script>

<img
	src={brandAssetUrl}
	alt={title}
	{title}
	class={cn("block w-auto shrink-0", heightClass, className)}
/>
