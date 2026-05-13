<script lang="ts">
	import type { Snippet } from "svelte";
	import { InputGroupButton } from "$lib/components/ui/input-group";

	type Props = {
		variant?:
			| "default"
			| "destructive"
			| "outline"
			| "secondary"
			| "ghost"
			| "link";
		size?: "xs" | "sm" | "icon-xs" | "icon-sm";
		class?: string;
		children?: Snippet;
	};

	let {
		variant = "ghost",
		size = "icon-sm",
		class: className,
		children,
		...restProps
	}: Props = $props();
</script>

<InputGroupButton
	{variant}
	{size}
	class={className}
	type="button"
	{...restProps}
>
	{@render children?.()}
</InputGroupButton>
