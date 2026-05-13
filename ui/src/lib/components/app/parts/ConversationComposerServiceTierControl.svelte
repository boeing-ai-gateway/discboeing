<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import ZapIcon from "@lucide/svelte/icons/zap";
	import {
		DropdownMenu,
		DropdownMenuContent,
		DropdownMenuItem,
		DropdownMenuTrigger,
	} from "$lib/components/ui/dropdown-menu";
	import { InputGroupButton } from "$lib/components/ui/input-group";

	type Props = {
		value?: string | undefined;
		tiers: string[];
		onSelect?: (value: string | undefined) => void;
	};

	let { value = undefined, tiers, onSelect = () => {} }: Props = $props();

	function formatServiceTierLabel(tier: string | undefined) {
		switch (tier?.toLowerCase()) {
			case "priority":
			case "fast":
				return "Fast";
			default:
				return "Standard";
		}
	}

	function formatServiceTierDescription(tier: string) {
		switch (tier.toLowerCase()) {
			case "priority":
			case "fast":
				return "Use the provider priority service tier";
			default:
				return `Use the ${tier} service tier`;
		}
	}

	const normalizedValue = $derived.by(() => value?.toLowerCase());
	const buttonLabel = $derived.by(() => formatServiceTierLabel(value));
</script>

<DropdownMenu>
	<DropdownMenuTrigger class="desktop-no-drag">
		<InputGroupButton
			size="xs"
			variant={value ? "secondary" : "ghost"}
			class="h-6 gap-1.5 px-2 text-xs"
			title={`Service tier: ${buttonLabel}`}
		>
			<ZapIcon class="size-3.5 shrink-0" />
			<span>{buttonLabel}</span>
		</InputGroupButton>
	</DropdownMenuTrigger>
	<DropdownMenuContent align="start" class="w-64">
		<DropdownMenuItem
			onclick={() => {
				onSelect(undefined);
			}}
			class="justify-between gap-3"
		>
			<div class="min-w-0 flex-1">
				<div class="font-medium">Standard</div>
				<div class="text-xs text-muted-foreground">
					Use the provider default service tier
				</div>
			</div>
			{#if !value}
				<CheckIcon class="size-3.5 text-primary" />
			{/if}
		</DropdownMenuItem>

		{#each tiers as tier (tier)}
			<DropdownMenuItem
				onclick={() => {
					onSelect(tier);
				}}
				class="justify-between gap-3"
			>
				<div class="min-w-0 flex-1">
					<div class="font-medium">{formatServiceTierLabel(tier)}</div>
					<div class="text-xs text-muted-foreground">
						{formatServiceTierDescription(tier)}
					</div>
				</div>
				{#if normalizedValue === tier.toLowerCase()}
					<CheckIcon class="size-3.5 text-primary" />
				{/if}
			</DropdownMenuItem>
		{/each}
	</DropdownMenuContent>
</DropdownMenu>
