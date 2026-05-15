<script lang="ts">
	import SessionToolbar from "$lib/components/app/SessionToolbar.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";

	const app = useAppContext();
	const mountedSessionIds = $derived.by(() => app.ui.mountedSessionIds);
	const selectedSessionId = $derived.by(() => app.sessions.selectedId);
</script>

{#each mountedSessionIds as sessionId (sessionId)}
	{#if sessionId === selectedSessionId}
		<SessionToolbar {sessionId} />
	{/if}
{/each}
