<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import DiffReviewFileRenderer from "$lib/components/app/parts/DiffReviewFileRenderer.svelte";
	import Skeleton from "$lib/components/ui/skeleton/skeleton.svelte";
	import { cn } from "$lib/utils";
	import type { RequestCommitPullDiffEntry } from "./request-commit-pull-diff";

	type Props = {
		file: RequestCommitPullDiffEntry;
	};

	let { file }: Props = $props();
	let isRendering = $state(false);
	let loadingStartedAt = 0;
	let loadingTimer: ReturnType<typeof setTimeout> | null = null;

	$effect(() => {
		if (file.params) {
			loadingStartedAt = Date.now();
			isRendering = true;
		}
		return () => {
			if (loadingTimer) {
				clearTimeout(loadingTimer);
				loadingTimer = null;
			}
		};
	});

	function getStatusLabel(
		status: RequestCommitPullDiffEntry["status"],
	): string {
		switch (status) {
			case "added":
				return "Added";
			case "deleted":
				return "Deleted";
			case "renamed":
				return "Renamed";
			default:
				return "Modified";
		}
	}

	function getStatusClasses(
		status: RequestCommitPullDiffEntry["status"],
	): string {
		switch (status) {
			case "added":
				return "border-green-500/20 bg-green-500/10 text-green-700 dark:text-green-300";
			case "deleted":
				return "border-red-500/20 bg-red-500/10 text-red-700 dark:text-red-300";
			case "renamed":
				return "border-blue-500/20 bg-blue-500/10 text-blue-700 dark:text-blue-300";
			default:
				return "border-border bg-muted text-muted-foreground";
		}
	}

	function handleRenderStateChange(rendering: boolean) {
		if (loadingTimer) {
			clearTimeout(loadingTimer);
			loadingTimer = null;
		}
		if (rendering) {
			loadingStartedAt = Date.now();
			isRendering = true;
			return;
		}

		const elapsed = Date.now() - loadingStartedAt;
		const remaining = Math.max(150 - elapsed, 0);
		if (remaining === 0) {
			isRendering = false;
			return;
		}

		loadingTimer = setTimeout(() => {
			isRendering = false;
			loadingTimer = null;
		}, remaining);
	}
</script>

<div class="space-y-3 rounded-md border bg-background/70 p-3 text-sm">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div class="min-w-0">
			<p class="font-mono text-xs">
				{file.path}
				{#if file.oldPath}
					<span class="text-muted-foreground"> ← {file.oldPath}</span>
				{/if}
			</p>
			<p class="mt-1 text-muted-foreground text-xs">
				{file.commitSubject} • {file.commitHash}
			</p>
		</div>
		<div class="flex flex-wrap gap-2 text-xs">
			<span
				class={cn(
					"rounded-full border px-2 py-0.5",
					getStatusClasses(file.status),
				)}
			>
				{getStatusLabel(file.status)}
			</span>
			<span class="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
				+{file.additions} / -{file.deletions}
			</span>
		</div>
	</div>

	{#if file.binary}
		<div
			class="rounded-md border border-dashed bg-muted/20 p-3 text-muted-foreground text-xs"
		>
			Binary patch preview is not available.
		</div>
	{:else if file.params}
		<div
			class="relative min-h-64 overflow-hidden rounded-md border bg-background/80"
		>
			{#if isRendering}
				<div
					class="absolute inset-0 z-10 flex flex-col justify-center gap-3 bg-muted/70 p-4 backdrop-blur-[1px]"
				>
					<div
						class="flex items-center gap-2 font-medium text-foreground text-sm"
					>
						<Loader2Icon class="size-4 animate-spin" />
						<span>Rendering diff preview for {file.path}…</span>
					</div>
					<p class="text-muted-foreground text-xs">
						Large or complex diffs can take a moment to appear.
					</p>
					<div class="space-y-2 rounded-md border bg-background/80 p-3">
						<Skeleton class="h-5 w-40 bg-muted-foreground/15" />
						<Skeleton class="h-24 w-full bg-muted-foreground/10" />
						<Skeleton class="h-20 w-11/12 bg-muted-foreground/10" />
					</div>
				</div>
			{/if}
			<div class:opacity-0={isRendering}>
				<DiffReviewFileRenderer
					params={file.params}
					onRenderStateChange={handleRenderStateChange}
				/>
			</div>
		</div>
	{:else}
		<div
			class="rounded-md border border-dashed bg-muted/20 p-3 text-muted-foreground text-xs"
		>
			No textual diff preview is available.
		</div>
	{/if}
</div>
