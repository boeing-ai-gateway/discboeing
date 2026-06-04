<script lang="ts">
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import AppSidebar from "$lib/components/app/AppSidebar.svelte";
	import {
		Popover,
		PopoverContent,
		PopoverTrigger,
	} from "$lib/components/ui/popover";
	import { setDesktopSidebarOpen } from "$lib/context/commands/app-view";
	import { useContext } from "$lib/context/context.svelte";

	type Props = {
		label?: string;
	};

	let { label = "Sessions" }: Props = $props();

	const context = useContext();
	let open = $state(false);
	const selectedSession = $derived.by(() => {
		const selectedSessionId = context.view.app.selection.sessionId;
		return selectedSessionId
			? (context.data.sessions.byId[selectedSessionId] ?? null)
			: null;
	});
	const triggerLabel = $derived.by(
		() =>
			label ||
			selectedSession?.displayName ||
			selectedSession?.name ||
			"Sessions",
	);
	const showingFallbackLabel = $derived(triggerLabel === "Sessions");
	const triggerClass = $derived(
		showingFallbackLabel
			? "inline-flex h-8 max-w-72 min-w-0 items-center gap-0.5 rounded-md px-2 text-xs font-medium uppercase tracking-[0.16em] text-foreground/50 transition-colors hover:text-foreground/80"
			: "inline-flex h-8 max-w-72 min-w-0 items-center gap-1 rounded-md px-2 text-sm font-medium transition-colors hover:bg-tree-hover",
	);

	function close() {
		open = false;
	}

	function pinSidebar() {
		setDesktopSidebarOpen(true);
		close();
	}
</script>

<Popover bind:open>
	<PopoverTrigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				aria-label="Open sessions menu"
				title="Open sessions menu"
				class={triggerClass}
			>
				<span class="truncate">{triggerLabel}</span>
				<ChevronDownIcon
					class={`size-3.5 shrink-0 transition-transform ${open ? "rotate-180" : ""}`}
				/>
			</button>
		{/snippet}
	</PopoverTrigger>
	<PopoverContent align="start" class="w-auto bg-sidebar p-0">
		<AppSidebar
			mode="dropdown"
			onThreadSelect={close}
			onPinSidebar={pinSidebar}
		/>
	</PopoverContent>
</Popover>
