<script lang="ts">
	import { onDestroy } from "svelte";
	import ConversationPane from "$lib/components/app/ConversationPane.svelte";
	import DockPanel from "$lib/components/app/DockPanel.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import { useThreadContext } from "$lib/context/thread-context.svelte";
	import { isChatView } from "$lib/session/view/create-session-view-state.svelte";

	type Props = {
		visible: boolean;
		reserveSidebarSpace?: boolean;
		mode?: "full" | "conversation-only";
	};

	const props: Props = $props();

	const session = useSessionContext();
	const thread = useThreadContext();

	$effect(() => {
		if (!session.current) {
			return;
		}
		void thread.connect();
	});

	onDestroy(() => {
		if (session.threadContexts.get(thread.threadId) === thread) {
			session.threadContexts.delete(thread.threadId);
		}
		thread.dispose();
	});

	const showDock = $derived(
		(props.mode ?? "full") === "full" && !isChatView(session.ui.activeView),
	);
	const dockMaximized = $derived(showDock && session.ui.dockMaximized);
</script>

{#if showDock && dockMaximized}
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
				<ThreadWorkspaceHeader
					reserveSidebarSpace={props.reserveSidebarSpace ?? false}
					title={session.threads.selected?.name ??
						(session.isPending ? "" : "No thread selected")}
					state={session.threads.selected?.state}
				/>
				<div class="min-h-0 min-w-0 flex-1 overflow-hidden">
					<ConversationPane visible={props.visible} />
				</div>
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
	<ThreadWorkspaceHeader
		reserveSidebarSpace={props.reserveSidebarSpace ?? false}
		title={session.threads.selected?.name ??
			(session.isPending ? "" : "No thread selected")}
		state={session.threads.selected?.state}
	/>

	<div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
		<ConversationPane visible={props.visible} />
	</div>
{/if}
