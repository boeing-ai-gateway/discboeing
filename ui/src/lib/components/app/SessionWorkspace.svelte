<script lang="ts">
	import { untrack } from "svelte";
	import SessionActivation from "$lib/components/app/SessionActivation.svelte";
	import ThreadWorkspace from "$lib/components/app/ThreadWorkspace.svelte";
	import { useContext } from "$lib/context";

	type Props = {
		sessionId: string;
		visible: boolean;
		mainClass: string;
		onPinSidebar?: () => void;
	};

	let { sessionId, visible, mainClass, onPinSidebar }: Props = $props();
	const context = useContext();
	const mountedSessionId = untrack(() => sessionId);
	const sessionRecord = $derived(
		context.data.sessions.byId[mountedSessionId] ?? null,
	);
	const currentSession = $derived(sessionRecord?.value ?? null);
	const projectReady = $derived(context.data.project.status.state === "ready");
	const isPendingWorkspace = $derived.by(
		() =>
			mountedSessionId === context.view.selection.pendingSessionId &&
			context.view.selection.sessionId !== mountedSessionId,
	);
	const requestedThreadId = $derived.by(() => {
		if (context.view.selection.sessionId === mountedSessionId) {
			return context.view.selection.threadId;
		}
		return (
			context.view.selection.requestedThreadIdBySessionId[mountedSessionId] ??
			null
		);
	});
	const threadId = $derived.by(
		() =>
			requestedThreadId ?? sessionRecord?.threads.allIds[0] ?? mountedSessionId,
	);

	void context.commands.view.mountSessionView(mountedSessionId);
</script>

{#if !isPendingWorkspace && projectReady}
	<SessionActivation sessionId={mountedSessionId} />
{/if}

<div class={visible ? "contents" : "hidden"}>
	{#key threadId}
		<ThreadWorkspace
			sessionId={mountedSessionId}
			{threadId}
			{visible}
			{mainClass}
			{onPinSidebar}
			mode={!currentSession ? "conversation-only" : undefined}
		/>
	{/key}
</div>
