<script lang="ts">
	import type { PaneAPI } from "paneforge";
	import AppHeader from "$lib/components/app/AppHeader.svelte";
	import AppKeyboardShortcuts from "$lib/components/app/AppKeyboardShortcuts.svelte";
	import AppSidebar from "$lib/components/app/AppSidebar.svelte";
	import SessionWorkspace from "$lib/components/app/SessionWorkspace.svelte";
	import StartupTasksBanner from "$lib/components/app/parts/StartupTasksBanner.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import * as Sheet from "$lib/components/ui/sheet";
	import { useContext } from "$lib/context/context.svelte";
	import {
		setDesktopSidebarOpen,
		setMobileSidebarOpen,
		shouldLoadSessionWorkspace,
	} from "$lib/context/commands/app-view";

	const context = useContext();
	const appEnvironment = $derived(context.view.app.environment);
	const SIDEBAR_LAYOUT_STORAGE_KEY = "paneforge:discobot-ui-sidebar-layout";
	const SIDEBAR_MIN_WIDTH_PX = 300;
	const SIDEBAR_MIN_SIZE_FALLBACK = 10;
	let desktopPaneGroupElement = $state<HTMLDivElement | null>(null);
	let desktopSidebarMinSize = $state(SIDEBAR_MIN_SIZE_FALLBACK);
	let desktopSidebarPane = $state<PaneAPI | null>(null);
	let desktopSidebarInitialized = $state(false);
	const currentSelectedSessionId = $derived.by(
		() =>
			context.view.app.selection.sessionId ??
			context.view.app.selection.pendingSessionId,
	);
	const showSessionToolbar = $derived.by(() => !!currentSelectedSessionId);
	const mountedSessionIds = $derived.by(
		() => context.view.app.navigation.mountedSessionIds,
	);
	const visibleStartupTasks = $derived.by(() =>
		context.view.app.startupTasks.visibleIds
			.map((taskId) => context.data.startupTasks.byId[taskId])
			.filter((task) => task !== undefined),
	);

	function toggleSidebar() {
		if (appEnvironment.isMobile) {
			setMobileSidebarOpen(!context.view.app.navigation.mobileSidebarOpen);
			return;
		}

		if (!desktopSidebarPane) {
			setDesktopSidebarOpen(!context.view.app.navigation.desktopSidebarOpen);
			return;
		}

		if (desktopSidebarPane.isCollapsed()) {
			desktopSidebarPane.expand();
			setDesktopSidebarOpen(true);
			return;
		}

		desktopSidebarPane.collapse();
		setDesktopSidebarOpen(false);
	}

	function handleSessionSelect() {
		if (appEnvironment.isMobile) {
			setMobileSidebarOpen(false);
		}
	}

	function hasSavedSidebarLayout() {
		return (
			typeof window !== "undefined" &&
			window.localStorage.getItem(SIDEBAR_LAYOUT_STORAGE_KEY)
		);
	}

	function updateDesktopSidebarMinSize(width: number) {
		if (!Number.isFinite(width) || width <= 0) {
			return;
		}
		desktopSidebarMinSize = Math.min(
			48,
			Math.max(SIDEBAR_MIN_SIZE_FALLBACK, (SIDEBAR_MIN_WIDTH_PX / width) * 100),
		);
	}

	$effect(() => {
		if (appEnvironment.isMobile || !desktopPaneGroupElement) {
			return;
		}

		updateDesktopSidebarMinSize(desktopPaneGroupElement.clientWidth);
		const resizeObserver = new ResizeObserver((entries) => {
			const entry = entries[0];
			if (!entry) {
				return;
			}
			updateDesktopSidebarMinSize(entry.contentRect.width);
		});
		resizeObserver.observe(desktopPaneGroupElement);

		return () => {
			resizeObserver.disconnect();
		};
	});

	$effect(() => {
		if (
			appEnvironment.isMobile ||
			!desktopSidebarPane ||
			!desktopSidebarInitialized
		) {
			return;
		}

		setDesktopSidebarOpen(!desktopSidebarPane.isCollapsed());
	});

	$effect(() => {
		if (
			appEnvironment.isMobile ||
			!desktopSidebarPane ||
			desktopSidebarInitialized
		) {
			return;
		}

		desktopSidebarInitialized = true;

		if (hasSavedSidebarLayout()) {
			setDesktopSidebarOpen(!desktopSidebarPane.isCollapsed());
			return;
		}

		desktopSidebarPane.expand();
		setDesktopSidebarOpen(true);
	});

	$effect(() => {
		if (
			appEnvironment.isMobile ||
			!desktopSidebarPane ||
			!desktopSidebarInitialized
		) {
			return;
		}

		const paneCollapsed = desktopSidebarPane.isCollapsed();
		if (context.view.app.navigation.desktopSidebarOpen && paneCollapsed) {
			desktopSidebarPane.expand();
			return;
		}

		if (!context.view.app.navigation.desktopSidebarOpen && !paneCollapsed) {
			desktopSidebarPane.collapse();
		}
	});
