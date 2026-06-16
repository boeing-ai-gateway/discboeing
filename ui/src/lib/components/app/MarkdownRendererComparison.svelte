<script lang="ts">
	import { onMount } from "svelte";
	import { api } from "$lib/api-client";
	import type { ChatMessage } from "$lib/api-types";
	import { MessageResponse } from "$lib/components/ai/message";
	import { SvelteStreamdown } from "$lib/components/ai/streamdown";
	import "$lib/web-components/markdown/define";
	import { useContext } from "$lib/context";

	type Props = {
		sessionId: string;
		threadId?: string | null;
	};

	type TextPart = {
		id: string;
		messageId: string;
		messageIndex: number;
		partIndex: number;
		role: ChatMessage["role"];
		text: string;
	};

	let { sessionId, threadId: requestedThreadId = null }: Props = $props();

	const context = useContext();
	let selectedThreadId = $state<string | null>(null);
	let error = $state<string | null>(null);
	let loading = $state(true);

	const threadRecord = $derived.by(() =>
		selectedThreadId
			? (context.data.sessions.byId[sessionId]?.threads.byId[
					selectedThreadId
				] ?? null)
			: null,
	);
	const messages = $derived.by(() => threadRecord?.content.messages ?? []);
	const textParts = $derived.by(() => collectTextParts(messages));

	onMount(() => {
		void load();
	});

	async function load() {
		loading = true;
		error = null;
		try {
			let threadId = requestedThreadId;
			if (!threadId) {
				const response = await api.getThreads(sessionId);
				threadId = response.threads[0]?.id ?? null;
			}
			if (!threadId) {
				throw new Error(`No threads found for session ${sessionId}`);
			}
			selectedThreadId = threadId;
			await context.commands.navigation.openThread(sessionId, threadId);
			await context.commands.threads.activateThread(sessionId, threadId, {
				wait: true,
			});
		} catch (caught) {
			error = caught instanceof Error ? caught.message : String(caught);
		} finally {
			loading = false;
		}
	}

	function collectTextParts(sourceMessages: ChatMessage[]): TextPart[] {
		const parts: TextPart[] = [];
		sourceMessages.forEach((message, messageIndex) => {
			message.parts.forEach((part, partIndex) => {
				if (part.type !== "text") {
					return;
				}
				parts.push({
					id: `${message.id}:${partIndex}`,
					messageId: message.id,
					messageIndex,
					partIndex,
					role: message.role,
					text: part.text,
				});
			});
		});
		return parts;
	}
</script>

<svelte:head>
	<title>Markdown renderer comparison</title>
</svelte:head>

<div class="min-h-screen bg-background text-foreground">
	<header
		class="sticky top-0 z-10 border-b border-border bg-background/95 px-4 py-3 backdrop-blur"
	>
		<div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
			<h1 class="text-base font-semibold">Markdown renderer comparison</h1>
			<span class="text-muted-foreground">session {sessionId}</span>
			{#if selectedThreadId}
				<span class="text-muted-foreground">thread {selectedThreadId}</span>
			{/if}
			<span class="text-muted-foreground">{textParts.length} text parts</span>
		</div>
	</header>

	{#if error}
		<div
			class="m-4 rounded-lg border border-destructive/40 bg-destructive/10 p-4 text-sm text-destructive"
		>
			{error}
		</div>
	{:else if loading && textParts.length === 0}
		<div class="p-4 text-sm text-muted-foreground">Loading session thread…</div>
	{:else}
		<div class="grid min-w-[1500px] grid-cols-3 divide-x divide-border">
			<section class="min-w-0 p-4">
				<h2
					class="sticky top-[49px] z-10 -mx-4 border-b border-border bg-background/95 px-4 py-2 text-sm font-semibold backdrop-blur"
				>
					Conversation MessageResponse
				</h2>
				{@render RendererColumn({ kind: "message-response", textParts })}
			</section>
			<section class="min-w-0 p-4">
				<h2
					class="sticky top-[49px] z-10 -mx-4 border-b border-border bg-background/95 px-4 py-2 text-sm font-semibold backdrop-blur"
				>
					Direct SvelteStreamdown
				</h2>
				{@render RendererColumn({ kind: "streamdown", textParts })}
			</section>
			<section class="min-w-0 p-4">
				<h2
					class="sticky top-[49px] z-10 -mx-4 border-b border-border bg-background/95 px-4 py-2 text-sm font-semibold backdrop-blur"
				>
					Web component
				</h2>
				{@render RendererColumn({ kind: "web-component", textParts })}
			</section>
		</div>
	{/if}
</div>

{#snippet RendererColumn({
	kind,
	textParts,
}: {
	kind: "message-response" | "streamdown" | "web-component";
	textParts: TextPart[];
})}
	<div class="space-y-4 pt-4">
		{#each textParts as part (part.id)}
			<article
				class="rounded-lg border border-border bg-card p-4 shadow-sm"
				data-message-id={part.messageId}
			>
				<div
					class="mb-3 flex items-center justify-between gap-2 border-b border-border pb-2 text-xs text-muted-foreground"
				>
					<span
						>{part.role} message {part.messageIndex + 1}, part {part.partIndex +
							1}</span
					>
					<span>{part.text.length} chars</span>
				</div>
				<div class="min-w-0 break-words">
					{#if kind === "message-response"}
						<MessageResponse
							text={part.text}
							mode="static"
							class="size-full [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
						/>
					{:else if kind === "streamdown"}
						<SvelteStreamdown
							text={part.text}
							mode="static"
							class="size-full [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
						/>
					{:else}
						<disco-markdown mode="static">{part.text}</disco-markdown>
					{/if}
				</div>
			</article>
		{/each}
	</div>
{/snippet}
