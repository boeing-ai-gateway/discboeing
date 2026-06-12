<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import { useContext } from "$lib/context";

	type Props = {
		sessionId: string;
	};

	let { sessionId }: Props = $props();
	const context = useContext();
	let activationStarted = false;

	onMount(() => {
		activationStarted = true;
		void context.commands.sessions
			.activateSession(sessionId)
			.catch(() => undefined);
	});

	onDestroy(() => {
		if (!activationStarted) {
			return;
		}
		void context.commands.sessions
			.deactivateSession(sessionId)
			.catch(() => undefined);
	});
</script>
