<script lang="ts">
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import { useContext } from "$lib/context";
	import { resolveSessionDisplayStatus } from "$lib/session-status";

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
		const record = context.data.sessions.byId[sessionId];
		return resolveSessionDisplayStatus(record);
	});
</script>

<SessionStatus
	{status}
	class={className}
	{iconClass}
	{labelClass}
	{showLabel}
/>
