<script lang="ts">
	import { onMount, untrack } from "svelte";
	import ThreadWorkspace from "$lib/components/app/ThreadWorkspace.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";

	type Props = {
		sessionId: string;
		visible: boolean;
		mainClass: string;
		showSidebarToggle?: boolean;
		reserveSidebarSpace?: boolean;
		onToggleSidebar?: () => void;
	};

	let {
		sessionId,
		visible,
		mainClass,
		showSidebarToggle,
		reserveSidebarSpace,
		onToggleSidebar,
	}: Props = $props();
	const session = useSessionContext(untrack(() => sessionId));
	const threadId = $derived.by(
		() => session.threads.selectedId ?? session.sessionId,
	);

	onMount(() => {
		void session.load();
	});
</script>

<div class={visible ? "contents" : "hidden"}>
	{#key threadId}
		<ThreadWorkspace
			{threadId}
			{visible}
			{mainClass}
			{showSidebarToggle}
			{reserveSidebarSpace}
			{onToggleSidebar}
			mode={session.isPending ? "conversation-only" : undefined}
		/>
	{/key}
</div>
