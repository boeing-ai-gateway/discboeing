<script lang="ts">
	import CheckIcon from "@lucide/svelte/icons/check";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import type { ChatMessage } from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

	type AssistantMessageCopyMode = "markdown" | "json";

	type Props = {
		message: ChatMessage;
	};

	let { message }: Props = $props();
	let copiedKey = $state<string | null>(null);
	const hasMarkdown = $derived(getMessageMarkdown().length > 0);

	function getMessageMarkdown(): string {
		const textParts: string[] = [];
		for (const part of message.parts) {
			if (part.type === "text" && part.text.length > 0) {
				textParts.push(part.text);
			}
		}
		return textParts.join("\n\n");
	}

	function getCopyText(mode: AssistantMessageCopyMode): string {
		if (mode === "json") {
			return JSON.stringify(message, null, "\t");
		}
		return getMessageMarkdown();
	}

	async function writeClipboardText(text: string) {
		if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
			await navigator.clipboard.writeText(text);
			return;
		}

		const textarea = document.createElement("textarea");
		textarea.value = text;
		textarea.setAttribute("readonly", "");
		textarea.style.position = "fixed";
		textarea.style.opacity = "0";
		document.body.append(textarea);
		textarea.select();

		try {
			document.execCommand("copy");
		} finally {
			textarea.remove();
		}
	}

	async function copyMessage(mode: AssistantMessageCopyMode) {
		const text = getCopyText(mode);
		if (!text) {
			return;
		}

		try {
			await writeClipboardText(text);
			copiedKey = mode;
			setTimeout(() => {
				if (copiedKey === mode) {
					copiedKey = null;
				}
			}, 2000);
		} catch (error) {
			console.error("Failed to copy assistant message", error);
		}
	}
</script>

<div
	class="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100"
>
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			<Button
				aria-label="Copy assistant response"
				class="size-7 text-muted-foreground hover:text-foreground"
				size="icon-xs"
				title="Copy assistant response"
				type="button"
				variant="ghost"
			>
				{#if copiedKey}
					<CheckIcon class="size-3.5" />
				{:else}
					<CopyIcon class="size-3.5" />
				{/if}
			</Button>
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start" class="w-52">
			<DropdownMenu.Item
				disabled={!hasMarkdown}
				onclick={() => {
					void copyMessage("markdown");
				}}
			>
				{#if copiedKey === "markdown"}
					<CheckIcon class="size-4" />
				{:else}
					<CopyIcon class="size-4" />
				{/if}
				<span>Copy Markdown</span>
			</DropdownMenu.Item>
			<DropdownMenu.Item
				onclick={() => {
					void copyMessage("json");
				}}
			>
				{#if copiedKey === "json"}
					<CheckIcon class="size-4" />
				{:else}
					<CopyIcon class="size-4" />
				{/if}
				<span>Copy JSON</span>
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</div>
