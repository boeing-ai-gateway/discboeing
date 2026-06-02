<script module lang="ts">
	import * as simpleIcons from "simple-icons";
	import { SvelteMap } from "svelte/reactivity";

	type SimpleIconData = {
		title: string;
		slug: string;
		hex: string;
		source: string;
		svg: string;
		path: string;
	};

	function normalizeSimpleIconName(value: string): string {
		return value
			.replace(/^(simple-icons?|simple):/i, "")
			.replace(/^si-/i, "")
			.replace(/^si/i, "")
			.replace(/[^a-z0-9]/gi, "")
			.toLowerCase();
	}

	const simpleIconIndex = new SvelteMap<string, SimpleIconData>();

	for (const value of Object.values(simpleIcons)) {
		if (
			typeof value !== "object" ||
			value === null ||
			!("slug" in value) ||
			!("title" in value) ||
			!("path" in value)
		) {
			continue;
		}

		const icon = value as SimpleIconData;
		simpleIconIndex.set(normalizeSimpleIconName(icon.slug), icon);
		simpleIconIndex.set(normalizeSimpleIconName(icon.title), icon);
	}

	function resolveSimpleIcon(value: string): SimpleIconData | null {
		return simpleIconIndex.get(normalizeSimpleIconName(value)) ?? null;
	}
</script>

<script lang="ts">
	import { cn } from "$lib/utils";

	type Props = {
		icon?: string | null;
		name?: string;
		class?: string;
	};

	let { icon, name = "Provider", class: className }: Props = $props();

	function isImageReference(value: string): boolean {
		return /^(https?:|data:image\/|\/|\.\/|\.\.\/)/i.test(value);
	}

	const trimmedIcon = $derived(icon?.trim() ?? "");
	const simpleIcon = $derived(
		trimmedIcon && !isImageReference(trimmedIcon)
			? resolveSimpleIcon(trimmedIcon)
			: null,
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
	{:else if trimmedIcon && isImageReference(trimmedIcon)}
		<img src={trimmedIcon} alt="" class="size-full object-contain" />
	{:else}
		<span class="text-[10px] font-medium leading-none">{initials}</span>
	{/if}
</span>
