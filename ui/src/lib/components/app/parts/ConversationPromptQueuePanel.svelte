<script lang="ts">
	import ClockIcon from "@lucide/svelte/icons/clock";
	import CheckIcon from "@lucide/svelte/icons/check";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ChevronUpIcon from "@lucide/svelte/icons/chevron-up";
	import PencilIcon from "@lucide/svelte/icons/pencil";
	import PauseIcon from "@lucide/svelte/icons/pause";
	import PlayIcon from "@lucide/svelte/icons/play";
	import Trash2Icon from "@lucide/svelte/icons/trash-2";
	import XIcon from "@lucide/svelte/icons/x";
	import { onMount } from "svelte";

	import type { QueuedPrompt, UpdateQueuedPromptRequest } from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import {
		Popover,
		PopoverContent,
		PopoverTrigger,
	} from "$lib/components/ui/popover";
	import { Textarea } from "$lib/components/ui/textarea";
	import ConversationPromptSchedulePicker from "$lib/components/app/parts/ConversationPromptSchedulePicker.svelte";

	type Props = {
		entries: QueuedPrompt[];
		onDelete: (queueId: string) => void | Promise<void>;
		onUpdate: (
			queueId: string,
			payload: UpdateQueuedPromptRequest,
		) => void | Promise<void>;
	};

	let { entries, onDelete, onUpdate }: Props = $props();

	let schedulerOpenById = $state<Record<string, boolean>>({});
	let savingById = $state<Record<string, boolean>>({});
	let editingById = $state<Record<string, boolean>>({});
	let editTextById = $state<Record<string, string>>({});
	let runAfterOverrideById = $state<Record<string, string | null>>({});
	let now = $state(Date.now());

	onMount(() => {
		const interval = window.setInterval(() => {
			now = Date.now();
		}, 1000);

		return () => {
			window.clearInterval(interval);
		};
	});

	const displayEntries = $derived(
		entries.map((entry, index) => ({
			entry,
			renderKey: `${entry.id}:${entry.createdAt ?? ""}:${index}`,
		})),
	);

	function getPromptText(entry: QueuedPrompt): string {
		const parts = entry.message.parts ?? [];
		const text: string[] = [];
		for (const part of parts) {
			if (part.type === "text") {
				const trimmed = part.text.trim();
				if (trimmed.length > 0) {
					text.push(trimmed);
				}
			}
		}
		return text.join(" ") || "Queued prompt";
	}

	function getAttachmentCount(entry: QueuedPrompt): number {
		const parts = entry.message.parts ?? [];
		return parts.filter((part) => part.type === "file").length;
	}

	function getDisplayedRunAfter(entry: QueuedPrompt): string | undefined {
		if (Object.hasOwn(runAfterOverrideById, entry.id)) {
			return runAfterOverrideById[entry.id] ?? undefined;
		}
		return entry.runAfter;
	}

	function parseRunAfter(value?: string): Date | null {
		if (!value) {
			return null;
		}
		const parsed = new Date(value);
		return Number.isNaN(parsed.getTime()) ? null : parsed;
	}

	function isPausedRunAfter(value?: string): boolean {
		const parsed = parseRunAfter(value);
		if (!parsed) {
			return false;
		}
		return parsed.getTime() >= now + 25 * 365 * 24 * 60 * 60 * 1000;
	}

	function formatAbsoluteRunAfter(value: Date): string {
		return value.toLocaleString(undefined, {
			dateStyle: "medium",
			timeStyle: "short",
		});
	}

	function formatRelativeRunAfter(value: Date): string {
		const diffMs = value.getTime() - now;
		if (diffMs <= 0) {
			return "Ready now";
		}

		const diffSeconds = Math.round(diffMs / 1000);
		if (diffSeconds < 60) {
			return `In ${diffSeconds} second${diffSeconds === 1 ? "" : "s"}`;
		}

		const diffMinutes = Math.round(diffMs / (60 * 1000));
		if (diffMinutes < 60) {
			return `In ${diffMinutes} minute${diffMinutes === 1 ? "" : "s"}`;
		}

		const diffHours = Math.round(diffMs / (60 * 60 * 1000));
		if (diffHours < 24) {
			return `In ${diffHours} hour${diffHours === 1 ? "" : "s"}`;
		}

		const diffDays = Math.round(diffMs / (24 * 60 * 60 * 1000));
		return `In ${diffDays} day${diffDays === 1 ? "" : "s"}`;
	}

	function formatRunAfterStatus(value?: string): string {
		if (isPausedRunAfter(value)) {
			return "Paused";
		}
		const parsed = parseRunAfter(value);
		if (!parsed) {
			return "";
		}
		return `${formatRelativeRunAfter(parsed)} · ${formatAbsoluteRunAfter(parsed)}`;
	}

	function setSaving(entryId: string, saving: boolean) {
		savingById = { ...savingById, [entryId]: saving };
	}

	function setEditing(entry: QueuedPrompt, editing: boolean) {
		editingById = { ...editingById, [entry.id]: editing };
		if (editing) {
			editTextById = { ...editTextById, [entry.id]: getPromptText(entry) };
		}
	}

	function setEditText(entryId: string, value: string) {
		editTextById = { ...editTextById, [entryId]: value };
	}

	function getSchedulerOpen(entryId: string): boolean {
		return schedulerOpenById[entryId] ?? false;
	}

	function setSchedulerOpen(entryId: string, open: boolean) {
		schedulerOpenById = { ...schedulerOpenById, [entryId]: open };
	}

	function buildPauseDate(): Date {
		const now = new Date();
		return new Date(
			now.getFullYear() + 100,
			now.getMonth(),
			now.getDate(),
			now.getHours(),
			now.getMinutes(),
			now.getSeconds(),
			now.getMilliseconds(),
		);
	}

	async function saveLater(entryId: string, runAfter: Date | null) {
		setSaving(entryId, true);
		try {
			if (runAfter === null) {
				runAfterOverrideById = { ...runAfterOverrideById, [entryId]: null };
				await onUpdate(entryId, { clearRunAfter: true });
			} else {
				runAfterOverrideById = {
					...runAfterOverrideById,
					[entryId]: runAfter.toISOString(),
				};
				await onUpdate(entryId, { runAfter: runAfter.toISOString() });
			}
			setSchedulerOpen(entryId, false);
		} finally {
			setSaving(entryId, false);
		}
	}

	async function savePromptText(entry: QueuedPrompt) {
		const text = (editTextById[entry.id] ?? "").trim();
		const fileParts = (entry.message.parts ?? []).filter(
			(part) => part.type === "file",
		);
		if (text.length === 0 && fileParts.length === 0) {
			return;
		}

		setSaving(entry.id, true);
		try {
			await onUpdate(entry.id, {
				message: {
					...entry.message,
					parts: [
						...(text.length > 0 ? [{ type: "text" as const, text }] : []),
						...fileParts,
					],
				},
			});
			setEditing(entry, false);
		} finally {
			setSaving(entry.id, false);
		}
	}

	async function movePrompt(entry: QueuedPrompt, position: number) {
		setSaving(entry.id, true);
		try {
			await onUpdate(entry.id, { position });
		} finally {
			setSaving(entry.id, false);
		}
	}
