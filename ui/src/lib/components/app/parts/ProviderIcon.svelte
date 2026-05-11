<script lang="ts">
	import * as simpleIcons from "simple-icons";
	import { cn } from "$lib/utils";

	type SimpleIconData = {
		title: string;
		slug: string;
		hex: string;
		source: string;
		svg: string;
		path: string;
	};

	type Props = {
		icon?: string | null;
		name?: string;
		class?: string;
	};

	let { icon, name = "Provider", class: className }: Props = $props();

	const simpleIconEntries = Object.values(simpleIcons).filter(
		(value): value is SimpleIconData =>
			typeof value === "object" &&
			value !== null &&
			"slug" in value &&
			"path" in value,
	);

	function normalizeSimpleIconName(value: string): string {
		return value
			.replace(/^(simple-icons?|simple):/i, "")
			.replace(/^si-/i, "")
			.replace(/^si/i, "")
			.replace(/[^a-z0-9]/gi, "")
			.toLowerCase();
	}

	function resolveSimpleIcon(value: string): SimpleIconData | null {
		const normalized = normalizeSimpleIconName(value);
		return (
			simpleIconEntries.find(
				(entry) => normalizeSimpleIconName(entry.slug) === normalized,
			) ??
			simpleIconEntries.find(
				(entry) => normalizeSimpleIconName(entry.title) === normalized,
			) ??
			null
		);
	}

	function isImageReference(value: string): boolean {
		return /^(https?:|data:image\/|\/|\.\/|\.\.\/)/i.test(value);
	}

	function isInlineSvg(value: string): boolean {
		return value.trimStart().toLowerCase().startsWith("<svg");
	}

	function sanitizeInlineSvg(value: string): string {
		return value
			.replace(/<script[\s\S]*?>[\s\S]*?<\/script>/gi, "")
			.replace(/<foreignObject[\s\S]*?>[\s\S]*?<\/foreignObject>/gi, "")
			.replace(/<(iframe|object|embed)[\s\S]*?>[\s\S]*?<\/\1>/gi, "")
			.replace(/\son[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi, "")
			.replace(/\s(href|xlink:href)\s*=\s*("|')\s*javascript:[\s\S]*?\2/gi, "");
	}

	const trimmedIcon = $derived(icon?.trim() ?? "");
	const simpleIcon = $derived(
		trimmedIcon && !isImageReference(trimmedIcon) && !isInlineSvg(trimmedIcon)
			? resolveSimpleIcon(trimmedIcon)
			: null,
	);
	const inlineSvg = $derived(
		trimmedIcon && isInlineSvg(trimmedIcon)
			? sanitizeInlineSvg(trimmedIcon)
			: "",
	);
	const initials = $derived(
		name
			.split(/\s+/)
			.filter(Boolean)
			.slice(0, 2)
			.map((part) => part[0]?.toUpperCase())
			.join("") || "P",
	);
</script>

<span
	class={cn(
		"inline-flex size-7 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-muted text-muted-foreground",
		className,
	)}
	aria-hidden="true"
>
	{#if simpleIcon}
		<svg viewBox="0 0 24 24" class="size-4" fill="currentColor">
			<path d={simpleIcon.path} />
		</svg>
	{:else if inlineSvg}
		<!-- eslint-disable svelte/no-at-html-tags -->
		<span class="provider-inline-svg size-4 text-current"
			>{@html inlineSvg}</span
		>
		<!-- eslint-enable svelte/no-at-html-tags -->
	{:else if trimmedIcon && isImageReference(trimmedIcon)}
		<img src={trimmedIcon} alt="" class="size-full object-contain" />
	{:else}
		<span class="text-[10px] font-medium leading-none">{initials}</span>
	{/if}
</span>

<style>
	.provider-inline-svg :global(svg) {
		display: block;
		height: 100%;
		width: 100%;
	}
</style>
