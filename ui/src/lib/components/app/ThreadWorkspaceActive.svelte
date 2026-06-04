<script lang="ts">
	import { onDestroy } from "svelte";
	import ConversationPane from "$lib/components/app/ConversationPane.svelte";
	import DockPanel from "$lib/components/app/DockPanel.svelte";
	import SessionHeaderDropdown from "$lib/components/app/SessionHeaderDropdown.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import {
		connectThread,
		releaseThreadState,
	} from "$lib/context/commands/session";
	import type {
		SessionContextValue,
		ThreadContextValue,
	} from "$lib/session/session-context.types";
	import { isChatView } from "$lib/session/view/create-session-view-state.svelte";

	type Props = {
		session: SessionContextValue;
		thread: ThreadContextValue;
		visible: boolean;
		reserveSidebarSpace?: boolean;
		mode?: "full" | "conversation-only";
	};

	const props: Props = $props();
	const session = $derived(props.session);
	const thread = $derived(props.thread);

	$effect(() => {
		if (!props.visible || !session.current) {
			return;
		}
		connectThread(session.sessionId, thread.threadId);
	});

	onDestroy(() => {
		releaseThreadState(session.sessionId, thread);
	});

	const showDock = $derived(
		(props.mode ?? "full") === "full" && !isChatView(session.ui.activeView),
	);
	const dockMaximized = $derived(showDock && session.ui.dockMaximized);
	const headerTitle = $derived.by(() => {
		if (session.threads.selected?.name) {
			return session.threads.selected.name;
		}
		if (session.isPending) {
			return "";
		}
		return "";
	});
	const sessionTitle = $derived.by(
		() => session.current?.displayName || session.current?.name || "Sessions",
	);
</script>

{#snippet sessionHeaderDropdown()}
	<SessionHeaderDropdown label={sessionTitle} />
{/snippet}

{#if showDock && dockMaximized}
	<div class="min-h-0 flex-1 overflow-hidden">
		<DockPanel
			sessionId={session.sessionId}
			threadId={thread.threadId}
			sessionView={session.ui}
		/>
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
					title={headerTitle}
					state={session.threads.selected?.state}
					titleContent={sessionHeaderDropdown}
				/>
				<div class="min-h-0 min-w-0 flex-1 overflow-hidden">
					{#if props.visible}
						<ConversationPane {session} {thread} visible={props.visible} />
					{/if}
				</div>
			</div>
		</Resizable.Pane>
		<Resizable.Handle class="bg-transparent" />
		<Resizable.Pane defaultSize={65} minSize={25} class="min-h-0 min-w-0">
			<div class="h-full min-h-0 min-w-0 overflow-auto">
				<DockPanel
					sessionId={session.sessionId}
					threadId={thread.threadId}
					sessionView={session.ui}
				/>
			</div>
		</Resizable.Pane>
	</Resizable.PaneGroup>
{:else}
	<ThreadWorkspaceHeader
		reserveSidebarSpace={props.reserveSidebarSpace ?? false}
		title={headerTitle}
		state={session.threads.selected?.state}
		titleContent={sessionHeaderDropdown}
	/>

	<div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
		{#if props.visible}
			<ConversationPane {session} {thread} visible={props.visible} />
		{/if}
	</div>
{/if}
