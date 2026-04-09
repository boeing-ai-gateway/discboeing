<script lang="ts">
	import SessionToolbar from "$lib/components/app/SessionToolbar.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";

	const app = useAppContext();
	const mountedSessionIds = $derived.by(() => app.ui.mountedSessionIds);
	const selectedSessionId = $derived.by(
		() => app.sessions.selectedId ?? app.sessions.pendingId,
	);
</script>

{#each mountedSessionIds as sessionId (sessionId)}
	<div class={sessionId === selectedSessionId ? "contents" : "hidden"}>
		<SessionToolbar {sessionId} />
	</div>
{/each}
