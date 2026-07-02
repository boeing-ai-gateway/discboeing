<script lang="ts">
	import { dev } from "$app/environment";
	import { page } from "$app/state";
	import AppShell from "$lib/components/app/AppShell.svelte";
	import ConversationRendererComparison from "$lib/components/app/ConversationRendererComparison.svelte";
	import MarkdownRendererComparison from "$lib/components/app/MarkdownRendererComparison.svelte";

	const compareConversation = $derived(
		dev ? page.url.searchParams.has("conversation-compare") : false,
	);
	const compareSessionId = $derived(
		dev ? page.url.searchParams.get("markdown-compare") : null,
	);
	const compareThreadId = $derived(
		dev ? page.url.searchParams.get("thread") : null,
	);
</script>

<svelte:head>
	<title>Discboeing UI</title>
</svelte:head>

{#if dev && compareConversation}
	<ConversationRendererComparison />
{:else if dev && compareSessionId}
	<MarkdownRendererComparison
		sessionId={compareSessionId}
		threadId={compareThreadId}
	/>
{:else}
	<AppShell />
{/if}
