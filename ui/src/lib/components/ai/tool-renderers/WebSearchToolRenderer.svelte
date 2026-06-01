<script lang="ts">
	import GlobeIcon from "@lucide/svelte/icons/globe";
	import {
		LinkSafetyModal,
		LinkSafetyState,
	} from "$lib/components/ai/link-safety-modal";
	import { MessageResponse } from "$lib/components/ai/message";
	import {
		ToolContent,
		ToolHeaderControls,
		ToolHeaderStatus,
	} from "$lib/components/ai/tool";
	import {
		type WebSearchToolOutput,
		validateWebSearchInput,
		validateWebSearchOutput,
	} from "$lib/components/ai/tool-schemas/websearch-schema";
	import { CollapsibleTrigger } from "$lib/components/ui/collapsible";
	import type { ToolRendererComponentProps } from "./types";
	import { getToolInputString, renderToolValue } from "./utils";

	let { toolPart, isRaw, onToggleRaw }: ToolRendererComponentProps = $props();

	const isStreaming = $derived.by(
		() =>
			toolPart.state === "input-streaming" ||
			toolPart.state === "input-available",
	);
	const headerText = $derived.by(() => {
		return (
			getToolInputString(toolPart.input, "query") ||
			getToolInputString(toolPart.input, "url")
		);
	});
	const inputValidation = $derived.by(() =>
		validateWebSearchInput(toolPart.input),
	);
	const validInput = $derived.by(() =>
		inputValidation.success ? inputValidation.data : undefined,
	);
	const outputValidation = $derived.by(() =>
		toolPart.output ? validateWebSearchOutput(toolPart.output) : null,
	);
	const validOutput = $derived.by(() =>
		outputValidation?.success
			? (outputValidation.data as WebSearchToolOutput)
			: undefined,
	);
	const searchError = $derived.by(() => toolPart.errorText);
	const rawOutputText = $derived.by(() => renderToolValue(toolPart.output));
	const inputActionType = $derived.by(() => validInput?.type);
	const inputURL = $derived.by(() => validInput?.url);
	const outputActionType = $derived.by(() => validOutput?.action?.type);
	const outputURL = $derived.by(() => validOutput?.action?.url);

	const linkSafety = new LinkSafetyState();

	function openResultURL(url: string) {
		linkSafety.requestOpen(url);
	}
</script>

<div class="flex items-center justify-between gap-4 px-4 pt-4">
	<CollapsibleTrigger
		class="flex min-w-0 flex-1 items-center gap-2 text-left text-muted-foreground"
	>
		<GlobeIcon class="size-4 shrink-0 text-muted-foreground" />
		<span class="truncate font-medium text-sm">
			{headerText || (isStreaming ? "Loading web search..." : "Web search")}
		</span>
		<ToolHeaderStatus state={toolPart.state} />
	</CollapsibleTrigger>
	<ToolHeaderControls {isRaw} {onToggleRaw} />
</div>

<ToolContent>
	{#if !toolPart.input || typeof toolPart.input !== "object"}
		<div class="p-4 pt-3 text-muted-foreground text-sm">
			{isStreaming
				? "Loading web search..."
				: "Search details are unavailable."}
		</div>
	{:else if !inputValidation.success || (!validInput?.query && !validInput?.url)}
		<div class="space-y-3 p-4 pt-3">
			<p class="text-muted-foreground text-sm">
				{isStreaming
					? "Loading web search..."
					: "Could not parse search details."}
			</p>
			{#if rawOutputText}
				<div class="rounded-md border border-dashed bg-muted/20 p-3">
					<pre
						class="overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs"><code
							>{rawOutputText}</code
						></pre>
				</div>
			{/if}
		</div>
	{:else}
		<div class="space-y-4 p-4 pt-3">
			<div class="rounded-md border bg-muted/30 p-3 text-xs">
				{#if validInput.query}
					<p>
						<span class="text-muted-foreground">query:</span>
						{validInput.query}
					</p>
				{/if}
				{#if inputURL}
					<p>
						<span class="text-muted-foreground">url:</span>
						<button
							type="button"
							onclick={() => openResultURL(inputURL)}
							class="break-all text-left font-mono text-primary underline"
						>
							{inputURL}
						</button>
					</p>
				{/if}
				{#if inputActionType}
					<p class="mt-1">
						<span class="text-muted-foreground">action:</span>
						{inputActionType}
					</p>
				{/if}
				{#if validInput.allowed_domains?.length}
					<p class="mt-1">
						<span class="text-muted-foreground">allowed:</span>
						{validInput.allowed_domains.join(", ")}
					</p>
				{/if}
				{#if validInput.blocked_domains?.length}
					<p class="mt-1">
						<span class="text-muted-foreground">blocked:</span>
						{validInput.blocked_domains.join(", ")}
					</p>
				{/if}
			</div>

			{#if validOutput?.results?.length}
				<div class="rounded-md border bg-muted/20 p-2">
					{#each validOutput.results as result, __key0 (__key0)}
						<button
							type="button"
							onclick={() => openResultURL(result.url)}
							class="block w-full rounded-sm border-b px-2 py-2 text-left last:border-b-0 hover:bg-muted/40"
						>
							<div class="font-medium text-sm">{result.title}</div>
							<div class="break-all font-mono text-muted-foreground text-xs">
								{result.url}
							</div>
							{#if result.snippet}
								<div class="mt-1 text-muted-foreground text-xs">
									{result.snippet}
								</div>
							{/if}
						</button>
					{/each}
				</div>
			{:else if validOutput?.content}
				<div class="rounded-md border bg-muted/20 p-3 text-sm">
					<MessageResponse text={validOutput.content} />
				</div>
			{:else if outputURL || outputActionType || validOutput?.status}
				<div class="rounded-md border bg-muted/20 p-3 text-xs">
					{#if outputURL}
						<p>
							<span class="text-muted-foreground">url:</span>
							<button
								type="button"
								onclick={() => openResultURL(outputURL)}
								class="break-all text-left font-mono text-primary underline"
							>
								{outputURL}
							</button>
						</p>
					{/if}
					{#if outputActionType}
						<p class="mt-1">
							<span class="text-muted-foreground">action:</span>
							{outputActionType}
						</p>
					{/if}
					{#if validOutput?.status}
						<p class="mt-1">
							<span class="text-muted-foreground">status:</span>
							{validOutput.status}
						</p>
					{/if}
				</div>
			{:else if outputValidation?.success && !searchError}
				<div
					class="rounded-md border border-dashed px-3 py-2 text-muted-foreground text-sm"
				>
					Search returned no results.
				</div>
			{/if}

			{#if searchError}
				<div
					class="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-destructive text-sm"
				>
					{searchError}
				</div>
			{/if}

			{#if outputValidation && !outputValidation.success && rawOutputText}
				<div class="rounded-md border border-dashed bg-muted/20 p-3">
					<h5
						class="mb-2 font-medium text-muted-foreground text-xs uppercase tracking-wide"
					>
						Unparsed output
					</h5>
					<pre
						class="overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs"><code
							>{rawOutputText}</code
						></pre>
				</div>
			{/if}
		</div>
	{/if}
</ToolContent>

<LinkSafetyModal
	isOpen={linkSafety.isOpen}
	onClose={() => linkSafety.close()}
	url={linkSafety.url}
/>
