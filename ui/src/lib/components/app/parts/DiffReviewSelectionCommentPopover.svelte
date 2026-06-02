<script lang="ts">
	import { onMount } from "svelte";
	import { Button } from "$lib/components/ui/button";
	import { Label } from "$lib/components/ui/label";
	import { Textarea } from "$lib/components/ui/textarea";

	type Props = {
		path: string;
		selectedText: string;
		draft: string;
		error: string | null;
		position: { top: number; left: number };
		queueing: boolean;
		submitting: boolean;
		onDraftChange: (path: string, value: string) => void;
		onSave: (path: string, action: "queue" | "submit") => Promise<void> | void;
		onClear: (path: string) => void;
	};

	let {
		path,
		selectedText,
		draft,
		error,
		position,
		queueing,
		submitting,
		onDraftChange,
		onSave,
		onClear,
	}: Props = $props();

	let textareaRef = $state<HTMLTextAreaElement | null>(null);

	const titleId = "diff-selection-comment-title";
	const commentId = "diff-selection-comment-input";
	const busy = $derived(queueing || submitting);

	onMount(() => {
		textareaRef?.focus();
	});
</script>

<div
	role="dialog"
	tabindex="-1"
	aria-labelledby={titleId}
	class="fixed z-50 w-[min(calc(100vw-1.5rem),24rem)] rounded-md border border-border bg-popover p-3 text-popover-foreground shadow-lg"
	style={`top: ${position.top}px; left: ${position.left}px;`}
	onkeydown={(event) => {
		if (event.key === "Escape") {
			onClear(path);
		}
	}}
>
	<p
		id={titleId}
		class="text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground"
	>
		Selected diff text
	</p>
	<pre
		class="mt-2 max-h-32 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted/50 p-2 font-mono text-xs text-foreground">{selectedText}</pre>
	<Label for={commentId} class="mt-3 block text-xs font-medium">
		Comment for the assistant
	</Label>
	<Textarea
		id={commentId}
		bind:ref={textareaRef}
		class="mt-1 min-h-24"
		placeholder="Add a comment for the assistant"
		value={draft}
		oninput={(event) => onDraftChange(path, event.currentTarget.value)}
	/>
	{#if error}
		<p class="mt-2 text-xs text-destructive">{error}</p>
	{/if}
	<div class="mt-3 flex flex-wrap items-center gap-2">
		<Button
			size="sm"
			onclick={() => void onSave(path, "queue")}
			disabled={busy}
			variant="outline"
		>
			{queueing ? "Queueing…" : "Queue"}
		</Button>
		<Button
			size="sm"
			onclick={() => void onSave(path, "submit")}
			disabled={busy}
		>
			{submitting ? "Submitting…" : "Submit"}
		</Button>
		<Button
			variant="ghost"
			size="sm"
			onclick={() => onClear(path)}
			disabled={busy}
		>
			Clear selection
		</Button>
	</div>
</div>
