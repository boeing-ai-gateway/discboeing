<script lang="ts">
	import { untrack } from "svelte";
	import ThreadWorkspaceActive from "$lib/components/app/ThreadWorkspaceActive.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import { setThreadContext } from "$lib/context/thread-context.svelte";

	type Props = {
		threadId: string;
		visible: boolean;
		mainClass: string;
		reserveSidebarSpace?: boolean;
		mode?: "full" | "conversation-only";
		sidebarOpen?: boolean;
	};

	let { threadId, visible, mainClass, reserveSidebarSpace, mode }: Props =
		$props();
	const session = useSessionContext();
	const thread = session.ensureThread(untrack(() => threadId));
	setThreadContext(thread);
</script>

<main class={mainClass}>
	<ThreadWorkspaceActive {visible} {reserveSidebarSpace} {mode} />
</main>
