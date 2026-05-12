<script lang="ts">
	import SessionStatus from "$lib/components/app/parts/SessionStatus.svelte";
	import type { ThreadDisplayStatusValue } from "$lib/app/thread-status";

	type Props = {
		status: ThreadDisplayStatusValue;
		class?: string;
	};

	let { status, class: className }: Props = $props();
</script>

<SessionStatus {status} showLabel={false} class={className} />
