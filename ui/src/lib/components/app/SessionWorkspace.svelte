<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import type { PaneAPI } from "paneforge";
	import AppHeader from "$lib/components/app/AppHeader.svelte";
	import StartupTasksBanner from "$lib/components/app/parts/StartupTasksBanner.svelte";
	import ThreadWorkspace from "$lib/components/app/ThreadWorkspace.svelte";
	import SessionSidebar from "$lib/components/app/SessionSidebar.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import * as Sheet from "$lib/components/ui/sheet";
	import { setSessionContext } from "$lib/context/session-context.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { IsMobile } from "$lib/hooks/is-mobile.svelte.js";

	const app = useAppContext();
	const isMobile = new IsMobile(1024);
	const SIDEBAR_LAYOUT_STORAGE_KEY = "paneforge:discobot-ui-sidebar-layout";
	const SIDEBAR_MIN_WIDTH_PX = 40;
	const SIDEBAR_MIN_SIZE_FALLBACK = 4;
	let desktopPaneGroupElement = $state<HTMLDivElement | null>(null);
	let desktopSidebarMinSize = $state(SIDEBAR_MIN_SIZE_FALLBACK);
	let desktopSidebarPane = $state<PaneAPI | null>(null);
	let desktopSidebarInitialized = $state(false);

	const session = setSessionContext();
	const sessionView = session.ui;

	onMount(() => {
		void session.load();
	});

	onDestroy(() => {
		if (app.sessions.sessionContexts.get(session.sessionId) !== session) {
			return;
		}
		session.dispose();
		app.sessions.sessionContexts.delete(session.sessionId);
	});

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

	function hasSavedSidebarLayout() {
		return (
			typeof window !== "undefined" &&
			window.localStorage.getItem(SIDEBAR_LAYOUT_STORAGE_KEY)
		);
	}

	$effect(() => {
		if (isMobile.current || !desktopSidebarPane || desktopSidebarInitialized) {
			return;
		}

		desktopSidebarInitialized = true;

		if (hasSavedSidebarLayout()) {
			sessionView.desktopSidebarOpen = !desktopSidebarPane.isCollapsed();
			return;
		}

		// Default to expanded — sessions list is primary navigation
		desktopSidebarPane.expand();
		sessionView.desktopSidebarOpen = true;
	});
</script>

<div class="h-screen flex flex-col bg-background text-foreground">
	<AppHeader showSessionToolbar={!session.isPending} />
	<StartupTasksBanner startup={app.startup} />

	<div class="flex min-h-0 flex-1 overflow-hidden">
		{#if isMobile.current}
			<Sheet.Root bind:open={sessionView.mobileSidebarOpen}>
				<Sheet.Content
					side="left"
					class="w-64 max-w-none bg-background p-3 [&>button]:hidden"
				>
					<SessionSidebar
						onThreadSelect={handleSessionSelect}
						onToggleSidebar={toggleSidebar}
					/>
				</Sheet.Content>
			</Sheet.Root>

			{#key session.threads.selectedId ?? session.sessionId}
				<ThreadWorkspace
					mainClass="flex min-h-0 flex-1 flex-col overflow-hidden pt-1"
					showSidebarToggle={!sidebarOpen()}
					reserveSidebarSpace={false}
					onToggleSidebar={toggleSidebar}
					mode={session.isPending ? "conversation-only" : undefined}
				/>
			{/key}
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
						defaultSize={24}
						minSize={desktopSidebarMinSize}
						maxSize={48}
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
							<SessionSidebar onToggleSidebar={toggleSidebar} />
						</div>
					</Resizable.Pane>
					<Resizable.Handle class="bg-transparent after:w-3" />
					<Resizable.Pane minSize={45} class="min-h-0 min-w-0">
						{#key session.threads.selectedId ?? session.sessionId}
							<ThreadWorkspace
								mainClass="flex h-full min-h-0 flex-col overflow-hidden pt-1"
								reserveSidebarSpace={!sessionView.desktopSidebarOpen}
								mode={session.isPending ? "conversation-only" : undefined}
							/>
						{/key}
					</Resizable.Pane>
				</Resizable.PaneGroup>

				{#if !sessionView.desktopSidebarOpen}
					<div
						class="pointer-events-none absolute inset-y-0 left-0 z-20 box-border pb-3 pl-3 pr-2 pt-1"
					>
						<div class="pointer-events-auto">
							<SessionSidebar
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
