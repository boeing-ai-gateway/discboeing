<script lang="ts">
	import type { Snippet } from "svelte";
	import { cn } from "$lib/utils";

	type Props = {
		dockMaximized: boolean;
		onClose: () => void;
		onToggleDockMaximized: () => void;
		shiftWindowControlsForSidebar?: boolean;
		closeLabel: string;
		minimizeLabel: string;
		maximizeTitle: string;
		shellClass?: string;
		headerClass?: string;
		contentClass?: string;
		maximizeRingOffsetClass?: string;
		title?: Snippet;
		actions?: Snippet;
		children?: Snippet;
	};

	let {
		dockMaximized,
		onClose,
		onToggleDockMaximized,
		shiftWindowControlsForSidebar = false,
		closeLabel,
		minimizeLabel,
		maximizeTitle,
		shellClass,
		headerClass,
		contentClass,
		maximizeRingOffsetClass = "ring-offset-sidebar",
		title,
		actions,
		children,
	}: Props = $props();

	function handleHeaderDoubleClick(event: MouseEvent) {
		const target = event.target;
		if (!(target instanceof HTMLElement)) {
			onToggleDockMaximized();
			return;
		}

		if (
			target.closest(
				'button, a, input, label, select, textarea, [role="button"], [data-dock-header-double-click-ignore]',
			)
		) {
			return;
		}

		onToggleDockMaximized();
	}
</script>

<div
	class={cn(
		"flex h-full flex-col overflow-hidden rounded-md border border-sidebar-border bg-sidebar text-sidebar-foreground",
		shellClass,
	)}
>
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class={cn(
			"flex h-10 shrink-0 items-center justify-between gap-3 border-b border-sidebar-border px-3",
			headerClass,
		)}
		ondblclick={handleHeaderDoubleClick}
	>
		<div class="flex min-w-0 items-center gap-2">
			<div
				class={cn(
					"flex shrink-0 gap-1.5",
					shiftWindowControlsForSidebar && "ml-36",
				)}
			>
				<button
					type="button"
					onclick={onClose}
					class="size-3 rounded-full bg-red-500 transition-opacity hover:opacity-80"
					aria-label={closeLabel}
					title={closeLabel}
				></button>
				<button
					type="button"
					onclick={onClose}
					class="size-3 rounded-full bg-yellow-500 transition-opacity hover:opacity-80"
					aria-label={minimizeLabel}
					title={minimizeLabel}
				></button>
				<button
					type="button"
					onclick={onToggleDockMaximized}
					class={cn(
						"size-3 rounded-full bg-green-500 transition-opacity hover:opacity-80",
						dockMaximized && "ring-2 ring-white/60 ring-offset-2",
						dockMaximized && maximizeRingOffsetClass,
					)}
					aria-label={maximizeTitle}
					title={maximizeTitle}
				></button>
			</div>
			{@render title?.()}
		</div>
		{#if actions}
			<div class="flex items-center gap-2">
				{@render actions()}
			</div>
		{/if}
	</div>

	<div class={cn("min-h-0 flex-1", contentClass)}>
		{@render children?.()}
	</div>
</div>
