<script lang="ts">
	import SettingsIcon from "@lucide/svelte/icons/settings";
	import type { SandboxProviderInstance } from "$lib/api-types";
	import ProviderIcon from "$lib/components/app/parts/ProviderIcon.svelte";
	import { Label } from "$lib/components/ui/label";
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectSeparator,
		SelectTrigger,
	} from "$lib/components/ui/select";

	type Props = {
		id: string;
		open?: boolean;
		value: string;
		providers: SandboxProviderInstance[];
		defaultProviderId: string;
		selectedProvider?: SandboxProviderInstance;
		selectedProviderTitle: string;
		labelClass?: string;
		triggerClass?: string;
		contentClass?: string;
		onSelect: (value: string) => void;
		onManageClick: () => void;
	};

	let {
		id,
		open = $bindable(false),
		value,
		providers,
		defaultProviderId,
		selectedProvider,
		selectedProviderTitle,
		labelClass = "sr-only",
		triggerClass = "h-8 px-2 text-xs",
		contentClass = "min-w-44",
		onSelect,
		onManageClick,
	}: Props = $props();
</script>

<Label for={id} class={labelClass}>Sandbox provider</Label>
<Select type="single" bind:open {value} onValueChange={onSelect}>
	<SelectTrigger
		{id}
		size="sm"
		class={triggerClass}
		title={selectedProviderTitle}
	>
		<ProviderIcon
			icon={selectedProvider?.icon}
			name={selectedProvider?.name ?? "Sandbox provider"}
			class="pointer-events-none size-4 border-0 bg-transparent"
		/>
	</SelectTrigger>
	<SelectContent class={contentClass}>
		{#each providers as provider (provider.id)}
			<SelectItem value={provider.id} label={provider.name}>
				<ProviderIcon
					icon={provider.icon}
					name={provider.name}
					class="size-4"
				/>
				<span>{provider.name}</span>
				{#if provider.id === defaultProviderId}
					<span
						class="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground"
					>
						default
					</span>
				{/if}
			</SelectItem>
		{/each}
		<SelectSeparator />
		<button
			type="button"
			class="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-hidden"
			onclick={onManageClick}
		>
			<SettingsIcon class="size-4" />
			<span>Manage</span>
		</button>
	</SelectContent>
</Select>
