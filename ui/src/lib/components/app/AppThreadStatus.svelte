<script lang="ts">
	import { resolveThreadDisplayStatus } from "$lib/app/thread-status";
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useContext } from "$lib/context/context.svelte";

	type Props = {
		sessionId: string;
		threadId: string;
		class?: string;
	};

	let { sessionId, threadId, class: className }: Props = $props();

	const context = useContext();
	const status = $derived.by(() => {
		const session = context.data.sessions.byId[sessionId];
		const thread = context.data.threads.bySessionId[sessionId]?.byId[threadId];
		const conversation = context.data.conversations.byThreadId[threadId];

		if (!session) {
			return "unknown";
		}

		return resolveThreadDisplayStatus({
			session,
			sessionThreadStatus:
				session.threadStatus?.threadId === threadId
					? session.threadStatus
					: undefined,
			thread,
			localActivityStatus: conversation?.isStreaming
				? { status: "running" }
				: conversation?.hasPendingQuestion
					? { status: "needs_attention" }
					: null,
		});
	});
</script>

<SessionStatus {status} showLabel={false} class={className} />
