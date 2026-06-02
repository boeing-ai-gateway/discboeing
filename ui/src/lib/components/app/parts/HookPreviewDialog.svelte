<script lang="ts">
	import CodeBlock from "$lib/components/ai/code-block/CodeBlock.svelte";
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import type { HookFailureMessageMetadata } from "$lib/components/app/conversation-pane-message-parts";
	import { getHookPathDisplayLabel } from "$lib/components/app/conversation-pane-message-parts";

	type Props = {
		open: boolean;
		metadata: HookFailureMessageMetadata | null;
		content: string;
		loading: boolean;
		error: string | null;
		onEdit: () => void;
	};

	let {
		open = $bindable(false),
		metadata,
		content,
		loading,
		error,
		onEdit,
	}: Props = $props();

	function getHookFileLanguage(path: string | undefined): string {
		const extension = path?.split(".").at(-1)?.toLowerCase() ?? "";
		switch (extension) {
			case "sh":
			case "bash":
			case "zsh":
				return "shell";
			case "py":
				return "python";
			case "js":
				return "javascript";
			case "ts":
				return "typescript";
			case "rb":
				return "ruby";
			case "go":
				return "go";
			case "yaml":
			case "yml":
				return "yaml";
			default:
				return "plaintext";
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-4xl">
		<Dialog.Header>
			<Dialog.Title>{metadata?.hookName ?? "Hook file"}</Dialog.Title>
		</Dialog.Header>
		{#if metadata?.hookPath}
			<div class="space-y-3">
				<div class="font-mono text-muted-foreground text-xs break-all">
					{getHookPathDisplayLabel(metadata.hookPath)}
				</div>
				{#if loading}
					<div
						class="rounded-md border border-border bg-background px-3 py-4 text-muted-foreground text-sm"
					>
						Loading hook file...
					</div>
				{:else if error}
					<div
						class="rounded-md border border-border bg-background px-3 py-4 text-destructive text-sm"
					>
						{error}
					</div>
				{:else if content}
					<CodeBlock
						class="max-h-[60vh] overflow-auto"
						code={content}
						language={getHookFileLanguage(metadata.hookPath)}
						showLineNumbers={true}
					/>
				{:else}
					<div
						class="rounded-md border border-border bg-background px-3 py-4 text-muted-foreground text-sm"
					>
						Hook file is empty.
					</div>
				{/if}
			</div>
		{/if}
		<Dialog.Footer>
			<Button
				variant="ghost"
				size="sm"
				onclick={() => {
					open = false;
				}}
			>
				Close
			</Button>
			<Button
				disabled={!metadata?.hookPath}
				size="sm"
				onclick={() => {
					onEdit();
				}}
			>
				Edit
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
