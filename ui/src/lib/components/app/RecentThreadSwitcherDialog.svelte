<script lang="ts">
	import { recentThreadKey } from "$lib/context/view/thread-switcher";
	import AppThreadStatus from "$lib/components/app/AppThreadStatus.svelte";
	import type { RecentThreadEntry } from "$lib/context/view/thread-switcher";

	type Props = {
		open: boolean;
		threads: RecentThreadEntry[];
		selectedKey: string | null;
		helpText: string;
		onHover: (sessionId: string, threadId: string) => void;
		onSelect: (sessionId: string, threadId: string) => void;
		onClose: () => void;
	};

	const titleId = "recent-thread-switcher-title";
	const helpTextId = "recent-thread-switcher-help";

	let {
		open,
		threads,
		selectedKey,
		helpText,
		onHover,
		onSelect,
		onClose,
	}: Props = $props();
	let dialogRef = $state<HTMLDivElement | null>(null);
	let listRef = $state<HTMLDivElement | null>(null);

	function handleDialogKeydown(event: KeyboardEvent) {
		if (event.key !== "Escape") {
			return;
		}

		event.preventDefault();
		onClose();
	}

	$effect(() => {
		if (open) {
			dialogRef?.focus();
		}
	});

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
			bind:this={dialogRef}
			role="dialog"
			aria-modal="true"
			aria-labelledby={titleId}
			aria-describedby={helpTextId}
			tabindex="-1"
			class="pointer-events-auto w-full max-w-2xl overflow-hidden rounded-2xl border border-border/80 bg-background/95 shadow-2xl"
			onkeydown={handleDialogKeydown}
		>
			<div
				class="flex items-center justify-between border-b border-border/70 px-4 py-3"
			>
				<p
					id={titleId}
					class="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground"
				>
					Threads
				</p>
				<p id={helpTextId} class="text-xs text-muted-foreground">{helpText}</p>
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
						aria-current={selectedKey === threadKey ? "true" : undefined}
						class={`flex w-full items-start gap-3 rounded-xl px-3 py-3 text-left transition-colors ${
							selectedKey === threadKey
								? "bg-accent text-accent-foreground shadow-sm"
								: "text-foreground/85 hover:bg-accent/70 hover:text-accent-foreground"
						}`}
						onmouseenter={() => onHover(thread.sessionId, thread.threadId)}
						onclick={() => onSelect(thread.sessionId, thread.threadId)}
					>
						<AppThreadStatus
							sessionId={thread.sessionId}
							threadId={thread.threadId}
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
