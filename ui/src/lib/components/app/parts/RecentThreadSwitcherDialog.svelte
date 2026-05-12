<script lang="ts">
	import { recentThreadKey } from "$lib/app/thread-switcher";
	import type { ThreadDisplayStatusValue } from "$lib/app/thread-status";
	import type { RecentThreadSummary } from "$lib/shell-types";
	import ThreadStatusIcon from "$lib/components/app/parts/ThreadStatusIcon.svelte";

	type Props = {
		open: boolean;
		threads: RecentThreadSummary[];
		threadStatuses: Record<string, ThreadDisplayStatusValue>;
		selectedKey: string | null;
		helpText: string;
		onHover: (sessionId: string, threadId: string) => void;
		onSelect: (sessionId: string, threadId: string) => void;
	};

	let {
		open,
		threads,
		threadStatuses,
		selectedKey,
		helpText,
		onHover,
		onSelect,
	}: Props = $props();
	let listRef = $state<HTMLDivElement | null>(null);

	$effect(() => {
		if (!open || !listRef || !selectedKey) {
			return;
		}

		const selectedItem = listRef.querySelector(
			`[data-thread-key="${selectedKey}"]`,
		);
		if (selectedItem && "scrollIntoView" in selectedItem) {
			(selectedItem as HTMLElement).scrollIntoView({ block: "nearest" });
		}
	});
</script>

{#if open}
	<div
		class="pointer-events-none absolute inset-0 z-40 flex items-start justify-center bg-background/20 px-4 pt-24 backdrop-blur-[2px]"
	>
		<div
			class="pointer-events-auto w-full max-w-2xl overflow-hidden rounded-2xl border border-border/80 bg-background/95 shadow-2xl"
		>
			<div
				class="flex items-center justify-between border-b border-border/70 px-4 py-3"
			>
				<p
					class="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground"
				>
					Threads
				</p>
				<p class="text-xs text-muted-foreground">{helpText}</p>
			</div>

			<div
				bind:this={listRef}
				class="max-h-[min(70vh,32rem)] overflow-y-auto p-2"
			>
				{#each threads as thread (`${thread.sessionId}:${thread.threadId}`)}
					{@const threadKey = recentThreadKey(
						thread.sessionId,
						thread.threadId,
					)}
					<button
						type="button"
						data-thread-key={threadKey}
						class={`flex w-full items-start gap-3 rounded-xl px-3 py-3 text-left transition-colors ${
							selectedKey === threadKey
								? "bg-accent text-accent-foreground shadow-sm"
								: "text-foreground/85 hover:bg-accent/70 hover:text-accent-foreground"
						}`}
						onmouseenter={() => onHover(thread.sessionId, thread.threadId)}
						onclick={() => onSelect(thread.sessionId, thread.threadId)}
					>
						<ThreadStatusIcon
							status={threadStatuses[threadKey] ?? "unknown"}
							class="mt-0.5 shrink-0"
						/>
						<span class="min-w-0 flex-1">
							<span class="flex min-w-0 items-center gap-2 overflow-hidden">
								<span class="block truncate text-sm font-medium">
									{thread.name || "New Thread"}
								</span>
							</span>
						</span>
					</button>
				{/each}
			</div>
		</div>
	</div>
{/if}
