<script lang="ts">
	import * as Dialog from "$lib/components/ui/dialog";

	type Props = {
		notes: string | null;
	};

	let { notes }: Props = $props();
</script>

<Dialog.Header>
	<Dialog.Title>Notes from the agent</Dialog.Title>
	<Dialog.Description>
		Additional context the agent wanted you to know before approving the commit
		pull.
	</Dialog.Description>
</Dialog.Header>
<div class="rounded-md border bg-muted/20 p-3 text-sm">
	<p class="whitespace-pre-wrap text-sm">{notes ?? ""}</p>
</div>
