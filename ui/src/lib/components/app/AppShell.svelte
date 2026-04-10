<script lang="ts">
	import { onDestroy } from "svelte";
	import type { PaneAPI } from "paneforge";
	import AppHeader from "$lib/components/app/AppHeader.svelte";
	import AppSidebar from "$lib/components/app/AppSidebar.svelte";
	import SessionWorkspace from "$lib/components/app/SessionWorkspace.svelte";
	import StartupTasksBanner from "$lib/components/app/parts/StartupTasksBanner.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import * as Sheet from "$lib/components/ui/sheet";
	import { ensureSessionContext } from "$lib/context/session-context.svelte";
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
	let managedSessionIds: string[] = [];
	let visitedSessionIds = $state<string[]>([]);
	const currentSelectedSessionId = $derived.by(
		() => app.sessions.selectedId ?? app.sessions.pendingId,
	);
	let selectedSession = $state(
		ensureSessionContext(
			app,
			app.sessions.selectedId ?? app.sessions.pendingId,
		),
	);
	const sessionView = $derived.by(() => selectedSession.ui);
	const selectedSessionId = $derived.by(() => selectedSession.sessionId);
	const showSessionToolbar = $derived.by(() => !selectedSession.isPending);
	const mountedSessionIds = $derived.by(() => app.ui.mountedSessionIds);
	const preloadSessionIds = $derived.by(() =>
		mountedSessionIds.filter(
			(sessionId) =>
				sessionId === app.sessions.selectedId ||
				visitedSessionIds.includes(sessionId),
		),
	);
	const renderedSessionIds = $derived.by(() =>
		Array.from(new Set([selectedSessionId, ...preloadSessionIds])),
	);

	function sidebarOpen() {
		return isMobile.current
			? sessionView.mobileSidebarOpen
			: sessionView.desktopSidebarOpen;
	}

	function toggleSidebar() {
		if (isMobile.current) {
			sessionView.mobileSidebarOpen = !sessionView.mobileSidebarOpen;
			return;
		}

		if (!desktopSidebarPane) {
			sessionView.desktopSidebarOpen = !sessionView.desktopSidebarOpen;
			return;
		}

		if (desktopSidebarPane.isCollapsed()) {
			desktopSidebarPane.expand();
			sessionView.desktopSidebarOpen = true;
			return;
		}

		desktopSidebarPane.collapse();
		sessionView.desktopSidebarOpen = false;
	}

	function handleSessionSelect() {
		if (isMobile.current) {
			sessionView.mobileSidebarOpen = false;
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
		const selectedSessionId = app.sessions.selectedId;
		if (!selectedSessionId || visitedSessionIds.includes(selectedSessionId)) {
			return;
		}
		visitedSessionIds = [...visitedSessionIds, selectedSessionId];
	});

	$effect(() => {
		const nextManagedIds = Array.from(
			new Set([...preloadSessionIds, currentSelectedSessionId]),
		);
		selectedSession = ensureSessionContext(app, currentSelectedSessionId);
		for (const sessionId of nextManagedIds) {
			ensureSessionContext(app, sessionId);
		}
		for (const sessionId of managedSessionIds) {
			if (nextManagedIds.includes(sessionId)) {
				continue;
			}
			app.sessions.sessionContexts.get(sessionId)?.dispose();
			app.sessions.sessionContexts.delete(sessionId);
		}
		managedSessionIds = nextManagedIds;
	});

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

		selectedSession.ui.desktopSidebarOpen = !desktopSidebarPane.isCollapsed();
	});

	$effect(() => {
		if (isMobile.current || !desktopSidebarPane || desktopSidebarInitialized) {
			return;
		}

		desktopSidebarInitialized = true;

		if (hasSavedSidebarLayout()) {
			sessionView.desktopSidebarOpen = !desktopSidebarPane.isCollapsed();
			return;
		}

		desktopSidebarPane.expand();
		sessionView.desktopSidebarOpen = true;
	});

	$effect(() => {
		if (isMobile.current || !desktopSidebarPane || !desktopSidebarInitialized) {
			return;
		}

		const paneCollapsed = desktopSidebarPane.isCollapsed();
		if (sessionView.desktopSidebarOpen && paneCollapsed) {
			desktopSidebarPane.expand();
			return;
		}

		if (!sessionView.desktopSidebarOpen && !paneCollapsed) {
			desktopSidebarPane.collapse();
		}
	});

	onDestroy(() => {
		for (const sessionId of managedSessionIds) {
			app.sessions.sessionContexts.get(sessionId)?.dispose();
			app.sessions.sessionContexts.delete(sessionId);
		}
	});
</script>

{#snippet mountedSessions(mainClass: string)}
	{#if selectedSession.isPending}
		<SessionWorkspace
			sessionId={selectedSessionId}
			visible={true}
			{mainClass}
			showSidebarToggle={isMobile.current && !sidebarOpen()}
			reserveSidebarSpace={!isMobile.current && !sidebarOpen()}
			onToggleSidebar={toggleSidebar}
		/>
	{/if}
	{#each renderedSessionIds as sessionId (sessionId)}
		<SessionWorkspace
			{sessionId}
			visible={sessionId === selectedSessionId}
			{mainClass}
			showSidebarToggle={isMobile.current && !sidebarOpen()}
			reserveSidebarSpace={!isMobile.current && !sidebarOpen()}
			onToggleSidebar={toggleSidebar}
		/>
	{/each}
{/snippet}

<div class="h-screen flex flex-col bg-background text-foreground">
	<AppHeader {showSessionToolbar} />
	<StartupTasksBanner startup={app.startup} />

	<div class="flex min-h-0 flex-1 overflow-hidden">
		{#if isMobile.current}
			<Sheet.Root bind:open={sessionView.mobileSidebarOpen}>
				<Sheet.Content
					side="left"
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
							sessionView.desktopSidebarOpen = false;
						}}
						onExpand={() => {
							sessionView.desktopSidebarOpen = true;
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

				{#if !sessionView.desktopSidebarOpen}
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
