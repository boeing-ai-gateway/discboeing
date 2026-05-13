<script lang="ts">
	import type { Snippet } from "svelte";
	import { setCodeBlockContext } from "./context";
	import CodeBlockContainer from "./CodeBlockContainer.svelte";
	import CodeBlockContent from "./CodeBlockContent.svelte";

	type Props = {
		code: string;
		language: string;
		showLineNumbers?: boolean;
		class?: string;
		children?: Snippet;
	};

	let {
		code,
		language,
		showLineNumbers = false,
		class: className,
		children,
		...restProps
	}: Props = $props();

	const codeBlock = $state({ code: "" });

	$effect(() => {
		codeBlock.code = code;
	});

	setCodeBlockContext(codeBlock);
</script>

<CodeBlockContainer class={className} {language} {...restProps}>
	{@render children?.()}
	<CodeBlockContent {code} {showLineNumbers} />
</CodeBlockContainer>
