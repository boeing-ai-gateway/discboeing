<script lang="ts">
	import type { Snippet } from "svelte";
	import { InputGroup } from "$lib/components/ui/input-group";
	import { cn } from "$lib/utils";
	import { setSnippetContext } from "./context";

	type Props = {
		code: string;
		class?: string;
		children?: Snippet;
	};

	let { code, class: className, children, ...restProps }: Props = $props();
	const snippet = $state({ code: "" });
	$effect(() => {
		snippet.code = code;
	});
	setSnippetContext(snippet);
</script>

<InputGroup class={cn("font-mono", className)} {...restProps}>
	{@render children?.()}
</InputGroup>
