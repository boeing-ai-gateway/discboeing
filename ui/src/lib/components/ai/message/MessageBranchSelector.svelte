<script lang="ts">
	import type { Snippet } from "svelte";
	import type { MessageRole } from "$lib/components/ai/types";
	import { ButtonGroup } from "$lib/components/ui/button-group";
	import { useMessageBranchContext } from "./context";

	type Props = {
		from: MessageRole;
		children?: Snippet;
	};

	let { children, ...restProps }: Props = $props();
	const messageBranch = useMessageBranchContext();
</script>

{#if messageBranch.totalBranches > 1}
	<ButtonGroup
		class="[&>*:not(:first-child)]:rounded-l-md [&>*:not(:last-child)]:rounded-r-md"
		orientation="horizontal"
		{...restProps}
	>
		{@render children?.()}
	</ButtonGroup>
{/if}
