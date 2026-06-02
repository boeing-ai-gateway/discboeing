<script lang="ts">
	import XIcon from "@lucide/svelte/icons/x";

	import { Badge } from "$lib/components/ui/badge";
	import type { FileStatus } from "$lib/api-types";
	import { cn } from "$lib/utils";

	type Props = {
		activePath: string | null;
		fileLabel: (path: string) => string;
		getStatus: (path: string) => FileStatus | undefined;
		isDirty: (path: string) => boolean;
		onClose: (path: string) => void;
		onOpen: (path: string) => void;
		openPaths: string[];
		statusBadgeClass: (status?: FileStatus) => string;
		statusLetter: (status?: FileStatus) => string;
	};

	let {
		activePath,
		fileLabel,
		getStatus,
		isDirty,
		onClose,
		onOpen,
		openPaths,
		statusBadgeClass,
		statusLetter,
	}: Props = $props();
</script>

<div
	class="flex min-h-10 items-end gap-1 overflow-x-auto border-b border-sidebar-border bg-sidebar px-2 py-2"
>
	{#if openPaths.length === 0}
		<p class="px-2 text-sm text-sidebar-foreground/50">
			Open a file from the explorer.
		</p>
	{:else}
		{#each openPaths as path (path)}
			{@const status = getStatus(path)}
			<div
				class={cn(
					"flex shrink-0 items-center rounded-md border text-sm transition",
					activePath === path
						? "border-sidebar-border bg-background text-foreground shadow-sm"
						: "border-transparent bg-sidebar-accent/60 text-sidebar-foreground/75 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
				)}
			>
				<button
					type="button"
					onclick={() => onOpen(path)}
					class="flex min-w-0 items-center gap-2 px-3 py-1.5 text-left"
				>
					<span class="truncate max-w-36">{fileLabel(path)}</span>
					{#if isDirty(path)}
						<span class="size-2 rounded-full bg-sidebar-primary"></span>
					{/if}
					{#if status}
						<Badge
							variant="outline"
							class={cn("px-1 py-0 text-[10px]", statusBadgeClass(status))}
						>
							{statusLetter(status)}
						</Badge>
					{/if}
				</button>
				<button
					type="button"
					onclick={() => onClose(path)}
					class="mr-1 rounded p-0.5 text-sidebar-foreground/45 transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
					aria-label={`Close ${fileLabel(path)} tab`}
				>
					<XIcon class="size-3.5" />
				</button>
			</div>
		{/each}
	{/if}
</div>
