<script lang="ts">
	import {
		CodeBlock,
		CodeBlockActions,
		CodeBlockCopyButton,
		CodeBlockHeader,
		CodeBlockTitle,
	} from "$lib/components/ai/code-block";
	import * as Dialog from "$lib/components/ui/dialog";

	type Props = {
		rawPatch: string;
	};

	let { rawPatch }: Props = $props();
</script>

<Dialog.Header>
	<Dialog.Title>Raw patch</Dialog.Title>
	<Dialog.Description>
		Review the raw replay bundle text that will be applied.
	</Dialog.Description>
</Dialog.Header>
<div class="max-h-[68vh] overflow-auto">
	{#if rawPatch.trim()}
		<CodeBlock
			code={rawPatch}
			language="diff"
			showLineNumbers={true}
			class="pr-1"
		>
			<CodeBlockHeader>
				<CodeBlockTitle>diff</CodeBlockTitle>
				<CodeBlockActions>
					<CodeBlockCopyButton
						aria-label="Copy raw patch"
						title="Copy raw patch"
					/>
				</CodeBlockActions>
			</CodeBlockHeader>
		</CodeBlock>
	{:else}
		<div class="text-muted-foreground text-sm">No raw patch is available.</div>
	{/if}
</div>
