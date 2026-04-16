<script lang="ts">
	import EyeIcon from "@lucide/svelte/icons/eye";
	import FileCodeIcon from "@lucide/svelte/icons/file-code";
	import FileTextIcon from "@lucide/svelte/icons/file-text";
	import GitCommitHorizontalIcon from "@lucide/svelte/icons/git-commit-horizontal";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import { parseUnifiedDiff, type ParsedDiffHunk } from "$lib/diff-utils";
	import {
		ToolContent,
		ToolHeaderControls,
		ToolHeaderStatus,
	} from "$lib/components/ai/tool";
	import { api } from "$lib/api-client";
	import type {
		CommitPullPreviewFile,
		CommitPullPreviewResponse,
		PendingQuestion,
	} from "$lib/api-types";
	import RequestCommitPullDiffViewer from "./RequestCommitPullDiffViewer.svelte";
	import RequestCommitPullNotesDialogContent from "./RequestCommitPullNotesDialogContent.svelte";
	import RequestCommitPullRawPatchDialogContent from "./RequestCommitPullRawPatchDialogContent.svelte";
	import { Button } from "$lib/components/ui/button";
	import { CollapsibleTrigger } from "$lib/components/ui/collapsible";
	import * as Dialog from "$lib/components/ui/dialog";
	import { Input } from "$lib/components/ui/input";
	import {
		buildDiffFileContents,
		type DiffRendererParams,
		type DiffStyle,
	} from "$lib/pierre-diff";
	import type { RequestCommitPullDiffEntry } from "./request-commit-pull-diff";
	import type { ToolRendererComponentProps } from "./types";

	const APPROVED_KEY = "__request_commit_pull_approved__";
	const REJECTED_KEY = "__request_commit_pull_rejected__";
	const REJECTED_REASON_KEY = "__request_commit_pull_rejection_reason__";
	const APPROVED_TEXT =
		"The user approved pulling the prepared sandbox commit into the host workspace.";
	const REJECTED_PREFIX =
		"The user rejected pulling the prepared sandbox commit into the host workspace.";

	type PendingQuestionResponse = {
		status: "pending" | "answered" | "expired";
		question: PendingQuestion | null;
	};

	type RequestCommitPullMetadata = {
		directory?: string;
		commitHash?: string;
		commitTitle?: string;
		commitBody?: string;
	};

	type PreviewFileEntry = CommitPullPreviewFile & {
		commitHash: string;
		commitSubject: string;
	};

	let {
		toolPart,
		sessionId = null,
		threadId = null,
		onToolApprovalResponse,
		isRaw,
		onToggleRaw,
		resolvedTheme = "light",
	}: ToolRendererComponentProps = $props();

	let pendingQuestion = $state<PendingQuestion | null>(null);
	let approvalStatus = $state<
		"idle" | "loading" | "pending" | "answered" | "error"
	>("idle");
	let approvalError = $state<string | null>(null);
	let preview = $state<CommitPullPreviewResponse | null>(null);
	let previewStatus = $state<"idle" | "loading" | "ready" | "error">("idle");
	let previewError = $state<string | null>(null);
	let retrying = $state(false);
	let diffStyle = $state<DiffStyle>("unified");
	let diffDialogOpen = $state(false);
	let rawPatchDialogOpen = $state(false);
	let notesDialogOpen = $state(false);
	let rejectionReason = $state("");
	let rejectDialogOpen = $state(false);

	const approvalId = $derived.by(() => {
		const approval = toolPart.approval;
		if (approval && typeof approval === "object" && "id" in approval) {
			return typeof approval.id === "string" ? approval.id : null;
		}
		return toolPart.toolCallId || null;
	});
	const summary = $derived.by(() => {
		const notes = pendingQuestion?.questions?.find(
			(question) => question.notes,
		)?.notes;
		const trimmed = notes?.trim();
		return trimmed ? trimmed : null;
	});
	const metadata = $derived.by(
		() => (pendingQuestion?.metadata ?? {}) as RequestCommitPullMetadata,
	);
	const previewCommit = $derived.by(
		() => preview?.commits?.[preview.commits.length - 1] ?? null,
	);
	const commitTitle = $derived.by(
		() =>
			previewCommit?.subject?.trim() ||
			metadata.commitTitle?.trim() ||
			"Untitled commit",
	);
	const commitHash = $derived.by(
		() => preview?.headCommit?.trim() || metadata.commitHash?.trim() || null,
	);
	const shortCommitHash = $derived.by(() =>
		commitHash ? commitHash.slice(0, 12) : null,
	);
	const commitBody = $derived.by(
		() => previewCommit?.body?.trim() || metadata.commitBody?.trim() || null,
	);
	const previewFiles = $derived.by<PreviewFileEntry[]>(
		() =>
			preview?.commits.flatMap((commit) =>
				commit.files.map((file) => ({
					...file,
					commitHash: commit.hash,
					commitSubject: commit.subject,
				})),
			) ?? [],
	);
	const previewDiffEntries = $derived.by<RequestCommitPullDiffEntry[]>(() =>
		previewFiles.map((file) => ({
			...file,
			params: buildPreviewDiffParams(file),
		})),
	);
	const outputText = $derived.by(() =>
		typeof toolPart.output === "string" ? toolPart.output : null,
	);
	const wasApproved = $derived.by(
		() => outputText === APPROVED_TEXT || approvalStatus === "answered",
	);
	const rejectionSummary = $derived.by(() => {
		const localReason = rejectionReason.trim();
		if (localReason) {
			return localReason;
		}
		if (!outputText || !outputText.startsWith(REJECTED_PREFIX)) {
			return null;
		}
		const reasonPrefix = `${REJECTED_PREFIX} Reason: `;
		if (outputText.startsWith(reasonPrefix)) {
			return outputText.slice(reasonPrefix.length).trim();
		}
		return "";
	});

	async function fetchPendingQuestion(
		sessionId: string,
		threadId: string,
		questionId: string,
	): Promise<PendingQuestionResponse> {
		return (await api.getThreadChatQuestion(
			sessionId,
			threadId,
			questionId,
		)) as PendingQuestionResponse;
	}

	async function fetchCommitPullPreview(
		sessionId: string,
		threadId: string,
		questionId: string,
	): Promise<CommitPullPreviewResponse> {
		return api.getThreadCommitPullPreview(sessionId, threadId, questionId);
	}

	$effect(() => {
		if (toolPart.state !== "approval-requested") {
			approvalStatus = "idle";
			approvalError = null;
			pendingQuestion = null;
			preview = null;
			previewStatus = "idle";
			previewError = null;
			diffDialogOpen = false;
			rawPatchDialogOpen = false;
			notesDialogOpen = false;
			rejectDialogOpen = false;
			return;
		}
		if (!sessionId || !threadId || !approvalId) {
			approvalStatus = "loading";
			approvalError = null;
			pendingQuestion = null;
			preview = null;
			previewStatus = "idle";
			previewError = null;
			return;
		}

		approvalStatus = "loading";
		approvalError = null;
		pendingQuestion = null;
		preview = null;
		previewStatus = "idle";
		previewError = null;
		diffDialogOpen = false;
		rawPatchDialogOpen = false;
		notesDialogOpen = false;

		let cancelled = false;
		void fetchPendingQuestion(sessionId, threadId, approvalId)
			.then((result) => {
				if (cancelled) {
					return;
				}
				if (result.status === "pending" && result.question) {
					pendingQuestion = result.question;
					approvalStatus = "pending";
					return;
				}
				approvalStatus = "answered";
			})
			.catch((error) => {
				if (cancelled) {
					return;
				}
				approvalStatus = "error";
				approvalError =
					error instanceof Error
						? error.message
						: "Failed to load commit pull request";
			});

		return () => {
			cancelled = true;
		};
	});

	$effect(() => {
		if (
			toolPart.state !== "approval-requested" ||
			approvalStatus !== "pending" ||
			!pendingQuestion ||
			!sessionId ||
			!threadId ||
			!approvalId
		) {
			if (approvalStatus !== "pending") {
				preview = null;
				previewStatus = "idle";
				previewError = null;
			}
			return;
		}

		previewStatus = "loading";
		previewError = null;
		preview = null;

		let cancelled = false;
		void fetchCommitPullPreview(sessionId, threadId, approvalId)
			.then((result) => {
				if (cancelled) {
					return;
				}
				preview = result;
				previewStatus = "ready";
			})
			.catch((error) => {
				if (cancelled) {
					return;
				}
				previewStatus = "error";
				previewError =
					error instanceof Error
						? error.message
						: "Failed to load commit preview";
			});

		return () => {
			cancelled = true;
		};
	});

	async function submitAnswers(
		answers: Record<string, string>,
		approved: boolean,
	) {
		approvalError = null;
		if (!sessionId || !threadId || !approvalId) {
			approvalError = "Missing thread context";
			return;
		}
		try {
			await api.submitThreadChatAnswer(sessionId, threadId, {
				toolUseID: approvalId,
				answers,
			});
			onToolApprovalResponse?.({
				id: approvalId,
				approved,
				reason: approved ? undefined : rejectionReason.trim() || undefined,
			});
			approvalStatus = "answered";
			pendingQuestion = null;
			rejectDialogOpen = false;
		} catch (error) {
			approvalStatus = "pending";
			approvalError =
				error instanceof Error
					? error.message
					: "Failed to submit commit pull response";
		}
	}

	function approve() {
		void submitAnswers({ [APPROVED_KEY]: "true" }, true);
	}

	function openRejectDialog() {
		approvalError = null;
		rejectDialogOpen = true;
	}

	function reject() {
		const trimmedReason = rejectionReason.trim();
		if (!trimmedReason) {
			approvalError = "Add a reason before rejecting the commit pull request.";
			return;
		}
		void submitAnswers(
			{
				[REJECTED_KEY]: "true",
				[REJECTED_REASON_KEY]: trimmedReason,
			},
			false,
		);
	}

	async function retry() {
		if (!sessionId || !threadId || !approvalId || !previewError) {
			approvalError = "Missing retry context";
			return;
		}

		const retryReason = `Please retry the commit pull request because the confirmation preview failed to load: ${previewError}`;

		retrying = true;
		approvalError = null;
		try {
			await api.submitThreadChatAnswer(sessionId, threadId, {
				toolUseID: approvalId,
				answers: {
					[REJECTED_KEY]: "true",
					[REJECTED_REASON_KEY]: retryReason,
				},
			});
			onToolApprovalResponse?.({
				id: approvalId,
				approved: false,
				reason: retryReason,
			});
			approvalStatus = "answered";
			pendingQuestion = null;
		} catch (error) {
			approvalStatus = "pending";
			approvalError =
				error instanceof Error
					? error.message
					: "Failed to retry commit pull request";
		} finally {
			retrying = false;
		}
	}

	function getFileHunks(file: PreviewFileEntry): ParsedDiffHunk[] {
		return file.patch ? parseUnifiedDiff(file.patch) : [];
	}

	function buildPreviewDiffParams(
		file: PreviewFileEntry,
	): DiffRendererParams | null {
		if (file.binary || !file.patch) {
			return null;
		}

		const hunks = getFileHunks(file);
		if (hunks.length === 0) {
			return null;
		}

		const oldLines: string[] = [];
		const newLines: string[] = [];
		for (const [index, hunk] of hunks.entries()) {
			if (index > 0) {
				oldLines.push("", "⋯");
				newLines.push("", "⋯");
			}
			for (const line of hunk.lines) {
				if (line.marker !== "+") {
					oldLines.push(line.content);
				}
				if (line.marker !== "-") {
					newLines.push(line.content);
				}
			}
		}

		const oldPath = file.oldPath ?? file.path;
		return {
			diffStyle,
			resolvedTheme,
			oldFile: buildDiffFileContents(
				oldPath,
				oldLines.join("\n"),
				`${file.commitHash}:${oldPath}:old`,
			),
			newFile: buildDiffFileContents(
				file.path,
				newLines.join("\n"),
				`${file.commitHash}:${file.path}:new`,
			),
			virtualized: false,
		};
	}
