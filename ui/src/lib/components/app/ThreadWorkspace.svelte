<script lang="ts">
	import { onDestroy, untrack } from "svelte";
	import ConversationPane from "$lib/components/app/ConversationPane.svelte";
	import ThreadWorkspaceHeader from "$lib/components/app/parts/ThreadWorkspaceHeader.svelte";
	import { isSessionTransitioningStatus } from "$lib/session/session-status";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import { setThreadContext } from "$lib/context/thread-context.svelte";

	type Props = {
		threadId: string;
		visible: boolean;
		reserveSidebarSpace?: boolean;
		mode?: "conversation" | "connection-only";
	};

	let {
		threadId,
		visible,
		reserveSidebarSpace = false,
		mode = "conversation",
	}: Props = $props();
	const session = useSessionContext();
	const thread = session.ensureThread(untrack(() => threadId));
	setThreadContext(thread);

	$effect(() => {
		if (!visible || !session.current) {
			return;
		}
		void thread.start();
	});

	onDestroy(() => {
		if (session.threadContexts.get(thread.threadId) === thread) {
			session.threadContexts.delete(thread.threadId);
		}
		thread.dispose();
	});

	const headerTitle = $derived.by(() => {
		if (session.threads.selected?.name) {
			return session.threads.selected.name;
		}
		if (session.isPending) {
			return "";
		}
		return isSessionTransitioningStatus(session.current?.sandboxStatus)
			? "Loading thread"
			: "";
	});
</script>

{#if mode === "conversation"}
	<ThreadWorkspaceHeader
		{reserveSidebarSpace}
		title={headerTitle}
		state={session.threads.selected?.state}
	/>

	<div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
		{#if visible}
			<ConversationPane {visible} />
		{/if}
	</div>
{/if}
