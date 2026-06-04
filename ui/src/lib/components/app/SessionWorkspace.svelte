<script lang="ts">
	import { onDestroy, untrack } from "svelte";
	import ThreadWorkspace from "$lib/components/app/ThreadWorkspace.svelte";
	import {
		ensureSessionState,
		releaseSessionState,
	} from "$lib/context/commands/session";

	type Props = {
		sessionId: string;
		visible: boolean;
		mainClass: string;
	};

	let { sessionId, visible, mainClass }: Props = $props();
	const session = ensureSessionState(untrack(() => sessionId));
	const threadId = $derived.by(
		() => session.threads.selectedId ?? session.sessionId,
	);

	onDestroy(() => {
		releaseSessionState(session);
	});
</script>

<div class={visible ? "contents" : "hidden"}>
	{#key threadId}
		<ThreadWorkspace
			{session}
			{threadId}
			{visible}
			{mainClass}
			mode={session.isPending ? "conversation-only" : undefined}
		/>
	{/key}
</div>
