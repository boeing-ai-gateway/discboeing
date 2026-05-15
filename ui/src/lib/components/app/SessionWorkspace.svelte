<script lang="ts">
	import { onDestroy, untrack } from "svelte";
	import ThreadWorkspace from "$lib/components/app/ThreadWorkspace.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { setSessionContext } from "$lib/context/session-context.svelte";

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
	const threadId = $derived.by(
		() => session.threads.selectedId ?? session.sessionId,
	);

	onDestroy(() => {
		if (app.sessions.sessionContexts.get(session.sessionId) === session) {
			app.sessions.sessionContexts.delete(session.sessionId);
			session.dispose();
		}
	});
</script>

<div class={visible ? "contents" : "hidden"}>
	{#key threadId}
		<ThreadWorkspace
			{threadId}
			{visible}
			{mainClass}
			{reserveSidebarSpace}
			mode={session.isPending ? "conversation-only" : undefined}
		/>
	{/key}
</div>
