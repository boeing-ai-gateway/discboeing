<script lang="ts">
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useContext } from "$lib/context";
	import { resolveThreadDisplayStatus } from "$lib/session-status";

	type Props = {
		sessionId: string;
		threadId: string;
		class?: string;
	};

	let { sessionId, threadId, class: className }: Props = $props();

	const context = useContext();
	const status = $derived.by(() => {
		const record = context.data.sessions.byId[sessionId];
		return resolveThreadDisplayStatus(record, threadId);
	});
</script>

<SessionStatus {status} showLabel={false} class={className} />
