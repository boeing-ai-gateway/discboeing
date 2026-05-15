<script lang="ts">
	import { onDestroy, untrack } from "svelte";
	import DockPanel from "$lib/components/app/DockPanel.svelte";
	import ThreadWorkspace from "$lib/components/app/ThreadWorkspace.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { setSessionContext } from "$lib/context/session-context.svelte";
	import { isChatView } from "$lib/session/view/create-session-view-state.svelte";

	type Props = {
		sessionId: string;
		visible: boolean;
		mainClass: string;
		reserveSidebarSpace?: boolean;
	};

	let { sessionId, visible, mainClass, reserveSidebarSpace }: Props = $props();
	const app = useAppContext();
	const session = app.ensureSession(untrack(() => sessionId));
	setSessionContext(session);
	const threadId = $derived.by(() => session.threads.selectedId);
	const showDock = $derived(
		!session.isPending && !isChatView(session.ui.activeView),
	);
	const dockMaximized = $derived(showDock && session.ui.dockMaximized);

	onDestroy(() => {
		if (app.sessions.sessionContexts.get(session.sessionId) === session) {
			app.sessions.sessionContexts.delete(session.sessionId);
			session.dispose();
		}
	});
</script>

<div class={visible ? "contents" : "hidden"}>
	<main class={mainClass}>
		{#if showDock && dockMaximized}
			{#key threadId}
				<ThreadWorkspace {threadId} {visible} mode="connection-only" />
			{/key}
			<div class="min-h-0 flex-1 overflow-hidden">
				<DockPanel />
			</div>
		{:else if showDock}
			<Resizable.PaneGroup
				direction="horizontal"
				autoSaveId="discobot-ui-thread-layout"
				class="min-h-0 min-w-0 flex-1"
			>
				<Resizable.Pane defaultSize={35} minSize={25} class="min-h-0 min-w-0">
					<div class="flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
						{#key threadId}
							<ThreadWorkspace {threadId} {visible} {reserveSidebarSpace} />
						{/key}
					</div>
				</Resizable.Pane>
				<Resizable.Handle class="bg-transparent" />
				<Resizable.Pane defaultSize={65} minSize={25} class="min-h-0 min-w-0">
					<div class="h-full min-h-0 min-w-0 overflow-auto">
						<DockPanel />
					</div>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		{:else}
			{#key threadId}
				<ThreadWorkspace {threadId} {visible} {reserveSidebarSpace} />
			{/key}
		{/if}
	</main>
</div>