</script>

{#snippet mountedSessions(mainClass: string)}
	{#each mountedSessionIds as sessionId (sessionId)}
		{#if shouldLoadSessionWorkspace(sessionId, { includePending: true })}
			<SessionWorkspace
				{sessionId}
				visible={sessionId === currentSelectedSessionId}
				{mainClass}
			/>
		{/if}
	{/each}
{/snippet}

<div class="h-[100dvh] flex flex-col bg-background text-foreground">
	<AppKeyboardShortcuts />
	<AppHeader {showSessionToolbar} onToggleSidebar={toggleSidebar} />
	<StartupTasksBanner
		tasks={visibleStartupTasks}
		hasActiveTasks={context.view.app.startupTasks.hasActiveTasks}
	/>

	<div class="flex min-h-0 flex-1 overflow-hidden">
		{#if appEnvironment.isMobile}
			<Sheet.Root bind:open={context.view.app.navigation.mobileSidebarOpen}>
				<Sheet.Content
					side="left"
					overlayClass="bg-transparent"
					class="w-64 max-w-none bg-background p-3 [&>button]:hidden"
				>
					<AppSidebar
						onThreadSelect={handleSessionSelect}
						onToggleSidebar={toggleSidebar}
					/>
				</Sheet.Content>
			</Sheet.Root>

			<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
				{@render mountedSessions(
					"flex min-h-0 flex-1 flex-col overflow-hidden pt-1",
				)}
			</div>
		{:else}
			<div
				bind:this={desktopPaneGroupElement}
				class="relative min-h-0 min-w-0 flex-1"
			>
				<Resizable.PaneGroup
					direction="horizontal"
					autoSaveId="discobot-ui-sidebar-layout"
					class="min-h-0 min-w-0 flex-1"
				>
					<Resizable.Pane
						bind:this={desktopSidebarPane}
						defaultSize={16}
						minSize={desktopSidebarMinSize}
						maxSize={50}
						collapsible
						collapsedSize={0}
						onCollapse={() => {
							setDesktopSidebarOpen(false);
						}}
						onExpand={() => {
							setDesktopSidebarOpen(true);
						}}
					>
						<div class="box-border h-full min-h-0 pb-3 pl-3 pr-2 pt-1">
							<AppSidebar onToggleSidebar={toggleSidebar} />
						</div>
					</Resizable.Pane>
					<Resizable.Handle class="bg-transparent after:w-3" />
					<Resizable.Pane minSize={45} class="min-h-0 min-w-0">
						<div class="relative h-full min-h-0">
							{@render mountedSessions(
								"flex h-full min-h-0 flex-col overflow-hidden pt-1",
							)}
						</div>
					</Resizable.Pane>
				</Resizable.PaneGroup>
			</div>
		{/if}
	</div>
</div>
