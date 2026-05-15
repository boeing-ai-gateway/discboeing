<script lang="ts">
	import type { PaneAPI } from "paneforge";
	import AppHeader from "$lib/components/app/AppHeader.svelte";
	import AppKeyboardShortcuts from "$lib/components/app/AppKeyboardShortcuts.svelte";
	import AppSidebar from "$lib/components/app/AppSidebar.svelte";
	import SessionWorkspace from "$lib/components/app/SessionWorkspace.svelte";
	import StartupTasksBanner from "$lib/components/app/parts/StartupTasksBanner.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import * as Sheet from "$lib/components/ui/sheet";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { IsMobile } from "$lib/hooks/is-mobile.svelte.js";

	const app = useAppContext();
	const isMobile = new IsMobile(1024);
	const SIDEBAR_LAYOUT_STORAGE_KEY = "paneforge:discobot-ui-sidebar-layout";
	const SIDEBAR_MIN_WIDTH_PX = 300;
	const SIDEBAR_MIN_SIZE_FALLBACK = 10;
	let desktopPaneGroupElement = $state<HTMLDivElement | null>(null);
	let desktopSidebarMinSize = $state(SIDEBAR_MIN_SIZE_FALLBACK);
	let desktopSidebarPane = $state<PaneAPI | null>(null);
	let desktopSidebarInitialized = $state(false);
	const currentSelectedSessionId = $derived.by(
		() => app.sessions.selectedId ?? app.sessions.pendingId,
	);
	const showSessionToolbar = $derived.by(() => !!currentSelectedSessionId);
	const mountedSessionIds = $derived.by(() => app.ui.mountedSessionIds);

	function sidebarOpen() {
		return isMobile.current
			? app.ui.mobileSidebarOpen
			: app.ui.desktopSidebarOpen;
	}

	function toggleSidebar() {
		if (isMobile.current) {
			app.ui.setMobileSidebarOpen(!app.ui.mobileSidebarOpen);
			return;
		}

		if (!desktopSidebarPane) {
			app.ui.setDesktopSidebarOpen(!app.ui.desktopSidebarOpen);
			return;
		}

		if (desktopSidebarPane.isCollapsed()) {
			desktopSidebarPane.expand();
			app.ui.setDesktopSidebarOpen(true);
			return;
		}

		desktopSidebarPane.collapse();
		app.ui.setDesktopSidebarOpen(false);
	}

	function handleSessionSelect() {
		if (isMobile.current) {
			app.ui.setMobileSidebarOpen(false);
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
		if (isMobile.current || !desktopPaneGroupElement) {
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
		if (isMobile.current || !desktopSidebarPane || !desktopSidebarInitialized) {
			return;
		}

		app.ui.setDesktopSidebarOpen(!desktopSidebarPane.isCollapsed());
	});

	$effect(() => {
		if (isMobile.current || !desktopSidebarPane || desktopSidebarInitialized) {
			return;
		}

		desktopSidebarInitialized = true;

		if (hasSavedSidebarLayout()) {
			app.ui.setDesktopSidebarOpen(!desktopSidebarPane.isCollapsed());
			return;
		}

		desktopSidebarPane.expand();
		app.ui.setDesktopSidebarOpen(true);
	});

	$effect(() => {
		if (isMobile.current || !desktopSidebarPane || !desktopSidebarInitialized) {
			return;
		}

		const paneCollapsed = desktopSidebarPane.isCollapsed();
		if (app.ui.desktopSidebarOpen && paneCollapsed) {
			desktopSidebarPane.expand();
			return;
		}

		if (!app.ui.desktopSidebarOpen && !paneCollapsed) {
			desktopSidebarPane.collapse();
		}
	});
</script>

{#snippet mountedSessions(mainClass: string)}
	{#each mountedSessionIds as sessionId (sessionId)}
		{#if app.sessions.shouldLoadSession(sessionId, { includePending: true })}
			<SessionWorkspace
				{sessionId}
				visible={sessionId === currentSelectedSessionId}
				{mainClass}
				reserveSidebarSpace={!isMobile.current && !sidebarOpen()}
			/>
		{/if}
	{/each}
{/snippet}

<div class="h-[100dvh] flex flex-col bg-background text-foreground">
	<AppKeyboardShortcuts />
	<AppHeader {showSessionToolbar} onToggleSidebar={toggleSidebar} />
	<StartupTasksBanner startup={app.startup} />

	<div class="flex min-h-0 flex-1 overflow-hidden">
		{#if isMobile.current}
			<Sheet.Root bind:open={app.ui.mobileSidebarOpen}>
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
							app.ui.setDesktopSidebarOpen(false);
						}}
						onExpand={() => {
							app.ui.setDesktopSidebarOpen(true);
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

				{#if !app.ui.desktopSidebarOpen}
					<div
						class="pointer-events-none absolute inset-y-0 left-0 z-20 box-border pb-3 pl-3 pr-2 pt-1"
					>
						<div class="pointer-events-auto">
							<AppSidebar
								mode="floating"
								collapsed
								onToggleSidebar={toggleSidebar}
							/>
						</div>
					</div>
				{/if}
			</div>
		{/if}
	</div>
</div>
