<script lang="ts">
	import ConversationPane from "$lib/components/app/ConversationPane.svelte";
	import DockPanel from "$lib/components/app/DockPanel.svelte";
	import SessionHeaderDropdown from "$lib/components/app/SessionHeaderDropdown.svelte";
	import ThreadActivation from "$lib/components/app/ThreadActivation.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import * as Resizable from "$lib/components/ui/resizable";
	import { useContext } from "$lib/context";

	type Props = {
		sessionId: string;
		threadId: string;
		visible: boolean;
		reserveSidebarSpace?: boolean;
		onPinSidebar?: () => void;
		mode?: "full" | "conversation-only";
	};

	const props: Props = $props();
	const sessionId = $derived(props.sessionId);
	const threadId = $derived(props.threadId);
	const context = useContext();
	const sessionRecord = $derived(context.data.sessions.byId[sessionId] ?? null);
	const currentSession = $derived(sessionRecord?.value ?? null);
	const sessionView = $derived(context.view.sessions[sessionId] ?? null);
	const selectedThreadId = $derived.by(() => {
		if (context.view.selection.sessionId === sessionId) {
			return context.view.selection.threadId;
		}
		return (
			context.view.selection.requestedThreadIdBySessionId[sessionId] ?? null
		);
	});
	const selectedThread = $derived.by(() =>
		selectedThreadId
			? (sessionRecord?.threads.byId[selectedThreadId]?.value ?? null)
			: null,
	);

	const showDock = $derived(
		(props.mode ?? "full") === "full" &&
			!isChatWorkspaceView(sessionView?.workspace.activeView),
	);
	const dockMaximized = $derived(
		showDock && (sessionView?.workspace.dockMaximized ?? false),
	);
	const headerTitle = $derived.by(() => {
		if (selectedThread?.name) {
			return selectedThread.name;
		}
		if (!currentSession) {
			return "";
		}
		return "";
	});
	const sessionTitle = $derived.by(
		() => currentSession?.displayName || currentSession?.name || "Sessions",
	);

	function isChatWorkspaceView(activeView: string | null | undefined): boolean {
		return (
			!activeView || activeView === "conversation" || activeView === "chat"
		);
	}
</script>

{#if props.visible && currentSession}
	<ThreadActivation {sessionId} {threadId} />
{/if}

{#snippet sessionHeaderDropdown()}
	<SessionHeaderDropdown
		label={sessionTitle}
		onPinSidebar={props.onPinSidebar}
	/>
{/snippet}

{#if showDock && dockMaximized}
	<div class="min-h-0 flex-1 overflow-hidden">
		<DockPanel {sessionId} {threadId} />
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
					state={selectedThread?.state}
					titleContent={sessionHeaderDropdown}
				/>
				<div class="min-h-0 min-w-0 flex-1 overflow-hidden">
					{#if props.visible}
						<ConversationPane {sessionId} {threadId} visible={props.visible} />
					{/if}
				</div>
			</div>
		</Resizable.Pane>
		<Resizable.Handle class="bg-transparent" />
		<Resizable.Pane defaultSize={65} minSize={25} class="min-h-0 min-w-0">
			<div class="h-full min-h-0 min-w-0 overflow-auto">
				<DockPanel {sessionId} {threadId} />
			</div>
		</Resizable.Pane>
	</Resizable.PaneGroup>
{:else}
	<ThreadWorkspaceHeader
		reserveSidebarSpace={props.reserveSidebarSpace ?? false}
		title={headerTitle}
		state={selectedThread?.state}
		titleContent={sessionHeaderDropdown}
	/>

	<div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
		{#if props.visible}
			<ConversationPane {sessionId} {threadId} visible={props.visible} />
		{/if}
	</div>
{/if}
