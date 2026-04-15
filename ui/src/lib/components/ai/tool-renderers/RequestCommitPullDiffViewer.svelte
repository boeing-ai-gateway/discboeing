<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";

	import DiffReviewFileRenderer from "$lib/components/app/parts/DiffReviewFileRenderer.svelte";
	import { Badge } from "$lib/components/ui/badge";
	import { Button } from "$lib/components/ui/button";
	import type { DiffStyle } from "$lib/pierre-diff";
	import { cn } from "$lib/utils";
	import type { RequestCommitPullDiffEntry } from "./request-commit-pull-diff";

	type Props = {
		files: RequestCommitPullDiffEntry[];
		diffStyle: DiffStyle;
		onDiffStyleChange?: (style: DiffStyle) => void;
	};

	let { files, diffStyle, onDiffStyleChange }: Props = $props();

	let expandedKey = $state<string | null>(null);
	let approvedPaths = $state<Record<string, true>>({});

	const approvedCount = $derived.by(
		() => files.filter((file) => approvedPaths[getEntryKey(file)]).length,
	);
	const allApproved = $derived.by(
		() => files.length > 0 && approvedCount === files.length,
	);

	$effect(() => {
		const availableKeys = new Set(files.map((file) => getEntryKey(file)));
		if (expandedKey && !availableKeys.has(expandedKey)) {
			expandedKey = null;
		}

		const nextApprovedPaths: Record<string, true> = {};
		for (const file of files) {
			const key = getEntryKey(file);
			if (approvedPaths[key]) {
				nextApprovedPaths[key] = true;
			}
		}
		const currentKeys = Object.keys(approvedPaths);
		const nextKeys = Object.keys(nextApprovedPaths);
		if (
			currentKeys.length !== nextKeys.length ||
			nextKeys.some((key) => !approvedPaths[key])
		) {
			approvedPaths = nextApprovedPaths;
		}
	});

	function getEntryKey(file: RequestCommitPullDiffEntry): string {
		return `${file.commitHash}:${file.path}`;
	}

	function toggleExpanded(file: RequestCommitPullDiffEntry) {
		const key = getEntryKey(file);
		expandedKey = expandedKey === key ? null : key;
	}

	function isApproved(file: RequestCommitPullDiffEntry): boolean {
		return Boolean(approvedPaths[getEntryKey(file)]);
	}

	function toggleApproved(file: RequestCommitPullDiffEntry) {
		const key = getEntryKey(file);
		if (approvedPaths[key]) {
			const nextApprovedPaths = { ...approvedPaths };
			delete nextApprovedPaths[key];
			approvedPaths = nextApprovedPaths;
			return;
		}

		approvedPaths = {
			...approvedPaths,
			[key]: true,
		};
	}

	function markAllApproved() {
		approvedPaths = Object.fromEntries(
			files.map((file) => [getEntryKey(file), true] as const),
		);
	}

	function statusBadgeClass(status: RequestCommitPullDiffEntry["status"]) {
		switch (status) {
			case "added":
				return "text-green-500 border-green-500/40";
			case "modified":
				return "text-yellow-500 border-yellow-500/40";
			case "deleted":
				return "text-red-500 border-red-500/40";
			case "renamed":
				return "text-purple-500 border-purple-500/40";
			default:
				return "text-muted-foreground border-border";
		}
	}

	function statusLabel(status: RequestCommitPullDiffEntry["status"]) {
		switch (status) {
			case "added":
				return "Added";
			case "modified":
				return "Modified";
			case "deleted":
				return "Deleted";
			case "renamed":
				return "Renamed";
			default:
				return "Changed";
		}
	}
</script>

