<script lang="ts">
	import type { BrowserEventFile } from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";

	type Props = {
		open: boolean;
		file: BrowserEventFile | null;
		url: string | null;
		loading: boolean;
		error: string | null;
	};

	let { open = $bindable(false), file, url, loading, error }: Props = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-5xl">
		<Dialog.Header>
			<Dialog.Title>{file?.filename ?? "Browser screenshot"}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-3">
			{#if file?.path}
				<div class="font-mono text-muted-foreground text-xs break-all">
					{file.path}
				</div>
			{/if}
			{#if loading}
				<div
					class="rounded-md border border-border bg-background px-3 py-4 text-muted-foreground text-sm"
				>
					Loading screenshot...
				</div>
			{:else if error}
				<div
					class="rounded-md border border-border bg-background px-3 py-4 text-destructive text-sm"
				>
					{error}
				</div>
			{:else if url}
				<div
					class="overflow-auto rounded-md border border-border bg-background p-2"
				>
					<img
						alt={file?.filename ?? "Browser screenshot"}
						class="mx-auto h-auto max-w-full rounded"
						src={url}
					/>
				</div>
			{:else}
				<div
					class="rounded-md border border-border bg-background px-3 py-4 text-muted-foreground text-sm"
				>
					Screenshot unavailable.
				</div>
			{/if}
		</div>
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
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