</script>

{#if entries.length > 0}
	<div class="mb-2 rounded-lg border border-border bg-background shadow-sm">
		<div
			class="border-b border-border px-3 py-2 text-xs font-medium text-muted-foreground"
		>
			Queued prompts ({entries.length})
		</div>
		<div class="flex flex-col gap-1 p-1">
			{#each displayEntries as { entry, renderKey }, index (renderKey)}
				<div
					class="flex items-start gap-2 rounded-md px-2 py-2 hover:bg-muted/50"
				>
					<div class="min-w-0 flex-1">
						{#if editingById[entry.id]}
							<div class="space-y-2">
								<Textarea
									value={editTextById[entry.id] ?? ""}
									oninput={(event) =>
										setEditText(entry.id, event.currentTarget.value)}
									class="min-h-20 resize-none text-sm"
									disabled={savingById[entry.id]}
								/>
								<div class="flex justify-end gap-1">
									<Button
										variant="ghost"
										size="sm"
										disabled={savingById[entry.id]}
										onclick={() => setEditing(entry, false)}
									>
										Cancel
									</Button>
									<Button
										variant="default"
										size="sm"
										disabled={savingById[entry.id] ||
											((editTextById[entry.id] ?? "").trim().length === 0 &&
												getAttachmentCount(entry) === 0)}
										onclick={() => {
											void savePromptText(entry);
										}}
									>
										<CheckIcon class="mr-1 size-3.5" />
										Save
									</Button>
								</div>
							</div>
						{:else}
							<div class="truncate text-sm text-foreground">
								{getPromptText(entry)}
							</div>
						{/if}
						<div
							class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground"
						>
							{#if getAttachmentCount(entry) > 0}
								<span
									>{getAttachmentCount(entry)} attachment{getAttachmentCount(
										entry,
									) === 1
										? ""
										: "s"}</span
								>
							{/if}
							{#if entry.model}
								<span>{entry.model}</span>
							{/if}
							{#if getDisplayedRunAfter(entry)}
								<span class="font-medium text-foreground/80"
									>{formatRunAfterStatus(getDisplayedRunAfter(entry))}</span
								>
							{/if}
						</div>
					</div>
					<div class="flex shrink-0 items-start gap-1">
						<Button
							variant="ghost"
							size="icon-sm"
							class="shrink-0"
							title="Move queued prompt up"
							disabled={savingById[entry.id] || index === 0}
							onclick={() => {
								void movePrompt(entry, index - 1);
							}}
						>
							<ChevronUpIcon class="size-3.5" />
						</Button>
						<Button
							variant="ghost"
							size="icon-sm"
							class="shrink-0"
							title="Move queued prompt down"
							disabled={savingById[entry.id] ||
								index === displayEntries.length - 1}
							onclick={() => {
								void movePrompt(entry, index + 1);
							}}
						>
							<ChevronDownIcon class="size-3.5" />
						</Button>
						<Button
							variant="ghost"
							size="icon-sm"
							class="shrink-0"
							title={editingById[entry.id]
								? "Cancel editing queued prompt"
								: "Edit queued prompt"}
							disabled={savingById[entry.id]}
							onclick={() => setEditing(entry, !editingById[entry.id])}
						>
							{#if editingById[entry.id]}
								<XIcon class="size-3.5" />
							{:else}
								<PencilIcon class="size-3.5" />
							{/if}
						</Button>
						{#if getDisplayedRunAfter(entry)}
							<Popover
								bind:open={
									() => getSchedulerOpen(entry.id),
									(open) => setSchedulerOpen(entry.id, open)
								}
							>
								<PopoverTrigger>
									<Button
										variant="ghost"
										size="icon-sm"
										class="shrink-0"
										title="Schedule queued prompt"
										disabled={savingById[entry.id]}
									>
										<ClockIcon class="size-3.5" />
									</Button>
								</PopoverTrigger>
								<PopoverContent align="end" class="w-72 p-3">
									<ConversationPromptSchedulePicker
										currentRunAfter={getDisplayedRunAfter(entry)}
										disabled={savingById[entry.id]}
										onSelect={(runAfter) => saveLater(entry.id, runAfter)}
									/>
								</PopoverContent>
							</Popover>
							<Button
								variant="ghost"
								size="icon-sm"
								class="shrink-0"
								title="Run queued prompt now"
								disabled={savingById[entry.id]}
								onclick={() => {
									void saveLater(entry.id, null);
								}}
							>
								<PlayIcon class="size-3.5" />
							</Button>
						{:else}
							<Button
								variant="ghost"
								size="icon-sm"
								class="shrink-0"
								title="Pause queued prompt"
								disabled={savingById[entry.id]}
								onclick={() => {
									void saveLater(entry.id, buildPauseDate());
								}}
							>
								<PauseIcon class="size-3.5" />
							</Button>
						{/if}
						<Button
							variant="ghost"
							size="icon-sm"
							class="shrink-0"
							title="Delete queued prompt"
							disabled={savingById[entry.id]}
							onclick={() => {
								void onDelete(entry.id);
							}}
						>
							<Trash2Icon class="size-3.5 text-destructive" />
						</Button>
					</div>
				</div>
			{/each}
		</div>
	</div>
{/if}
