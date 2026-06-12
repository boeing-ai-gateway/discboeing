<script lang="ts">
	import SessionToolbar from "$lib/components/app/SessionToolbar.svelte";
	import { useContext } from "$lib/context";
	import { shouldLoadSessionToolbar } from "$lib/shell-selectors";

	const context = useContext();
	const mountedSessionIds = $derived.by(
		() => context.view.navigation.mountedSessionIds,
	);
	const selectedSessionId = $derived.by(() => context.view.selection.sessionId);
</script>

{#each mountedSessionIds as sessionId (sessionId)}
	{#if shouldLoadSessionToolbar(context, sessionId)}
		<div class={sessionId === selectedSessionId ? "contents" : "hidden"}>
			<SessionToolbar {sessionId} />
		</div>
	{/if}
{/each}