<div class="space-y-4">
	<div class="flex items-center justify-end gap-2">
		<div
			class="inline-flex rounded-md border border-border bg-background p-0.5"
		>
			<Button
				variant={diffStyle === "unified" ? "secondary" : "ghost"}
				size="sm"
				class="h-8 rounded-r-none px-3"
				onclick={() => onDiffStyleChange?.("unified")}
			>
				Unified
			</Button>
			<Button
				variant={diffStyle === "split" ? "secondary" : "ghost"}
				size="sm"
				class="h-8 rounded-l-none border-l border-border px-3"
				onclick={() => onDiffStyleChange?.("split")}
			>
				Split
			</Button>
		</div>
	</div>

	{#if files.length === 0}
		<div
			class="rounded-md border bg-muted/20 p-3 text-muted-foreground text-sm"
		>
			No file diff preview is available.
		</div>
	{:else}
		<div
			class="flex max-h-[68vh] min-h-0 flex-1 flex-col overflow-hidden rounded-md border border-sidebar-border bg-sidebar"
		>
			<div
				class="flex items-center justify-between gap-3 border-b border-sidebar-border px-3 py-2"
			>
				<div>
					<p
						class="text-xs font-medium uppercase tracking-[0.16em] text-sidebar-foreground/60"
					>
						{files.length} changed {files.length === 1 ? "file" : "files"}
					</p>
					{#if approvedCount > 0}
						<p class="mt-1 text-xs text-sidebar-foreground/60">
							{approvedCount} approved
						</p>
					{/if}
				</div>
				<div class="flex items-center gap-3">
					<div class="flex items-center gap-3 text-xs font-medium">
						<span class="text-green-500"
							>+{files.reduce((sum, file) => sum + file.additions, 0)}</span
						>
						<span class="text-red-500"
							>-{files.reduce((sum, file) => sum + file.deletions, 0)}</span
						>
					</div>
					<Button
						variant="ghost"
						size="sm"
						onclick={markAllApproved}
						disabled={allApproved}
					>
						<CheckIcon class="size-4" />
						Mark all approved
					</Button>
				</div>
			</div>

			<div class="min-h-0 flex-1 overflow-y-auto">
				<div class="divide-y divide-sidebar-border">
					{#each files as file (`${file.commitHash}:${file.path}`)}
						{@const expanded = expandedKey === getEntryKey(file)}
						{@const approved = isApproved(file)}
						<section class={cn("flex flex-col", approved && "opacity-80")}>
							<button
								type="button"
								class="flex items-center justify-between gap-3 bg-sidebar/60 px-3 py-2 text-left transition hover:bg-sidebar-accent/70"
								onclick={() => toggleExpanded(file)}
							>
								<div class="flex min-w-0 items-center gap-2">
									{#if expanded}
										<ChevronDownIcon class="size-4 shrink-0" />
									{:else}
										<ChevronRightIcon class="size-4 shrink-0" />
									{/if}
									<Badge
										variant="outline"
										class={cn(
											"inline-grid grid-cols-1 place-items-center",
											statusBadgeClass(file.status),
										)}
									>
										<span class="col-start-1 row-start-1"
											>{statusLabel(file.status)}</span
										>
										<span
											class="invisible col-start-1 row-start-1"
											aria-hidden="true">Modified</span
										>
									</Badge>
									<div class="min-w-0">
										<p class="truncate font-mono text-xs text-foreground">
											{file.path}
										</p>
										{#if file.oldPath && file.oldPath !== file.path}
											<p class="truncate text-[11px] text-muted-foreground">
												{file.oldPath} → {file.path}
											</p>
										{/if}
										<p class="truncate text-[11px] text-muted-foreground">
											{file.commitSubject} • {file.commitHash}
										</p>
									</div>
									{#if approved}
										<span
											class="flex items-center gap-1 text-xs text-green-500"
										>
											<CheckIcon class="size-3.5" />
											Approved
										</span>
									{/if}
								</div>
								<div
									class="flex shrink-0 items-center gap-2 text-xs font-medium"
								>
									{#if file.additions > 0}
										<span class="text-green-500">+{file.additions}</span>
									{/if}
									{#if file.deletions > 0}
										<span class="text-red-500">-{file.deletions}</span>
									{/if}
								</div>
							</button>

							{#if expanded}
								<div class="space-y-3 px-3 py-3">
									<div
										class="flex flex-wrap items-center justify-between gap-2"
									>
										<div
											class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
										>
											{#if file.binary}
												<span>Binary diff</span>
											{:else if file.lineCount > 0}
												<span>{file.lineCount.toLocaleString()} diff lines</span
												>
											{/if}
										</div>
										<div class="flex flex-wrap items-center gap-2">
											<Button
												variant={approved ? "secondary" : "outline"}
												size="sm"
												onclick={() => toggleApproved(file)}
											>
												<CheckIcon class="size-4" />
												{approved ? "Approved" : "Mark approved"}
											</Button>
										</div>
									</div>

									{#if file.binary}
										<div
											class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
										>
											This is a binary file, so the text diff cannot be rendered
											here.
										</div>
									{:else if !file.params}
										<div
											class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
										>
											No textual diff preview is available.
										</div>
									{:else}
										<div
											class="overflow-hidden rounded-md border border-border bg-background"
										>
											<DiffReviewFileRenderer params={file.params} />
										</div>
									{/if}
								</div>
							{/if}
						</section>
					{/each}
				</div>
			</div>
		</div>
	{/if}
</div>
