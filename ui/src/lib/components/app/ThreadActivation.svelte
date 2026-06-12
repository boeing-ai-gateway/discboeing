<script lang="ts">
	import { onDestroy, onMount } from "svelte";
	import { useContext } from "$lib/context";

	type Props = {
		sessionId: string;
		threadId: string;
	};

	let { sessionId, threadId }: Props = $props();
	const context = useContext();
	let activationStarted = false;

	onMount(() => {
		activationStarted = true;
		void context.commands.threads
			.activateThread(sessionId, threadId)
			.catch(() => undefined);
	});

	onDestroy(() => {
		if (!activationStarted) {
			return;
		}
		void context.commands.threads
			.deactivateThread(sessionId, threadId)
			.catch(() => undefined);
	});
</script>
