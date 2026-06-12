<script lang="ts">
	import MessageSquarePlusIcon from "@lucide/svelte/icons/message-square-plus";
	import { tick } from "svelte";
	import { Button } from "$lib/components/ui/button";
	import type { ConversationComment } from "$lib/context/context.types";

	type PendingSelectionComment = {
		snippet: string;
		left: number;
		top: number;
	};

	type Props = {
		conversationRoot: HTMLElement | null;
		scrollContainer: HTMLElement | null;
		onQueueComment: (
			comment: Omit<ConversationComment, "id">,
		) => Promise<void> | void;
		onSubmitComment: (
			comment: Omit<ConversationComment, "id">,
		) => Promise<void> | void;
	};

	let {
		conversationRoot,
		scrollContainer,
		onQueueComment,
		onSubmitComment,
	}: Props = $props();

	let pendingSelectionComment = $state<PendingSelectionComment | null>(null);
	let selectionCommentOpen = $state(false);
	let selectionCommentDraft = $state("");
	let selectionCommentTextarea = $state<HTMLTextAreaElement | null>(null);
	let pendingAction = $state<"queue" | "submit" | null>(null);
	let selectionCommentError = $state<string | null>(null);

	function clampSelectionCommentPosition(left: number, top: number) {
		return {
			left: Math.min(Math.max(left, 12), window.innerWidth - 220),
			top: Math.min(Math.max(top, 12), window.innerHeight - 80),
		};
	}

	function selectedTextBelongsToConversation(range: Range) {
		if (!conversationRoot) {
			return false;
		}
		const container = range.commonAncestorContainer;
		const target =
			container.nodeType === Node.ELEMENT_NODE
				? container
				: container.parentElement;
		return target ? conversationRoot.contains(target) : false;
	}

	function updatePendingSelectionComment() {
		const selection = window.getSelection();
		if (!selection || selection.rangeCount === 0 || selection.isCollapsed) {
			if (!selectionCommentOpen) {
				pendingSelectionComment = null;
			}
			return;
		}

		const range = selection.getRangeAt(0);
		if (!selectedTextBelongsToConversation(range)) {
			if (!selectionCommentOpen) {
				pendingSelectionComment = null;
			}
			return;
		}

		const snippet = selection.toString().replace(/\s+/g, " ").trim();
		if (!snippet) {
			pendingSelectionComment = null;
			return;
		}

		const rect = range.getBoundingClientRect();
		const position = clampSelectionCommentPosition(rect.right + 8, rect.top);
		pendingSelectionComment = {
			snippet,
			left: position.left,
			top: position.top,
		};
	}

	function openSelectionComment() {
		if (!pendingSelectionComment) {
			return;
		}
		selectionCommentDraft = "";
		selectionCommentOpen = true;
		void tick().then(() => selectionCommentTextarea?.focus());
	}

	function closeSelectionComment() {
		selectionCommentOpen = false;
		selectionCommentDraft = "";
		selectionCommentError = null;
		pendingAction = null;
		pendingSelectionComment = null;
	}

	async function saveSelectionComment(action: "queue" | "submit") {
		if (!pendingSelectionComment || !selectionCommentDraft.trim()) {
			return;
		}
		const comment = {
			snippet: pendingSelectionComment.snippet,
			comment: selectionCommentDraft,
		};
		pendingAction = action;
		selectionCommentError = null;
		try {
			if (action === "queue") {
				await onQueueComment(comment);
			} else {
				await onSubmitComment(comment);
			}
			window.getSelection()?.removeAllRanges();
			closeSelectionComment();
		} catch (error) {
			selectionCommentError =
				error instanceof Error ? error.message : "Failed to save comment";
		} finally {
			pendingAction = null;
		}
	}

	$effect(() => {
		const element = scrollContainer;
		if (!element) {
			return;
		}

		const handleSelectionUpdate = () => {
			void tick().then(updatePendingSelectionComment);
		};
		const handleScroll = () => {
			if (!selectionCommentOpen) {
				pendingSelectionComment = null;
			}
		};

		element.addEventListener("mouseup", handleSelectionUpdate);
		element.addEventListener("keyup", handleSelectionUpdate);
		element.addEventListener("scroll", handleScroll);

		return () => {
			element.removeEventListener("mouseup", handleSelectionUpdate);
			element.removeEventListener("keyup", handleSelectionUpdate);
			element.removeEventListener("scroll", handleScroll);
		};
	});
</script>

{#if pendingSelectionComment && !selectionCommentOpen}
	<div
		class="fixed z-50"
		style={`left: ${pendingSelectionComment.left}px; top: ${pendingSelectionComment.top}px;`}
	>
		<Button
			class="gap-1.5 rounded-full border-primary/50 bg-background shadow-lg"
			onclick={openSelectionComment}
			onmousedown={(event) => event.preventDefault()}
			size="sm"
			type="button"
			variant="outline"
		>
			<MessageSquarePlusIcon class="size-4" />
			Comment
		</Button>
	</div>
{/if}

{#if pendingSelectionComment && selectionCommentOpen}
	<div
		class="fixed z-50 w-80 rounded-xl border border-border bg-card p-3 text-card-foreground shadow-xl"
		style={`left: ${Math.min(pendingSelectionComment.left, window.innerWidth - 340)}px; top: ${pendingSelectionComment.top}px;`}
	>
		<div class="mb-2 text-muted-foreground text-xs">Comment on</div>
		<div
			class="mb-3 line-clamp-3 border-primary/60 border-l-2 bg-primary/5 py-1 pl-2 text-sm italic"
		>
			{pendingSelectionComment.snippet}
		</div>
		<textarea
			aria-label="Comment"
			bind:this={selectionCommentTextarea}
			bind:value={selectionCommentDraft}
			class="min-h-24 w-full resize-none rounded-md border border-input bg-background px-3 py-2 text-sm outline-none ring-offset-background placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
			placeholder="Add a comment..."
		></textarea>
		{#if selectionCommentError}
			<p class="mt-2 text-destructive text-xs">{selectionCommentError}</p>
		{/if}
		<div class="mt-3 flex justify-end gap-2">
			<Button
				onclick={closeSelectionComment}
				size="sm"
				type="button"
				variant="ghost"
			>
				Cancel
			</Button>
			<Button
				disabled={!selectionCommentDraft.trim() || pendingAction !== null}
				onclick={() => void saveSelectionComment("queue")}
				size="sm"
				type="button"
				variant="outline"
			>
				{pendingAction === "queue" ? "Queueing…" : "Queue"}
			</Button>
			<Button
				disabled={!selectionCommentDraft.trim() || pendingAction !== null}
				onclick={() => void saveSelectionComment("submit")}
				size="sm"
				type="button"
			>
				{pendingAction === "submit" ? "Submitting…" : "Submit"}
			</Button>
		</div>
	</div>
{/if}
