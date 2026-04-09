<script lang="ts">
	import { onDestroy } from "svelte";
	import type { PaneAPI } from "paneforge";
	import {
		getMountedSessionIds,
		RECENT_SESSIONS_LIMIT,
	} from "$lib/app/app-helpers";
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
	let desktopSidebarPane = $state<PaneAPI | null>(null);
	let desktopSidebarInitialized = $state(false);
	let managedSessionIds: string[] = [];
	let visitedSessionIds = $state<string[]>([]);
	const currentSelectedSessionId = $derived.by(
		() => app.sessions.selectedId ?? app.sessions.pendingId,
	);
	let selectedSession = $state(
		ensureSessionContext(app.sessions.selectedId ?? app.sessions.pendingId),
	);
	const sessionView = $derived.by(() => selectedSession.ui);
	const selectedSessionId = $derived.by(() => selectedSession.sessionId);
	const showSessionToolbar = $derived.by(() => !selectedSession.isPending);
	const mountedSessionIds = $derived.by(() =>
		getMountedSessionIds(
			app.sessions.selectedId,
			app.sessions.recentThreads,
			RECENT_SESSIONS_LIMIT,
		),
	);
	const preloadSessionIds = $derived.by(() =>
		mountedSessionIds.filter(
			(sessionId) =>
				sessionId === app.sessions.selectedId ||
				visitedSessionIds.includes(sessionId),
		),
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
		selectedSession = ensureSessionContext(currentSelectedSessionId);
		for (const sessionId of nextManagedIds) {
			ensureSessionContext(sessionId);
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
			sidebarOpen={sidebarOpen()}
			onToggleSidebar={toggleSidebar}
		/>
	{/if}
	{#each preloadSessionIds as sessionId (sessionId)}
		<SessionWorkspace
			{sessionId}
			visible={sessionId === selectedSessionId}
			{mainClass}
			sidebarOpen={sidebarOpen()}
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
					"flex min-h-0 flex-1 flex-col overflow-hidden",
				)}
			</div>
		{:else}
			<Resizable.PaneGroup
				direction="horizontal"
				autoSaveId="discobot-ui-sidebar-layout"
				class="min-h-0 flex-1"
			>
				<Resizable.Pane
					bind:this={desktopSidebarPane}
					defaultSize={16}
					minSize={10}
					maxSize={35}
					collapsible
					collapsedSize={0}
					onCollapse={() => {
						sessionView.desktopSidebarOpen = false;
					}}
					onExpand={() => {
						sessionView.desktopSidebarOpen = true;
					}}
				>
					<div class="box-border h-full min-h-0 py-3 pl-3 pr-2">
						<AppSidebar onToggleSidebar={toggleSidebar} />
					</div>
				</Resizable.Pane>
				<Resizable.Handle class="bg-transparent after:w-3" />
				<Resizable.Pane minSize={45} class="min-h-0">
					<div class="relative h-full min-h-0">
						{@render mountedSessions(
							"flex h-full min-h-0 flex-col overflow-hidden",
						)}
					</div>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		{/if}
	</div>
</div>
