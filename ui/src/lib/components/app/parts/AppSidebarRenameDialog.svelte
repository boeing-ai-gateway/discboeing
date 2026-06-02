<script lang="ts">
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import { Input } from "$lib/components/ui/input";

	type Props = {
		open: boolean;
		title: string;
		description: string;
		label: string;
		value: string;
		placeholder: string;
		saving?: boolean;
		saveDisabled?: boolean;
		onOpenChange?: (open: boolean) => void;
		onValueChange: (value: string) => void;
		onCancel: () => void;
		onSave: () => void;
	};

	let {
		open = $bindable(false),
		title,
		description,
		label,
		value,
		placeholder,
		saving = false,
		saveDisabled = false,
		onOpenChange,
		onValueChange,
		onCancel,
		onSave,
	}: Props = $props();

	function handleInput(event: Event) {
		onValueChange((event.currentTarget as HTMLInputElement).value);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === "Enter") {
			event.preventDefault();
			if (saving || saveDisabled) {
				return;
			}
			onSave();
		}
	}
</script>

<Dialog.Root bind:open {onOpenChange}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>{description}</Dialog.Description>
		</Dialog.Header>
		<label class="space-y-1.5">
			<span class="text-sm font-medium text-foreground">{label}</span>
			<Input
				{value}
				oninput={handleInput}
				onkeydown={handleKeydown}
				maxlength={120}
				{placeholder}
			/>
		</label>
		<Dialog.Footer>
			<Button variant="ghost" size="sm" onclick={onCancel} disabled={saving}>
				Cancel
			</Button>
			<Button
				variant="default"
				size="sm"
				onclick={onSave}
				disabled={saving || saveDisabled}
			>
				Save
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
