<script lang="ts">
	import type { Icon } from "$lib/api-types";

	type CredentialTypePickerOption = {
		value: string;
		label: string;
		description: string;
		image: Icon | null;
		imageClass?: string;
		monogram: string;
	};

	type CredentialTypePickerGroup = {
		group: string;
		name: string;
		options: CredentialTypePickerOption[];
	};

	type Props = {
		groups: CredentialTypePickerGroup[];
		onChoose: (optionValue: string) => void;
	};

	let { groups, onChoose }: Props = $props();
</script>

<div class="space-y-4">
	{#each groups as group}
		<div class="space-y-2">
			<div
				class="text-xs font-medium uppercase tracking-wide text-muted-foreground"
			>
				{group.name}
			</div>
			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each group.options as option}
					<button
						type="button"
						class="flex min-h-28 flex-col items-start gap-3 rounded-lg border border-border bg-background p-4 text-left transition-colors hover:bg-muted/40"
						onclick={() => onChoose(option.value)}
					>
						<div class="flex items-center gap-3">
							{#if option.image}
								<div
									class="flex size-10 items-center justify-center rounded-md border border-border/70 bg-muted/50 p-1.5"
								>
									<img
										src={option.image.src}
										alt=""
										class={option.imageClass ?? ""}
									/>
								</div>
							{:else}
								<div
									class="flex size-10 items-center justify-center rounded-md border border-border bg-muted text-sm font-semibold"
								>
									{option.monogram}
								</div>
							{/if}
							<div class="font-medium leading-none">{option.label}</div>
						</div>
						<div class="text-sm text-muted-foreground">
							{option.description}
						</div>
					</button>
				{/each}
			</div>
		</div>
	{/each}
</div>
