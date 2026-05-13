<script lang="ts">
	import { resolveSessionDisplayStatus } from "$lib/app/thread-status";
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";

	type Props = {
		sessionId: string;
		class?: string;
		iconClass?: string;
		labelClass?: string;
		showLabel?: boolean;
	};

	let {
		sessionId,
		class: className,
		iconClass,
		labelClass,
		showLabel = true,
	}: Props = $props();

	const app = useAppContext();
	const status = $derived.by(() => {
		const session = app.sessions.peek(sessionId);
		if (!session) {
			return "unknown";
		}

		return resolveSessionDisplayStatus({
			sessionStatus: session.status,
			sessionActivityStatus: session.threadStatus?.status,
			commitStatus: session.commitStatus,
			commitOperation: session.commitOperation,
		});
	});
</script>

<SessionStatus
	{status}
	class={className}
	{iconClass}
	{labelClass}
	{showLabel}
/>