</script>

<div class="flex items-center justify-between gap-4 px-4 pt-4">
	<CollapsibleTrigger
		class="flex min-w-0 flex-1 items-center gap-2 text-left text-muted-foreground"
		disabled={toolPart.state === "approval-requested"}
	>
		<GitCommitHorizontalIcon class="size-4 shrink-0 text-muted-foreground" />
		<span class="truncate font-medium text-sm">Pull Sandbox Commit</span>
		<ToolHeaderStatus state={toolPart.state} />
	</CollapsibleTrigger>
	<ToolHeaderControls
		{isRaw}
		{onToggleRaw}
		canCollapse={toolPart.state !== "approval-requested"}
	/>
</div>

<ToolContent>
	<div class="space-y-4 p-4 pt-3">
		{#if approvalStatus === "loading"}
			<div class="flex items-center gap-2 text-muted-foreground text-sm">
				<Loader2Icon class="size-4 animate-spin" />
				<span>Loading commit pull approval…</span>
			</div>
		{:else if approvalStatus === "error"}
			<div
				class="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-destructive text-sm"
			>
				{approvalError ?? "Failed to load commit pull request."}
			</div>
		{:else if pendingQuestion}
			<div class="space-y-3 rounded-lg border bg-card p-4">
				<p class="font-medium text-sm">
					{pendingQuestion.questions?.[0]?.question ??
						"Allow the sandbox commit to be pulled into the workspace?"}
				</p>

				<div class="space-y-2 rounded-md border bg-muted/20 p-3 text-sm">
					<div class="space-y-1">
						<div
							class="flex items-center gap-2 text-muted-foreground text-xs uppercase tracking-wide"
						>
							<span class="font-medium">Commit</span>
							{#if shortCommitHash}
								<span class="font-mono normal-case tracking-normal"
									>{shortCommitHash}</span
								>
							{/if}
						</div>
						<p class="font-medium break-words">{commitTitle}</p>
					</div>
					{#if commitBody}
						<p class="whitespace-pre-wrap text-muted-foreground text-sm">
							{commitBody}
						</p>
					{/if}
					{#if preview?.stats}
						<div class="flex flex-wrap gap-2 text-xs">
							<span
								class="rounded-full bg-background px-2 py-0.5 text-muted-foreground"
							>
								{preview.commitCount}
								{preview.commitCount === 1 ? "commit" : "commits"}
							</span>
							<span
								class="rounded-full bg-background px-2 py-0.5 text-muted-foreground"
							>
								{preview.stats.filesChanged} files
							</span>
							<span
								class="rounded-full bg-background px-2 py-0.5 text-muted-foreground"
							>
								+{preview.stats.additions} / -{preview.stats.deletions}
							</span>
							<span
								class="rounded-full bg-background px-2 py-0.5 text-muted-foreground"
							>
								{preview.stats.lineCount} changed lines
							</span>
						</div>
					{/if}
				</div>

				<div class="flex flex-wrap items-center justify-between gap-3">
					<div class="flex items-center gap-2">
						{#if previewStatus === "loading"}
							<div
								class="flex items-center gap-2 text-muted-foreground text-sm"
							>
								<Loader2Icon class="size-4 animate-spin" />
								<span>Loading replay bundle preview…</span>
							</div>
						{:else if previewStatus === "error"}
							<div
								class="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-destructive text-sm"
							>
								{previewError ?? "Failed to load commit preview."}
							</div>
						{:else if preview}
							<Button
								variant="outline"
								size="icon-sm"
								onclick={() => (diffDialogOpen = true)}
								aria-label="Show diff"
								title="Show diff"
							>
								<EyeIcon class="size-4" />
							</Button>
							<Button
								variant="outline"
								size="icon-sm"
								onclick={() => (rawPatchDialogOpen = true)}
								aria-label="Show raw patch"
								title="Show raw patch"
							>
								<FileCodeIcon class="size-4" />
							</Button>
						{/if}
						{#if summary}
							<Button
								variant="outline"
								size="icon-sm"
								onclick={() => (notesDialogOpen = true)}
								aria-label="Show notes from the agent"
								title="Show notes from the agent"
							>
								<FileTextIcon class="size-4" />
							</Button>
						{/if}
					</div>

					<div class="ml-auto flex flex-wrap items-center justify-end gap-2">
						{#if previewStatus === "error"}
							<Button onclick={retry} disabled={retrying}>
								{retrying ? "Retrying…" : "Retry"}
							</Button>
							<Button
								onclick={openRejectDialog}
								variant="outline"
								disabled={retrying}
							>
								Reject
							</Button>
						{:else}
							<Button onclick={approve}>Approve</Button>
							<Button onclick={openRejectDialog} variant="outline"
								>Reject</Button
							>
						{/if}
					</div>
				</div>
				{#if approvalError}
					<div
						class="rounded-md border border-destructive/20 bg-destructive/10 p-3 text-destructive text-sm"
					>
						{approvalError}
					</div>
				{/if}
			</div>
			<Dialog.Root bind:open={notesDialogOpen}>
				<Dialog.Content class="sm:max-w-lg">
					<RequestCommitPullNotesDialogContent notes={summary} />
				</Dialog.Content>
			</Dialog.Root>
			<Dialog.Root bind:open={diffDialogOpen}>
				<Dialog.Content class="max-h-[85vh] sm:max-w-6xl">
					<Dialog.Header>
						<Dialog.Title>Commit diff</Dialog.Title>
						<Dialog.Description>
							Review the prepared sandbox patch before approving it.
						</Dialog.Description>
					</Dialog.Header>
					<RequestCommitPullDiffViewer
						files={previewDiffEntries}
						{diffStyle}
						onDiffStyleChange={(nextStyle) => (diffStyle = nextStyle)}
					/>
				</Dialog.Content>
			</Dialog.Root>
			<Dialog.Root bind:open={rawPatchDialogOpen}>
				<Dialog.Content class="max-h-[85vh] sm:max-w-5xl">
					<RequestCommitPullRawPatchDialogContent
						rawPatch={preview?.rawPatch ?? ""}
					/>
				</Dialog.Content>
			</Dialog.Root>
			<Dialog.Root bind:open={rejectDialogOpen}>
				<Dialog.Content class="sm:max-w-md">
					<Dialog.Header>
						<Dialog.Title>Reject commit pull?</Dialog.Title>
						<Dialog.Description>
							Tell Discobot why this commit should not be pulled into the host
							workspace.
						</Dialog.Description>
					</Dialog.Header>
					<div class="space-y-2">
						<label class="font-medium text-sm" for="commit-pull-reason">
							Reason
						</label>
						<Input
							id="commit-pull-reason"
							name="discobot-commit-pull-reason"
							bind:value={rejectionReason}
							autocomplete="off"
							autocorrect="off"
							autocapitalize="off"
							data-form-type="other"
							placeholder="Explain why the commit should not be pulled"
						/>
					</div>
					<Dialog.Footer>
						<Button
							variant="ghost"
							onclick={() => {
								rejectDialogOpen = false;
							}}
						>
							Cancel
						</Button>
						<Button onclick={reject} disabled={!rejectionReason.trim()}>
							Reject commit
						</Button>
					</Dialog.Footer>
				</Dialog.Content>
			</Dialog.Root>
		{:else if rejectionSummary !== null}
			<div class="rounded-md border bg-muted/20 p-3 text-sm">
				<p class="font-medium text-sm">Commit pull rejected.</p>
				{#if rejectionSummary}
					<p class="mt-1 text-muted-foreground text-sm">{rejectionSummary}</p>
				{/if}
			</div>
		{:else if wasApproved}
			<div class="rounded-md border bg-muted/20 p-3 text-sm">
				Commit pull approved. Discobot is applying the sandbox commit.
			</div>
		{:else if summary}
			<div class="rounded-md border bg-muted/20 p-3 text-sm">{summary}</div>
		{:else}
			<p class="text-muted-foreground text-sm">
				Waiting for commit pull details…
			</p>
		{/if}
	</div>
</ToolContent>
