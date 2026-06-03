<script lang="ts">
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import PanelLeftIcon from "@lucide/svelte/icons/panel-left";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";
	import SettingsIcon from "@lucide/svelte/icons/settings";
	import AppMacWindowSpacer from "$lib/components/app/AppMacWindowSpacer.svelte";
	import AppSidebar from "$lib/components/app/AppSidebar.svelte";
	import DiscobotBrand from "$lib/components/app/parts/DiscobotBrand.svelte";
	import DiscobotLogo from "$lib/components/app/parts/DiscobotLogo.svelte";
	import RightWindowControls from "$lib/components/app/parts/RightWindowControls.svelte";
	import SessionToolbarStack from "$lib/components/app/SessionToolbarStack.svelte";
	import SettingsDialog from "$lib/components/app/SettingsDialog.svelte";
	import { Button } from "$lib/components/ui/button";
	import {
		Popover,
		PopoverContent,
		PopoverTrigger,
	} from "$lib/components/ui/popover";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { IsMobile } from "$lib/hooks/is-mobile.svelte.js";

	type Props = {
		showSessionToolbar?: boolean;
		showDesktopSidebarToggle?: boolean;
		onToggleSidebar?: () => void;
	};

	let {
		showSessionToolbar = true,
		showDesktopSidebarToggle = false,
		onToggleSidebar,
	}: Props = $props();

	const app = useAppContext();
	const environment = app.environment;
	const sessions = app.sessions;
	const ui = app.ui;
	const updates = app.updates;
	const preferences = app.preferences;
	const isMobile = new IsMobile(1024);
	let desktopSessionsPopoverOpen = $state(false);

	function showWindowsLinuxControls(): boolean {
		return (
			!isMobile.current &&
			environment.supportsNativeWindowControls &&
			environment.windowControlsSide === "right"
		);
	}

	function closeDesktopSessionsPopover(): void {
		desktopSessionsPopoverOpen = false;
	}
</script>

<header
	class={`desktop-drag-region relative grid h-10 items-center bg-background ${isMobile.current ? "grid-cols-[auto_minmax(0,1fr)_auto]" : "grid-cols-[auto_minmax(0,1fr)_auto_auto]"}`}
	data-desktop-drag-region
>
	<div class="absolute inset-0 pointer-events-auto"></div>

	<div class="relative z-20 flex min-w-0 items-center gap-2 pl-4 pr-3">
		{#if isMobile.current}
			<DiscobotLogo size={24} />
			{#if onToggleSidebar}
				<Button
					variant="ghost"
					onclick={onToggleSidebar}
					aria-label="Expand sessions panel"
					title="Expand sessions panel"
					class="desktop-no-drag gap-1 px-1.5 text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground"
				>
					<PanelLeftIcon class="size-3.5" />
					<span>Sessions</span>
				</Button>
			{/if}
		{:else}
			<AppMacWindowSpacer />
			<DiscobotBrand heightClass="h-6" />
			{#if showDesktopSidebarToggle && onToggleSidebar}
				<div
					class="desktop-no-drag inline-flex shrink-0 p-1 translate-y-1 items-center overflow-hidden rounded-md border border-border text-foreground/50"
				>
					<button
						type="button"
						onclick={() => {
							closeDesktopSessionsPopover();
							onToggleSidebar?.();
						}}
						aria-label="Expand sessions panel"
						title="Expand sessions panel"
						class="inline-flex items-center py-0.5 pl-1.5 pr-1 transition-colors hover:text-foreground/80"
					>
						<PanelLeftIcon class="size-3 shrink-0" />
					</button>
					<Popover bind:open={desktopSessionsPopoverOpen}>
						<PopoverTrigger>
							{#snippet child({ props })}
								<button
									{...props}
									type="button"
									aria-label="Open sessions menu"
									title="Open sessions menu"
									class="inline-flex shrink-0 items-center gap-0.5 py-0.5 pl-0 pr-1.5 text-xs font-medium uppercase tracking-[0.16em] transition-colors hover:text-foreground/80"
								>
									<span>Sessions</span>
									<ChevronDownIcon
										class={`size-3 shrink-0 transition-transform ${desktopSessionsPopoverOpen ? "rotate-180" : ""}`}
									/>
								</button>
							{/snippet}
						</PopoverTrigger>
						<PopoverContent align="start" class="w-auto bg-sidebar p-0">
							<AppSidebar
								mode="dropdown"
								onThreadSelect={closeDesktopSessionsPopover}
							/>
						</PopoverContent>
					</Popover>
				</div>
			{/if}
		{/if}
	</div>

	<div class="relative z-20 min-w-0 px-2">
		{#if showSessionToolbar}
			<SessionToolbarStack />
		{/if}
	</div>

	<div class="relative z-20 flex min-w-0 items-center justify-end gap-2">
		<button
			type="button"
			onclick={() => sessions.startNew()}
			aria-label="New session"
			title="New session"
			class="desktop-no-drag inline-flex shrink-0 items-center gap-1 rounded-md px-1 py-0.5 text-xs font-medium uppercase tracking-[0.16em] text-foreground/50 transition-colors hover:text-foreground/80"
		>
			<PlusIcon class="size-3 shrink-0" />
			<span>{isMobile.current ? "New" : "New Session"}</span>
		</button>

		<div class="flex min-w-0 items-center gap-1 pr-2">
			{#if preferences.showRefreshButton}
				<Button
					variant="ghost"
					size="icon-sm"
					onclick={() => location.reload()}
					aria-label="Refresh"
					title="Refresh"
					class="desktop-no-drag"
				>
					<RefreshCwIcon class="size-4" />
				</Button>
			{/if}
			<Button
				variant="ghost"
				size="icon-sm"
				onclick={() => ui.openSettings()}
				aria-label="Settings"
				title="Settings"
				class="desktop-no-drag relative"
			>
				<SettingsIcon class="size-4" />
				{#if updates.showBadge}
					<span class="absolute right-1 top-1 h-2 w-2 rounded-full bg-primary"
					></span>
				{/if}
			</Button>
		</div>
	</div>

	{#if !isMobile.current}
		<div
			class="desktop-no-drag relative z-20 flex h-full min-w-0 items-stretch justify-self-end pr-0"
		>
			{#if showWindowsLinuxControls()}
				<RightWindowControls />
			{/if}
		</div>
	{/if}

	<SettingsDialog />
</header>
