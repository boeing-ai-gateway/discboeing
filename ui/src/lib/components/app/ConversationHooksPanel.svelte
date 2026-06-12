<script lang="ts">
	import CheckCircleIcon from "@lucide/svelte/icons/check-circle";
	import ClockIcon from "@lucide/svelte/icons/clock";
	import DownloadIcon from "@lucide/svelte/icons/download";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import PauseCircleIcon from "@lucide/svelte/icons/pause-circle";
	import PlayCircleIcon from "@lucide/svelte/icons/play-circle";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import XCircleIcon from "@lucide/svelte/icons/x-circle";
	import { api } from "$lib/api-client";
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import { getHookDisplayState } from "$lib/conversation-helpers";
	import { useContext } from "$lib/context";
	import type {
		HookOutputResponse,
		HookRunStatus as ApiHookRunStatus,
	} from "$lib/api-types";
	import { downloadFile } from "$lib/shell";

	type HookPanelRunStatus = Pick<
		ApiHookRunStatus,
		| "hookId"
		| "hookName"
		| "type"
		| "engine"
		| "phase"
		| "lastResult"
		| "runCount"
		| "failCount"
	> & {
		command?: string;
		lastRunAt?: string;
		lastExitCode?: number;
		executionPaused: boolean;
	};

	type HooksStatus = {
		hooks: HookPanelRunStatus[];
		pendingHookIds: string[];
		executionPaused: boolean;
	};

	type Props = {
		sessionId: string;
		threadId: string;
		expanded: boolean;
		hooksStatus: HooksStatus;
		outputById: Record<string, HookOutputResponse>;
		onRerunHook: (hookId: string) => void;
		onSetExecutionPaused: (paused: boolean) => void;
		onSetHookExecutionPaused: (hookId: string, paused: boolean) => void;
	};

	let {
		sessionId,
		threadId,
		expanded,
		hooksStatus,
		outputById,
		onRerunHook,
		onSetExecutionPaused,
		onSetHookExecutionPaused,
	}: Props = $props();

	const context = useContext();
	const sessionView = $derived(context.view.sessions[sessionId]);
	const selectedHookId = $derived(
		sessionView?.hooks.dialog.selectedHookId ?? null,
	);
	const sessionRecord = $derived(context.data.sessions.byId[sessionId] ?? null);
	const selectedThread = $derived(
		sessionRecord?.threads.byId[threadId]?.value ?? null,
	);
	let reviewPhaseSaving = $state(false);

	$effect(() => {
		if (sessionView && !sessionView.hooks.dialog.open) {
			sessionView.hooks.dialog.selectedHookId = null;
		}
	});

	function pendingHookSet() {
		return new Set(hooksStatus.pendingHookIds);
	}

	function hookDisplayState(hook: HooksStatus["hooks"][number]) {
		return getHookDisplayState(hook, pendingHookSet());
	}

	function hookPassedCount() {
		return hooksStatus.hooks.filter(
			(hook) => hookDisplayState(hook) === "success",
		).length;
	}

	function hookStatusTone(hook: HooksStatus["hooks"][number]) {
		const displayState = hookDisplayState(hook);
		if (displayState === "running") {
			return "text-blue-500";
		}
		if (displayState === "pending") {
			return "text-muted-foreground";
		}
		if (displayState === "success") {
			return "text-green-500";
		}
		if (displayState === "failure") {
			return "text-red-500";
		}
		return "text-muted-foreground";
	}

	function hookStatusLabel(hook: HooksStatus["hooks"][number]) {
		const displayState = hookDisplayState(hook);
		if (displayState === "running") {
			return "Running";
		}
		if (displayState === "pending") {
			return "Pending";
		}
		if (displayState === "success") {
			return "Passed";
		}
		if (displayState === "failure") {
			return "Failed";
		}
		return "Not run";
	}

	function hookPaused(hook: HooksStatus["hooks"][number]) {
		return hooksStatus.executionPaused || hook.executionPaused;
	}

	function hookExecutionLabel(hook: HooksStatus["hooks"][number]) {
		return hookPaused(hook) ? "paused" : hookStatusLabel(hook);
	}

	function canRerunHook(hook: HooksStatus["hooks"][number]) {
		return (
			!hooksStatus.executionPaused &&
			!hook.executionPaused &&
			hookDisplayState(hook) !== "running"
		);
	}

	const reviewHooks = $derived.by(() =>
		hooksStatus.hooks.filter((hook) => hook.phase === "review"),
	);
	const draftHooks = $derived.by(() =>
		hooksStatus.hooks.filter((hook) => hook.phase !== "review"),
	);
	const selectedThreadPhase = $derived(selectedThread?.phase ?? "");

	async function toggleReviewPhase() {
		if (!threadId || reviewPhaseSaving) {
			return;
		}

		reviewPhaseSaving = true;
		try {
			await context.commands.threads.updateThread(
				sessionId,
				threadId,
				{ phase: selectedThreadPhase === "review" ? "" : "review" },
				{ wait: true },
			);
		} finally {
			reviewPhaseSaving = false;
		}
	}

	function openHookDialog(hookId: string) {
		if (!sessionView) {
			return;
		}
		sessionView.hooks.dialog.selectedHookId = hookId;
		sessionView.hooks.dialog.open = true;
	}

	function closeHookDialog() {
		if (!sessionView) {
			return;
		}
		sessionView.hooks.dialog.open = false;
		sessionView.hooks.dialog.selectedHookId = null;
	}

	function setExecutionPaused(paused: boolean) {
		onSetExecutionPaused(paused);
		if (paused) {
			closeHookDialog();
		}
	}

	const selectedHookData = $derived.by(() => {
		if (!selectedHookId) {
			return null;
		}
		return (
			hooksStatus.hooks.find((hook) => hook.hookId === selectedHookId) ?? null
		);
	});

	const selectedHookOutputData = $derived.by(() => {
		if (!selectedHookId) {
			return null;
		}
		return outputById[selectedHookId] ?? null;
	});

	function formatBytes(value: number) {
		if (!Number.isFinite(value) || value <= 0) {
			return "0 B";
		}
		const units = ["B", "KB", "MB", "GB"];
		let size = value;
		let unitIndex = 0;
		while (size >= 1024 && unitIndex < units.length - 1) {
			size /= 1024;
			unitIndex += 1;
		}
		return `${size.toFixed(size >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
	}

	async function downloadSelectedHookOutput() {
		if (!selectedHookId) {
			return;
		}

		const content = await api.downloadHookOutput(sessionId, selectedHookId);
		await downloadFile({
			filename: `${selectedHookId}.log`,
			content,
			mimeType: "text/plain;charset=utf-8",
		});
	}

	function formatRelativeTime(isoString?: string) {
		if (!isoString) {
			return "never";
		}
		const date = new Date(isoString);
		const diffMs = Date.now() - date.getTime();
		const diffSec = Math.floor(diffMs / 1000);
		if (diffSec < 5) {
			return "just now";
		}
		if (diffSec < 60) {
			return `${diffSec}s ago`;
		}
		const diffMin = Math.floor(diffSec / 60);
		if (diffMin < 60) {
			return `${diffMin}m ago`;
		}
		const diffHour = Math.floor(diffMin / 60);
		if (diffHour < 24) {
			return `${diffHour}h ago`;
		}
		return date.toLocaleDateString();
	}
</script>

{#snippet hookRow(hook: HooksStatus["hooks"][number])}
	{@const displayState = hookDisplayState(hook)}
	<div
		class={`flex items-center gap-2 rounded-md px-2 py-1.5 text-sm ${displayState === "running" ? "bg-blue-500/10" : ""}`}
	>
		{#if displayState === "running"}
			<Loader2Icon class={`size-3 animate-spin ${hookStatusTone(hook)}`} />
		{:else if displayState === "pending"}
			<ClockIcon class={`size-3 ${hookStatusTone(hook)}`} />
		{:else if displayState === "failure"}
			<XCircleIcon class={`size-3 ${hookStatusTone(hook)}`} />
		{:else if displayState === "success"}
			<CheckCircleIcon class={`size-3 ${hookStatusTone(hook)}`} />
		{:else}
			<ClockIcon class={`size-3 ${hookStatusTone(hook)}`} />
		{/if}
		<div class="min-w-0 flex-1">
			<div class="truncate text-foreground">{hook.hookName}</div>
			<div class="truncate text-[11px] text-muted-foreground">
				{hook.type}{hook.phase ? ` · ${hook.phase}` : ""} · {hookExecutionLabel(
					hook,
				)} · runs {hook.runCount}
			</div>
		</div>
		<Button
			variant="ghost"
			size="xs"
			onclick={() => openHookDialog(hook.hookId)}
		>
			Details
		</Button>
		<Button
			variant="ghost"
			size="icon-xs"
			onclick={() => {
				onSetHookExecutionPaused(hook.hookId, !hookPaused(hook));
			}}
			title={hookPaused(hook) ? "Resume this hook" : "Pause this hook"}
			aria-label={hookPaused(hook) ? "Resume this hook" : "Pause this hook"}
		>
			{#if hookPaused(hook)}
				<PlayCircleIcon class="size-3 text-amber-500" />
			{:else}
				<PauseCircleIcon class="size-3" />
			{/if}
		</Button>
		{#if canRerunHook(hook)}
			<Button
				variant="ghost"
				size="icon-xs"
				onclick={() => {
					onRerunHook(hook.hookId);
				}}
				title="Rerun hook"
				aria-label="Rerun hook"
			>
				<RotateCcwIcon class="size-3" />
			</Button>
		{/if}
	</div>
{/snippet}

{#if expanded && hooksStatus.hooks.length > 0}
	<div
		class="group -mb-px rounded-t-md rounded-b-none border border-b-0 border-border bg-background shadow-sm"
	>
		<div
			class="flex items-center gap-2 border-b border-border px-3 py-2 transition-colors group-hover:bg-muted/50"
		>
			<div class="min-w-0 flex-1 text-xs font-medium text-muted-foreground">
				Hooks ({hookPassedCount()} passed)
				{#if hooksStatus.executionPaused}
					<span class="text-amber-500"> · paused</span>
				{/if}
			</div>
			<Button
				variant="ghost"
				size="xs"
				class="h-7 gap-1.5 px-2"
				onclick={() => setExecutionPaused(!hooksStatus.executionPaused)}
				title={hooksStatus.executionPaused
					? "Resume hook execution"
					: "Pause all hook execution"}
			>
				{#if hooksStatus.executionPaused}
					<PlayCircleIcon class="size-3.5" />
					Resume
				{:else}
					<PauseCircleIcon class="size-3.5" />
					Pause all
				{/if}
			</Button>
		</div>
		<div class="max-h-96 overflow-x-hidden overflow-y-auto px-1 pt-1 pb-3">
			{#if reviewHooks.length > 0}
				<div class="flex items-center gap-2 px-2 py-1">
					<div
						class="min-w-0 flex-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground"
					>
						Review hooks
					</div>
					<Button
						variant={selectedThreadPhase === "review" ? "outline" : "default"}
						size={selectedThreadPhase === "review" ? "xs" : "sm"}
						class={selectedThreadPhase === "review"
							? "h-6 gap-1.5 px-2"
							: "h-7 gap-1.5 px-2.5 shadow-sm"}
						onclick={toggleReviewPhase}
						disabled={reviewPhaseSaving || !threadId}
						aria-label={selectedThreadPhase === "review"
							? "Set this thread to draft"
							: "Mark this thread ready for review"}
						title={selectedThreadPhase === "review"
							? "Set this thread to draft"
							: "Mark this thread ready for review"}
					>
						{#if reviewPhaseSaving}
							<Loader2Icon class="size-3.5 animate-spin" />
						{/if}
						{selectedThreadPhase === "review"
							? "Set to Draft"
							: "Ready for Review"}
					</Button>
				</div>
				{#each reviewHooks as hook (hook.hookId)}
					{@render hookRow(hook)}
				{/each}
				{#if draftHooks.length > 0}
					<div
						class="px-2 pb-1 pt-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground"
					>
						Change hooks
					</div>
				{/if}
			{/if}
			{#each draftHooks as hook (hook.hookId)}
				{@render hookRow(hook)}
			{/each}
		</div>
	</div>
{/if}

{#if sessionView}
	<Dialog.Root bind:open={sessionView.hooks.dialog.open}>
		{#if selectedHookData}
			{@const hook = selectedHookData}
			{@const displayState = hookDisplayState(hook)}
			<Dialog.Content
				class="sm:max-w-4xl max-h-[85vh] flex flex-col overflow-hidden"
			>
				<Dialog.Header>
					<Dialog.Title class="flex items-center gap-2">
						{#if displayState === "running"}
							<Loader2Icon class="size-4 animate-spin text-blue-500" />
						{:else if displayState === "pending"}
							<ClockIcon class="size-4 text-muted-foreground" />
						{:else if displayState === "failure"}
							<XCircleIcon class="size-4 text-red-500" />
						{:else if displayState === "success"}
							<CheckCircleIcon class="size-4 text-green-500" />
						{:else}
							<ClockIcon class="size-4 text-muted-foreground" />
						{/if}
						{hook.hookName}
					</Dialog.Title>
					<Dialog.Description>
						{hook.type}{hook.phase ? ` · ${hook.phase}` : ""} hook · last run {formatRelativeTime(
							hook.lastRunAt,
						)}
					</Dialog.Description>
				</Dialog.Header>

				<div class="flex items-center gap-4 text-sm">
					<span class="text-muted-foreground"
						>Status: {hookStatusLabel(hook)}</span
					>
					{#if hookPaused(hook)}
						<span class="text-amber-500/80">Paused</span>
					{/if}
					<span class="text-muted-foreground">Runs: {hook.runCount}</span>
					{#if typeof hook.lastExitCode === "number"}
						<span class="text-muted-foreground"
							>Exit code: {hook.lastExitCode}</span
						>
					{/if}
					{#if hook.failCount > 0}
						<span class="text-red-500/80">Failures: {hook.failCount}</span>
					{/if}
					<Button
						variant="outline"
						size="xs"
						class="ms-auto"
						onclick={() =>
							onSetHookExecutionPaused(hook.hookId, !hookPaused(hook))}
					>
						{#if hookPaused(hook)}
							<PlayCircleIcon class="size-3 text-amber-500" />
							Resume hook
						{:else}
							<PauseCircleIcon class="size-3" />
							Pause hook
						{/if}
					</Button>
					{#if canRerunHook(hook)}
						<Button
							variant="outline"
							size="xs"
							onclick={() => onRerunHook(hook.hookId)}
						>
							<RotateCcwIcon class="size-3" />
							Rerun
						</Button>
					{/if}
				</div>

				<div
					class="mt-2 flex-1 min-h-0 overflow-hidden rounded-md border border-border bg-muted/30"
				>
					<div
						class="border-b border-border px-3 py-2 text-xs font-medium text-muted-foreground"
					>
						{hook.command ? `Command: ${hook.command}` : "Output"}
					</div>
					<div class="max-h-[50vh] overflow-auto">
						{#if selectedHookOutputData?.tooLarge}
							<div
								class="flex items-center gap-3 border-b border-border px-3 py-2 text-sm"
							>
								<p class="min-w-0 flex-1 text-muted-foreground">
									Showing the last {formatBytes(
										selectedHookOutputData.displayedBytes,
									)} of {formatBytes(selectedHookOutputData.sizeBytes)}.
								</p>
								<Button
									variant="outline"
									size="sm"
									onclick={downloadSelectedHookOutput}
								>
									<DownloadIcon class="size-4" />
									Download full log
								</Button>
							</div>
						{/if}
						<pre
							class="p-3 text-xs leading-relaxed text-foreground whitespace-pre-wrap break-words">{selectedHookOutputData?.output ??
								"No output available"}</pre>
					</div>
				</div>
			</Dialog.Content>
		{/if}
	</Dialog.Root>
{/if}
