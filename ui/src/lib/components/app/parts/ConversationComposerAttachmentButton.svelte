<script lang="ts">
	import PaperclipIcon from "@lucide/svelte/icons/paperclip";
	import { InputGroupButton } from "$lib/components/ui/input-group";

	type Props = {
		onFilesAdd: (files: File[] | FileList) => void;
		disabled?: boolean;
	};

	let { onFilesAdd, disabled = false }: Props = $props();

	let fileInputRef = $state<HTMLInputElement | null>(null);

	function openFileDialog() {
		if (disabled) {
			return;
		}
		fileInputRef?.click();
	}

	function handleFileInputChange(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		if (input.files) {
			onFilesAdd(input.files);
		}
		input.value = "";
	}
</script>

<input
	bind:this={fileInputRef}
	type="file"
	class="hidden"
	multiple
	{disabled}
	onchange={handleFileInputChange}
/>

<InputGroupButton
	size="icon-sm"
	variant="ghost"
	class="desktop-no-drag"
	aria-label="Add photos or files"
	{disabled}
	onclick={openFileDialog}
>
	<PaperclipIcon class="size-4" />
</InputGroupButton>
