<script lang="ts">
	import { onMount } from "svelte";
	import { useContext } from "$lib/context";

	type Props = {
		sessionId: string;
		threadId: string;
	};

	let { sessionId, threadId }: Props = $props();
	const context = useContext();

	onMount(() => {
		void context.commands.threads
			.activateThread(sessionId, threadId)
			.catch(() => undefined);
	});
</script>
