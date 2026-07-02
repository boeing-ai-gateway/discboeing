<script lang="ts">
	import logoAssetUrl from "./branding-assets/discboeing-logo-purple.svg";
	import { cn } from "$lib/utils";

	type Props = {
		class?: string;
		title?: string;
		size?: number;
	};

	let {
		class: className,
		title = "Discboeing logo",
		size = 24,
	}: Props = $props();
</script>

<img
	src={logoAssetUrl}
	alt={title}
	{title}
	width={size}
	height={size}
	class={cn("block shrink-0", className)}
/>
