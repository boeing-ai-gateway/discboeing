<script lang="ts">
	import PlusIcon from "@lucide/svelte/icons/plus";
	import XIcon from "@lucide/svelte/icons/x";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { Label } from "$lib/components/ui/label";

	type EnvVarRow = {
		id: string;
		key: string;
		value: string;
		hasStoredValue: boolean;
		replaceValue: boolean;
		valueFocused: boolean;
	};

	type Props = {
		rows: EnvVarRow[];
		onAddRow: () => void;
		onRemoveRow: (rowId: string) => void;
		onUpdateRow: (rowId: string, patch: Partial<Omit<EnvVarRow, "id">>) => void;
		onShowValueInput: (rowId: string) => void;
		onHideValueInput: (rowId: string) => void;
		onPaste: (
			rowId: string,
			field: "key" | "value",
			event: ClipboardEvent,
		) => void;
	};

	let {
		rows,
		onAddRow,
		onRemoveRow,
		onUpdateRow,
		onShowValueInput,
		onHideValueInput,
		onPaste,
	}: Props = $props();
</script>

<div class="space-y-2">
	<div class="flex items-center justify-between">
		<Label>Environment variables</Label>
		<Button variant="outline" size="xs" onclick={onAddRow}>
			<PlusIcon class="size-3" />
			Add row
		</Button>
	</div>
	{#each rows as row (row.id)}
		<div
			class="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-start"
		>
			<Input
				value={row.key}
				aria-label="Environment variable name"
				placeholder="KEY"
				class="min-w-0 font-mono"
				data-env-var-row-id={row.id}
				data-env-var-field="key"
				oninput={(event) =>
					onUpdateRow(row.id, {
						key: (event.currentTarget as HTMLInputElement).value,
					})}
				onpaste={(event) => onPaste(row.id, "key", event)}
			/>
			<div class="min-w-0 space-y-1">
				{#if row.hasStoredValue && !row.replaceValue}
					<div class="text-sm text-muted-foreground">
						A value is already stored.
					</div>
					<Button
						variant="ghost"
						size="xs"
						class="h-auto px-0"
						onclick={() => onShowValueInput(row.id)}
					>
						Update value
					</Button>
				{:else}
					<Input
						type={row.valueFocused ? "text" : "password"}
						value={row.value}
						aria-label="Environment variable value"
						placeholder={row.hasStoredValue ? "Enter a new value" : "value"}
						class="font-mono"
						data-env-var-row-id={row.id}
						data-env-var-field="value"
						onfocus={() => onUpdateRow(row.id, { valueFocused: true })}
						onblur={() => onUpdateRow(row.id, { valueFocused: false })}
						oninput={(event) =>
							onUpdateRow(row.id, {
								value: (event.currentTarget as HTMLInputElement).value,
							})}
						onpaste={(event) => onPaste(row.id, "value", event)}
					/>
					<p class="text-sm text-muted-foreground">
						{row.hasStoredValue
							? "Saving will replace the stored value."
							: "This value will be stored securely."}
					</p>
					{#if row.hasStoredValue}
						<Button
							variant="ghost"
							size="xs"
							class="h-auto px-0"
							onclick={() => onHideValueInput(row.id)}
						>
							Keep existing value
						</Button>
					{/if}
				{/if}
			</div>
			{#if rows.length > 1}
				<Button
					variant="ghost"
					size="icon-xs"
					class="md:self-start"
					aria-label="Remove environment variable"
					onclick={() => onRemoveRow(row.id)}
				>
					<XIcon class="size-3" aria-hidden="true" />
				</Button>
			{/if}
		</div>
	{/each}
</div>
