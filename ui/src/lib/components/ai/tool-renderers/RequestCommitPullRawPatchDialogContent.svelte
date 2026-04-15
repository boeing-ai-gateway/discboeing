<script lang="ts">
	import SvelteStreamdown from "$lib/components/ai/streamdown/SvelteStreamdown.svelte";
	import * as Dialog from "$lib/components/ui/dialog";

	type Props = {
		rawPatchMarkdown: string;
	};

	let { rawPatchMarkdown }: Props = $props();
</script>

<Dialog.Header>
	<Dialog.Title>Raw patch</Dialog.Title>
	<Dialog.Description>
		Review the raw replay bundle text that will be applied.
	</Dialog.Description>
</Dialog.Header>
<div class="max-h-[68vh] overflow-auto">
	{#if rawPatchMarkdown}
		<SvelteStreamdown text={rawPatchMarkdown} mode="static" class="pr-1" />
	{:else}
		<div class="text-muted-foreground text-sm">No raw patch is available.</div>
	{/if}
</div>
