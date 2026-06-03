<script lang="ts">
	import { resolveSessionDisplayStatus } from "$lib/app/thread-status";
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useContext } from "$lib/context/context.svelte";

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

	const context = useContext();
	const status = $derived.by(() => {
		const session = context.data.sessions.byId[sessionId];
		if (!session) {
			return "unknown";
		}

		return resolveSessionDisplayStatus(session);
	});
</script>

<SessionStatus
	{status}
	class={className}
	{iconClass}
	{labelClass}
	{showLabel}
/>
