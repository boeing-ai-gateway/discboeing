<script module lang="ts">
	import type { ModelInfo } from "$lib/api-types";
	import type {
		DisplayModel,
		ModelProviderEntry,
	} from "./conversation-composer-model-control-search";
	import { filterModelProviderEntries } from "./conversation-composer-model-control-search";

	function cleanModelName(name: string) {
		return name.replace(/\s*\(latest\)\s*/gi, "").trim();
	}

	function getBaseName(name: string) {
		return cleanModelName(name)
			.replace(/\s+v\d+\s*/gi, "")
			.replace(/\s+[\d.]+\s*$/, "")
			.trim();
	}

	function extractVersion(name: string) {
		const matches = name.match(/(\d+(?:\.\d+)?)/g);
		if (!matches || matches.length === 0) {
			return 0;
		}
		return Number.parseFloat(matches[matches.length - 1]);
	}

	function sortModels(left: DisplayModel, right: DisplayModel) {
		const baseCompare = getBaseName(left.name).localeCompare(
			getBaseName(right.name),
		);
		if (baseCompare !== 0) {
			return baseCompare;
		}

		const versionLeft = extractVersion(left.name);
		const versionRight = extractVersion(right.name);
		if (versionLeft !== versionRight) {
			return versionRight - versionLeft;
		}

		return left.name.localeCompare(right.name);
	}

	function toDisplayModel(model: ModelInfo): DisplayModel {
		return {
			...model,
			name: cleanModelName(model.name),
			selectedIds: [model.id],
		};
	}

	function mergeSelectedIds(target: DisplayModel, source: ModelInfo) {
		if (!target.selectedIds.includes(source.id)) {
			target.selectedIds.push(source.id);
		}
	}

	function buildModelProviderEntries(models: ModelInfo[]) {
		const modelByProviderAndName: Record<string, DisplayModel> = {};

		for (const model of models) {
			const cleanName = cleanModelName(model.name);
			const isLatest = /\(latest\)/i.test(model.name);
			const dedupeKey = `${model.provider || "Other"}::${cleanName}`;
			const existing = modelByProviderAndName[dedupeKey];

			if (!existing) {
				modelByProviderAndName[dedupeKey] = toDisplayModel(model);
				continue;
			}

			mergeSelectedIds(existing, model);
			if (isLatest) {
				modelByProviderAndName[dedupeKey] = {
					...toDisplayModel(model),
					selectedIds: existing.selectedIds,
				};
			}
		}

		const grouped: Record<string, DisplayModel[]> = {};
		for (const model of Object.values(modelByProviderAndName).sort(
			sortModels,
		)) {
			const provider = model.provider || "Other";
			if (!grouped[provider]) {
				grouped[provider] = [];
			}
			grouped[provider].push(model);
		}

		return Object.entries(grouped).sort(([left], [right]) =>
			left.localeCompare(right),
		) as ModelProviderEntry[];
	}

	function findSelectedModel(
		entries: ModelProviderEntry[],
		value: string | null,
	) {
		if (value === null) {
			return null;
		}

		return (
			entries
				.flatMap(([, providerModels]) => providerModels)
				.find((model) => model.selectedIds.includes(value)) ?? null
		);
	}
</script>

<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import {
		DropdownMenu,
		DropdownMenuContent,
		DropdownMenuItem,
		DropdownMenuLabel,
		DropdownMenuSeparator,
		DropdownMenuTrigger,
	} from "$lib/components/ui/dropdown-menu";
	import { Input } from "$lib/components/ui/input";
	import { InputGroupButton } from "$lib/components/ui/input-group";

	type Props = {
		value?: string | null;
		onSelect?: (value: string | null) => void;
		models: ModelInfo[];
	};

	let { value = null, onSelect = () => {}, models }: Props = $props();

	let modelSearchQuery = $state("");
	const modelProviderEntries = $derived(buildModelProviderEntries(models));
	const filteredModelProviderEntries = $derived.by(() =>
		filterModelProviderEntries(modelProviderEntries, modelSearchQuery),
	);
	const selectedModel = $derived(
		findSelectedModel(modelProviderEntries, value),
	);
</script>

<DropdownMenu>
	<DropdownMenuTrigger class="desktop-no-drag">
		<InputGroupButton
			size="xs"
			variant="ghost"
			class="h-6 max-w-[160px] px-0.5 text-xs"
			title={selectedModel ? `Model: ${selectedModel.name}` : "Model"}
		>
			{#if selectedModel}
				<span class="truncate">{selectedModel.name}</span>
			{:else}
				<span>Model</span>
			{/if}
		</InputGroupButton>
	</DropdownMenuTrigger>
	<DropdownMenuContent align="start" class="max-h-[24rem] w-80 overflow-y-auto">
		<div
			onmousedown={(event) => event.stopPropagation()}
			class="p-2"
			role="presentation"
		>
			<Input
				bind:value={modelSearchQuery}
				type="search"
				placeholder="Search models"
				aria-label="Search models"
				class="h-8 text-xs"
			/>
		</div>
		<DropdownMenuSeparator />
		<DropdownMenuItem
			onclick={() => {
				onSelect(null);
			}}
			class="justify-between"
		>
			<span>Default model</span>
			{#if value === null}
				<CheckIcon class="size-3.5 text-primary" />
			{/if}
		</DropdownMenuItem>

		{#if filteredModelProviderEntries.length > 0}
			{#each filteredModelProviderEntries as [provider, providerModels], providerIndex (provider)}
				{#if providerIndex > 0}
					<DropdownMenuSeparator />
				{/if}
				<DropdownMenuLabel
					class="text-xs uppercase tracking-[0.16em] text-muted-foreground"
				>
					{provider}
				</DropdownMenuLabel>
				{#each providerModels as model (model.id)}
					<DropdownMenuItem
						onclick={() => {
							onSelect(model.id);
						}}
						class="justify-between gap-3"
					>
						<div class="min-w-0 flex-1 pl-3">
							<div class="truncate font-medium">{model.name}</div>
							{#if model.description}
								<div class="truncate text-xs text-muted-foreground">
									{model.description}
								</div>
							{/if}
						</div>
						{#if value !== null && model.selectedIds.includes(value)}
							<CheckIcon class="size-3.5 text-primary" />
						{/if}
					</DropdownMenuItem>
				{/each}
			{/each}
		{:else}
			<div class="px-3 py-2 text-xs text-muted-foreground">
				{#if modelSearchQuery.trim().length > 0}
					No models match “{modelSearchQuery}”
				{:else}
					No models available
				{/if}
			</div>
		{/if}
	</DropdownMenuContent>
</DropdownMenu>
