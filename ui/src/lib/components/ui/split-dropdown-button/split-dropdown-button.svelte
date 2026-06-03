<script lang="ts">
	import type { Snippet } from "svelte";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import type { DropdownMenu as DropdownMenuPrimitive } from "bits-ui";
	import type {
		ButtonProps,
		ButtonSize,
		ButtonVariant,
	} from "$lib/components/ui/button";
	import { Button } from "$lib/components/ui/button";
	import {
		DropdownMenu,
		DropdownMenuContent,
		DropdownMenuTrigger,
	} from "$lib/components/ui/dropdown-menu";
	import { cn } from "$lib/utils";

	type Props = {
		label: string;
		menuAriaLabel: string;
		icon?: Snippet;
		iconOnly?: boolean;
		class?: string;
		contentClass?: string;
		children?: Snippet;
		onclick?: ButtonProps["onclick"];
		variant?: ButtonVariant;
		size?: ButtonSize;
		type?: ButtonProps["type"];
		disabled?: boolean;
		primaryDisabled?: boolean;
		align?: DropdownMenuPrimitive.ContentProps["align"];
		sideOffset?: DropdownMenuPrimitive.ContentProps["sideOffset"];
	};

	let {
		label,
		menuAriaLabel,
		icon,
		iconOnly = false,
		class: className,
		contentClass,
		children,
		onclick,
		variant = "outline",
		size = "xs",
		type = "button",
		disabled = false,
		primaryDisabled = false,
		align = "end",
		sideOffset = 8,
	}: Props = $props();

	const iconClass = $derived(
		size === "xs" || size === "icon-xs" ? "size-3.5" : "size-4",
	);
	const useSharedOutlineBorder = $derived(variant === "outline");
	const groupClass = $derived(
		useSharedOutlineBorder
			? "group inline-flex items-center overflow-hidden rounded-md border border-border bg-background p-0.5 shadow-xs"
			: "group inline-flex items-center",
	);
	const primaryButtonClass = $derived(
		useSharedOutlineBorder
			? "rounded-l-[calc(var(--radius)-1px)] rounded-r-none border-0 bg-transparent shadow-none group-hover:bg-accent group-hover:text-accent-foreground dark:bg-transparent dark:group-hover:bg-accent/50"
			: "rounded-r-none group-hover:bg-accent group-hover:text-accent-foreground dark:group-hover:bg-accent/50",
	);
	const triggerButtonClass = $derived(
		useSharedOutlineBorder
			? "rounded-r-[calc(var(--radius)-1px)] rounded-l-none border-0 border-l border-border bg-transparent px-2 shadow-none group-hover:bg-accent group-hover:text-accent-foreground dark:bg-transparent dark:group-hover:bg-accent/50"
			: "rounded-l-none border-l-0 px-2 group-hover:bg-accent group-hover:text-accent-foreground dark:group-hover:bg-accent/50",
	);
</script>

<DropdownMenu>
	<div class={cn(groupClass, className)}>
		<Button
			{variant}
			{size}
			{type}
			disabled={disabled || primaryDisabled}
			{onclick}
			class={primaryButtonClass}
		>
			{@render icon?.()}
			<span class={iconOnly ? "sr-only" : undefined}>{label}</span>
		</Button>
		<DropdownMenuTrigger>
			{#snippet child({ props })}
				<Button
					{...props}
					{variant}
					{size}
					{type}
					{disabled}
					class={triggerButtonClass}
					aria-label={menuAriaLabel}
				>
					<ChevronDownIcon class={iconClass} />
				</Button>
			{/snippet}
		</DropdownMenuTrigger>
	</div>
	<DropdownMenuContent {align} {sideOffset} class={contentClass}>
		{@render children?.()}
	</DropdownMenuContent>
</DropdownMenu>
