<script lang="ts">
	import {
		resolveThreadContextDisplayStatus,
		resolveThreadDisplayStatus,
	} from "$lib/app/thread-status";
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";

	type Props = {
		sessionId: string;
		threadId: string;
		class?: string;
	};

	let { sessionId, threadId, class: className }: Props = $props();

	const app = useAppContext();
	const status = $derived.by(() => {
		const session = app.sessions.peek(sessionId);
		const sessionContext = app.sessions.sessionContexts.get(sessionId);
		const thread = sessionContext?.threads.list.find(
			(threadObj) => threadObj.id === threadId,
		);
		const threadContext = sessionContext?.threadContexts.get(threadId);

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
			localActivityStatus: resolveThreadContextDisplayStatus(threadContext),
		});
	});
</script>

<SessionStatus {status} showLabel={false} class={className} />
