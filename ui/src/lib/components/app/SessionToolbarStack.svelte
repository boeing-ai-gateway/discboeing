<script lang="ts">
	import SessionToolbar from "$lib/components/app/SessionToolbar.svelte";
	import { shouldLoadSessionToolbar } from "$lib/context/commands/app-view";
	import { useContext } from "$lib/context/context.svelte";

	const context = useContext();
	const mountedSessionIds = $derived.by(
		() => context.view.app.navigation.mountedSessionIds,
	);
	const selectedSessionId = $derived.by(
		() => context.view.app.selection.sessionId,
	);
</script>

{#each mountedSessionIds as sessionId (sessionId)}
	{#if shouldLoadSessionToolbar(sessionId)}
		<div class={sessionId === selectedSessionId ? "contents" : "hidden"}>
			<SessionToolbar {sessionId} />
		</div>
	{/if}
{/each}
