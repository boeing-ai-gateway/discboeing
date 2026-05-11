<script lang="ts">
	import type { FileContents, SelectedLineRange } from "@pierre/diffs";
	import CheckIcon from "@lucide/svelte/icons/check";
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";

	import { writeStorage } from "$lib/local-storage";
	import { api } from "$lib/api-client";
	import type {
		SessionDiffFileEntry,
		SessionDiffStats,
		SessionSingleFileDiffResponse,
	} from "$lib/api-types";
	import DockWindowChrome from "$lib/components/app/parts/DockWindowChrome.svelte";
	import DiffReviewFileRenderer from "$lib/components/app/parts/DiffReviewFileRenderer.svelte";
	import { Badge } from "$lib/components/ui/badge";
	import { Button } from "$lib/components/ui/button";
	import { Checkbox } from "$lib/components/ui/checkbox";
	import {
		DropdownMenu,
		DropdownMenuContent,
		DropdownMenuItem,
		DropdownMenuSeparator,
		DropdownMenuTrigger,
	} from "$lib/components/ui/dropdown-menu";
	import { Input } from "$lib/components/ui/input";
	import { Textarea } from "$lib/components/ui/textarea";
	import {
		buildDiffFileContents,
		DIFF_HARD_LIMIT,
		DIFF_WARNING_THRESHOLD,
		type DiffRendererParams,
		type DiffStyle,
	} from "$lib/pierre-diff";
	import {
		buildWhitespaceIgnoredFileDiff,
		equalIgnoringWhitespace,
	} from "$lib/pierre-diff-utils";
	import {
		countDiffLinesFast,
		hashString,
		reconstructOriginalFromPatch,
	} from "$lib/diff-utils";
	import type { ResolvedTheme } from "$lib/theme";
	import { cn } from "$lib/utils";
	import { onMount, untrack } from "svelte";
	import { SvelteMap } from "svelte/reactivity";

	const APPROVAL_STORAGE_KEY = "discobot.ui.diff-review.approved";
	const DIFF_STYLE_STORAGE_KEY = "discobot.ui.diff-review.style";
	const APPROVAL_LOAD_CONCURRENCY = 6;

	type Props = {
		dockMaximized: boolean;
		onClose: () => void;
		onDiffTargetChange: (target: string) => Promise<void> | void;
		onOpenFile: (path: string) => Promise<void> | void;
		onRefresh: () => Promise<void> | void;
		onQueueSelectionComment: (payload: {
			path: string;
			selectedText: string;
			comment: string;
		}) => Promise<void> | void;
		onSubmitSelectionComment: (payload: {
			path: string;
			selectedText: string;
			comment: string;
		}) => Promise<void> | void;
		onToggleDockMaximized: () => void;
		sessionId: string;
		diff: SessionDiffFileEntry[];
		diffTarget: string;
		fileContents: Record<string, string>;
		diffStats: SessionDiffStats;
		resolvedTheme: ResolvedTheme;
		shiftWindowControlsForSidebar?: boolean;
	};

	type SnapshotState = {
		originalContent: string;
		modifiedContent: string;
		source: "reverse-patch" | "base-read";
	};

	type ReadyDiffState = {
		status: "ready";
		response: SessionSingleFileDiffResponse;
		patchHash: string | null;
		lineCount: number;
		snapshotStatus: "idle" | "loading" | "ready" | "error";
		snapshot?: SnapshotState;
		snapshotError?: string;
		oldFile?: FileContents;
		newFile?: FileContents;
	};

	type DiffSelectionState = {
		range: SelectedLineRange;
		text: string;
	};

	type CommentPopoverPosition = {
		top: number;
		left: number;
	};

	type LoadedDiffState =
		| {
				status: "idle";
				snapshotStatus: "idle";
		  }
		| {
				status: "loading";
				snapshotStatus: "idle";
		  }
		| {
				status: "error";
				errorMessage: string;
				snapshotStatus: "idle";
		  }
		| ReadyDiffState;

	let {
		dockMaximized,
		onClose,
		onDiffTargetChange,
		onOpenFile,
		onRefresh,
		onQueueSelectionComment,
		onSubmitSelectionComment,
		onToggleDockMaximized,
		sessionId,
		diff,
		diffTarget,
		fileContents,
		diffStats,
		resolvedTheme,
		shiftWindowControlsForSidebar = false,
	}: Props = $props();

	const diffStates = new SvelteMap<string, LoadedDiffState>();
	let approvedBySession = $state<Record<string, Record<string, string>>>({});
	let storageLoaded = $state(false);
	let listReady = $state(false);
	let expandedPath = $state<string | null>(null);
	let refreshing = $state(false);
	let diffTargetDraft = $state("");
	let loadGeneration = 0;
	let diffStyle = $state<DiffStyle>("unified");
	let ignoreWhitespaceByPath = $state<Record<string, boolean>>({});
	let resolvedApprovalCount = $state(0);
	let approvedCount = $state(0);
	let selectedDiffTextByPath = $state<
		Record<string, DiffSelectionState | null>
	>({});
	let commentDraftByPath = $state<Record<string, string>>({});
	let queueingCommentByPath = $state<Record<string, boolean>>({});
	let submittingCommentByPath = $state<Record<string, boolean>>({});
	let commentErrorByPath = $state<Record<string, string | null>>({});
	let commentPopoverPositionByPath = $state<
		Record<string, CommentPopoverPosition | null>
	>({});
	let lastPointerPositionByPath: Record<string, CommentPopoverPosition | null> =
		{};

	const diffCount = $derived.by(() => diff.length);
	const sortedDiff = $derived.by(() =>
		listReady
			? [...diff].toSorted((left, right) => left.path.localeCompare(right.path))
			: diff,
	);
	const sessionApprovals = $derived.by(
		() => approvedBySession[sessionId] ?? {},
	);
	const approvalsLoading = $derived.by(
		() => diffCount > 0 && resolvedApprovalCount < diffCount,
	);
	const allApproved = $derived.by(
		() => diffCount > 0 && approvedCount === diffCount,
	);
	const customDiffTargetActive = $derived.by(
		() => diffTarget !== "" && diffTarget !== "HEAD",
	);
	const diffTargetLabel = $derived.by(() => {
		if (diffTarget === "") {
			return "Merge base";
		}
		if (diffTarget === "HEAD") {
			return "HEAD";
		}
		return diffTarget;
	});
	const maximizeTitle = $derived.by(() =>
		dockMaximized ? "Restore split view" : "Maximize diff review panel",
	);

	onMount(() => {
		approvedBySession = readApprovalState();
		diffStyle = readDiffStyle();
		diffTargetDraft = diffTarget === "HEAD" ? "" : diffTarget;
		storageLoaded = true;

		const frameId = requestAnimationFrame(() => {
			listReady = true;
		});

		return () => {
			cancelAnimationFrame(frameId);
		};
	});

	$effect(() => {
		diffTargetDraft = diffTarget === "HEAD" ? "" : diffTarget;
	});

	$effect(() => {
		if (!storageLoaded) {
			return;
		}
		writeStorage(APPROVAL_STORAGE_KEY, JSON.stringify(approvedBySession));
	});

	$effect(() => {
		if (!storageLoaded) {
			return;
		}
		writeStorage(DIFF_STYLE_STORAGE_KEY, diffStyle);
	});

	$effect(() => {
		const currentSessionId = sessionId;
		const currentEntries = diff;
		const currentDiffTarget = diffTarget;
		void currentEntries;
		void currentDiffTarget;

		loadGeneration += 1;
		clearDiffStates();
		ignoreWhitespaceByPath = {};

		if (!currentSessionId || currentEntries.length === 0) {
			expandedPath = null;
			return;
		}

		const currentExpandedPath = untrack(() => expandedPath);
		if (
			currentExpandedPath &&
			!currentEntries.some((file) => file.path === currentExpandedPath)
		) {
			expandedPath = null;
		}
	});

	$effect(() => {
		const currentExpandedPath = expandedPath;
		const currentSessionId = sessionId;
		const currentDiffTarget = diffTarget;
		const generation = loadGeneration;
		if (!currentExpandedPath || !currentSessionId) {
			return;
		}
		void ensureExpandedDiffReady(
			currentExpandedPath,
			currentSessionId,
			currentDiffTarget,
			generation,
		);
	});

	$effect(() => {
		const ready = listReady;
		const currentEntries = sortedDiff;
		const currentSessionId = sessionId;
		const currentDiffTarget = diffTarget;
		const generation = loadGeneration;
		if (!ready || !currentSessionId || currentEntries.length === 0) {
			return;
		}

		let cancelled = false;
		const queue = currentEntries.map((file) => file.path);
		const workerCount = Math.min(APPROVAL_LOAD_CONCURRENCY, queue.length);

		async function worker() {
			while (!cancelled) {
				const nextPath = queue.shift();
				if (!nextPath) {
					return;
				}
				await loadDiffEntry(
					nextPath,
					currentSessionId,
					currentDiffTarget,
					generation,
				);
				if (
					cancelled ||
					generation !== loadGeneration ||
					currentSessionId !== sessionId
				) {
					return;
				}
			}
		}

		void Promise.all(Array.from({ length: workerCount }, () => worker()));

		return () => {
			cancelled = true;
		};
	});

	function createIdleState(): LoadedDiffState {
		return { status: "idle", snapshotStatus: "idle" };
	}

	function clearDiffStates() {
		diffStates.clear();
		resolvedApprovalCount = 0;
		approvedCount = 0;
		selectedDiffTextByPath = {};
		commentDraftByPath = {};
		queueingCommentByPath = {};
		submittingCommentByPath = {};
		commentErrorByPath = {};
		commentPopoverPositionByPath = {};
		lastPointerPositionByPath = {};
	}

	function getDiffState(path: string): LoadedDiffState | undefined {
		return diffStates.get(path);
	}

	function isResolvedState(state: LoadedDiffState | undefined): boolean {
		return (
			state !== undefined &&
			state.status !== "idle" &&
			state.status !== "loading"
		);
	}

	function isApprovedState(
		path: string,
		state: LoadedDiffState | undefined,
		approvals = sessionApprovals,
	): boolean {
		const patchHash = state?.status === "ready" ? state.patchHash : null;
		return Boolean(patchHash && approvals[path] === patchHash);
	}

	function setDiffState(path: string, nextState: LoadedDiffState) {
		const previousState = diffStates.get(path);
		const wasResolved = isResolvedState(previousState);
		const nextResolved = isResolvedState(nextState);
		const wasApproved = isApprovedState(path, previousState);
		const nextApproved = isApprovedState(path, nextState);

		diffStates.set(path, nextState);

		if (!wasResolved && nextResolved) {
			resolvedApprovalCount += 1;
		} else if (wasResolved && !nextResolved) {
			resolvedApprovalCount = Math.max(0, resolvedApprovalCount - 1);
		}

		if (!wasApproved && nextApproved) {
			approvedCount += 1;
		} else if (wasApproved && !nextApproved) {
			approvedCount = Math.max(0, approvedCount - 1);
		}
	}

	function updateDiffState(
		path: string,
		updater: (current: LoadedDiffState) => LoadedDiffState,
	) {
		const current = diffStates.get(path) ?? createIdleState();
		setDiffState(path, updater(current));
	}

	function recalculateApprovedCount() {
		let nextApprovedCount = 0;
		for (const file of diff) {
			if (isApprovedState(file.path, diffStates.get(file.path))) {
				nextApprovedCount += 1;
			}
		}
		approvedCount = nextApprovedCount;
	}

	function readApprovalState(): Record<string, Record<string, string>> {
		if (typeof window === "undefined") {
			return {};
		}
		const stored = window.localStorage.getItem(APPROVAL_STORAGE_KEY);
		if (!stored) {
			return {};
		}
		try {
			const parsed = JSON.parse(stored);
			return typeof parsed === "object" && parsed !== null ? parsed : {};
		} catch {
			return {};
		}
	}

	function readDiffStyle(): DiffStyle {
		if (typeof window === "undefined") {
			return "unified";
		}
		const stored = window.localStorage.getItem(DIFF_STYLE_STORAGE_KEY);
		return stored === "split" ? "split" : "unified";
	}

	function errorMessage(error: unknown): string {
		if (error instanceof Error && error.message.length > 0) {
			return error.message;
		}
		return "Unable to load diff.";
	}

	async function loadDiffEntry(
		path: string,
		currentSessionId: string,
		currentDiffTarget: string,
		generation: number,
	) {
		const state = getDiffState(path);
		if (state?.status === "loading" || state?.status === "ready") {
			return;
		}

		setDiffState(path, {
			status: "loading",
			snapshotStatus: "idle",
		});

		try {
			const response = (await api.getSessionDiff(currentSessionId, {
				path,
				target: currentDiffTarget,
			})) as SessionSingleFileDiffResponse;
			const patchHash = response.patch
				? await hashString(response.patch)
				: null;
			if (generation !== loadGeneration || currentSessionId !== sessionId) {
				return;
			}

			setDiffState(path, {
				status: "ready",
				response,
				patchHash,
				lineCount: response.patch ? countDiffLinesFast(response.patch) : 0,
				snapshotStatus: "idle",
			});
		} catch (error) {
			if (generation !== loadGeneration || currentSessionId !== sessionId) {
				return;
			}
			setDiffState(path, {
				status: "error",
				errorMessage: errorMessage(error),
				snapshotStatus: "idle",
			});
		}
	}

	async function ensureExpandedDiffReady(
		path: string,
		currentSessionId: string,
		currentDiffTarget: string,
		generation: number,
	) {
		await loadDiffEntry(path, currentSessionId, currentDiffTarget, generation);
		if (generation !== loadGeneration || currentSessionId !== sessionId) {
			return;
		}
		await ensureSnapshot(path, currentSessionId, generation);
	}

	async function ensureSnapshot(
		path: string,
		currentSessionId: string,
		generation: number,
	) {
		const state = getDiffState(path);
		if (!state || state.status !== "ready") {
			return;
		}
		if (
			state.snapshotStatus === "loading" ||
			state.snapshotStatus === "ready"
		) {
			return;
		}
		if (state.response.binary || !state.response.patch) {
			return;
		}

		updateDiffState(path, (current) => {
			if (current.status !== "ready") {
				return current;
			}
			return {
				...current,
				snapshotStatus: "loading",
			};
		});

		try {
			let originalContent = "";
			let modifiedContent = "";
			let source: SnapshotState["source"] = "reverse-patch";

			if (state.response.status === "deleted") {
				originalContent = reconstructOriginalFromPatch(
					"",
					state.response.patch,
				);
				if (originalContent.length === 0 && state.response.deletions > 0) {
					const baseFile = await api.readSessionFile(currentSessionId, path, {
						fromBase: true,
					});
					originalContent = baseFile.content;
					source = "base-read";
				}
			} else {
				modifiedContent =
					fileContents[path] ??
					(await api.readSessionFile(currentSessionId, path)).content;
				originalContent =
					state.response.status === "added"
						? ""
						: reconstructOriginalFromPatch(
								modifiedContent,
								state.response.patch,
							);
			}

			if (generation !== loadGeneration || currentSessionId !== sessionId) {
				return;
			}

			updateDiffState(path, (current) => {
				if (current.status !== "ready") {
					return current;
				}
				const oldPath = current.response.oldPath ?? path;
				const oldFile = buildDiffFileContents(
					oldPath,
					originalContent,
					current.patchHash ? `${current.patchHash}:old` : null,
				);
				const newFile = buildDiffFileContents(
					path,
					modifiedContent,
					current.patchHash ? `${current.patchHash}:new` : null,
				);
				return {
					...current,
					snapshotStatus: "ready",
					snapshot: {
						originalContent,
						modifiedContent,
						source,
					},
					oldFile,
					newFile,
				};
			});
		} catch (error) {
			if (generation !== loadGeneration || currentSessionId !== sessionId) {
				return;
			}
			updateDiffState(path, (current) => {
				if (current.status !== "ready") {
					return current;
				}
				return {
					...current,
					snapshotStatus: "error",
					snapshotError: errorMessage(error),
				};
			});
		}
	}

	function toggleExpanded(path: string) {
		expandedPath = expandedPath === path ? null : path;
	}

	function isApproved(path: string): boolean {
		return isApprovedState(path, getDiffState(path));
	}

	function isApprovalStateLoading(path: string): boolean {
		const state = getDiffState(path);
		return !state || state.status === "idle" || state.status === "loading";
	}

	function toggleApproved(path: string) {
		const state = getDiffState(path);
		if (!state || state.status !== "ready" || !state.patchHash) {
			return;
		}

		const nextSessionApprovals = { ...sessionApprovals };
		const wasApproved = nextSessionApprovals[path] === state.patchHash;
		if (wasApproved) {
			delete nextSessionApprovals[path];
		} else {
			nextSessionApprovals[path] = state.patchHash;
		}

		approvedBySession = {
			...approvedBySession,
			[sessionId]: nextSessionApprovals,
		};
		recalculateApprovedCount();

		if (!wasApproved && expandedPath === path) {
			const currentIndex = sortedDiff.findIndex((file) => file.path === path);
			const nextUnapproved = sortedDiff.slice(currentIndex + 1).find((file) => {
				const nextState = getDiffState(file.path);
				const nextHash =
					nextState && nextState.status === "ready"
						? nextState.patchHash
						: null;
				return !(nextHash && nextSessionApprovals[file.path] === nextHash);
			});
			expandedPath = nextUnapproved?.path ?? null;
		}
	}

	function markAllApproved() {
		const nextSessionApprovals: Record<string, string> = {};
		for (const file of sortedDiff) {
			const state = getDiffState(file.path);
			if (state?.status === "ready" && state.patchHash) {
				nextSessionApprovals[file.path] = state.patchHash;
			}
		}
		approvedBySession = {
			...approvedBySession,
			[sessionId]: nextSessionApprovals,
		};
		recalculateApprovedCount();
	}

	function statusBadgeClass(
		status:
			| SessionSingleFileDiffResponse["status"]
			| SessionDiffFileEntry["status"],
	) {
		switch (status) {
			case "added":
				return "text-green-500 border-green-500/40";
			case "modified":
				return "text-yellow-500 border-yellow-500/40";
			case "deleted":
				return "text-red-500 border-red-500/40";
			case "renamed":
				return "text-purple-500 border-purple-500/40";
			default:
				return "text-muted-foreground border-border";
		}
	}

	function statusLabel(
		status:
			| SessionSingleFileDiffResponse["status"]
			| SessionDiffFileEntry["status"],
	) {
		switch (status) {
			case "added":
				return "Added";
			case "modified":
				return "Modified";
			case "deleted":
				return "Deleted";
			case "renamed":
				return "Renamed";
			default:
				return "Changed";
		}
	}

	function showLargeDiffFallback(state: LoadedDiffState): boolean {
		return state.status === "ready" && state.lineCount > DIFF_HARD_LIMIT;
	}

	function useVirtualizedDiff(state: LoadedDiffState): boolean {
		return state.status === "ready" && state.lineCount > DIFF_WARNING_THRESHOLD;
	}

	function ignoringWhitespace(path: string): boolean {
		return ignoreWhitespaceByPath[path] ?? false;
	}

	function toggleIgnoreWhitespace(path: string, checked: boolean) {
		ignoreWhitespaceByPath = {
			...ignoreWhitespaceByPath,
			[path]: checked,
		};
	}

	function hasOnlyWhitespaceChanges(state: ReadyDiffState): boolean {
		if (state.snapshotStatus !== "ready" || !state.snapshot) {
			return false;
		}
		return equalIgnoringWhitespace(
			state.snapshot.originalContent,
			state.snapshot.modifiedContent,
		);
	}

	function getSelectedDiffText(path: string): string {
		return selectedDiffTextByPath[path]?.text ?? "";
	}

	function getSelectedLines(path: string): SelectedLineRange | null {
		return selectedDiffTextByPath[path]?.range ?? null;
	}

	function getCommentDraft(path: string): string {
		return commentDraftByPath[path] ?? "";
	}

	function getCommentError(path: string): string | null {
		return commentErrorByPath[path] ?? null;
	}

	function getCommentPopoverPosition(path: string) {
		return commentPopoverPositionByPath[path] ?? null;
	}

	function findDiffReviewFileElement(path: string) {
		return Array.from(
			document.querySelectorAll<HTMLElement>("[data-diff-review-file]"),
		).find((element) => element.dataset.diffReviewPath === path);
	}

	function updateCommentPopoverPosition(path: string) {
		requestAnimationFrame(() => {
			const fileElement = findDiffReviewFileElement(path);
			if (!fileElement) {
				commentPopoverPositionByPath = {
					...commentPopoverPositionByPath,
					[path]: null,
				};
				return;
			}

			const selectedLine = fileElement.querySelector<HTMLElement>(
				"[data-selected-line]",
			);
			const anchorRect =
				selectedLine?.getBoundingClientRect() ??
				fileElement.getBoundingClientRect();
			const pointerPosition = lastPointerPositionByPath[path] ?? null;
			const popoverWidth = Math.min(window.innerWidth - 24, 384);
			const anchorLeft = pointerPosition?.left ?? anchorRect.left + 36;
			const anchorTop = pointerPosition?.top ?? anchorRect.bottom;
			const left = Math.min(
				Math.max(anchorLeft + 12, 12),
				window.innerWidth - popoverWidth - 12,
			);
			const belowTop = anchorTop + 12;
			const top =
				belowTop + 260 <= window.innerHeight
					? belowTop
					: Math.max(12, Math.min(anchorTop - 272, window.innerHeight - 272));
			commentPopoverPositionByPath = {
				...commentPopoverPositionByPath,
				[path]: { top, left },
			};
		});
	}

	function trackDiffPointer(path: string, event: PointerEvent) {
		lastPointerPositionByPath = {
			...lastPointerPositionByPath,
			[path]: {
				top: event.clientY,
				left: event.clientX,
			},
		};
	}

	function normalizeSelectedLineRange(
		range: SelectedLineRange,
	): SelectedLineRange {
		return {
			...range,
			start: Math.min(range.start, range.end),
			end: Math.max(range.start, range.end),
		};
	}

	function getSelectedSideText(
		state: ReadyDiffState,
		side: "deletions" | "additions",
		start: number,
		end: number,
	): string {
		if (state.snapshotStatus !== "ready" || !state.snapshot) {
			return "";
		}
		const source =
			side === "deletions"
				? state.snapshot.originalContent
				: state.snapshot.modifiedContent;
		const lines = source.split("\n");
		return lines
			.slice(Math.max(start - 1, 0), Math.max(end, 0))
			.join("\n")
			.trim();
	}

	function formatSelectedSideLabel(
		side: "deletions" | "additions",
		start: number,
		end: number,
	): string {
		return `${side === "deletions" ? "Deletion" : "Addition"} lines ${start}-${end}`;
	}

	function buildSelectedDiffText(
		state: ReadyDiffState,
		range: SelectedLineRange,
	): string {
		const normalizedRange = normalizeSelectedLineRange(range);
		const startSide =
			normalizedRange.side ?? normalizedRange.endSide ?? "additions";
		const endSide =
			normalizedRange.endSide ?? normalizedRange.side ?? startSide;

		if (startSide === endSide) {
			const excerpt = getSelectedSideText(
				state,
				startSide,
				normalizedRange.start,
				normalizedRange.end,
			);
			if (!excerpt) {
				return "";
			}
			return `${formatSelectedSideLabel(
				startSide,
				normalizedRange.start,
				normalizedRange.end,
			)}:\n${excerpt}`;
		}

		const startExcerpt = getSelectedSideText(
			state,
			startSide,
			normalizedRange.start,
			normalizedRange.start,
		);
		const endExcerpt = getSelectedSideText(
			state,
			endSide,
			normalizedRange.end,
			normalizedRange.end,
		);
		return [
			startExcerpt
				? `${formatSelectedSideLabel(
						startSide,
						normalizedRange.start,
						normalizedRange.start,
					)}:\n${startExcerpt}`
				: "",
			endExcerpt
				? `${formatSelectedSideLabel(
						endSide,
						normalizedRange.end,
						normalizedRange.end,
					)}:\n${endExcerpt}`
				: "",
		]
			.filter(Boolean)
			.join("\n\n");
	}

	function handleLineSelection(
		path: string,
		state: ReadyDiffState,
		range: SelectedLineRange | null,
	) {
		const normalizedRange = range ? normalizeSelectedLineRange(range) : null;
		const nextSelection = normalizedRange
			? buildSelectedDiffText(state, normalizedRange)
			: "";
		selectedDiffTextByPath = {
			...selectedDiffTextByPath,
			[path]:
				normalizedRange && nextSelection
					? {
							range: normalizedRange,
							text: nextSelection,
						}
					: null,
		};
		commentErrorByPath = {
			...commentErrorByPath,
			[path]: null,
		};
		if (nextSelection.length === 0) {
			commentDraftByPath = {
				...commentDraftByPath,
				[path]: "",
			};
			commentPopoverPositionByPath = {
				...commentPopoverPositionByPath,
				[path]: null,
			};
		} else {
			updateCommentPopoverPosition(path);
		}
	}

	function updateCommentDraft(path: string, value: string) {
		commentDraftByPath = {
			...commentDraftByPath,
			[path]: value,
		};
	}

	function resetSelectionComment(path: string) {
		selectedDiffTextByPath = {
			...selectedDiffTextByPath,
			[path]: null,
		};
		commentDraftByPath = {
			...commentDraftByPath,
			[path]: "",
		};
		commentErrorByPath = {
			...commentErrorByPath,
			[path]: null,
		};
		commentPopoverPositionByPath = {
			...commentPopoverPositionByPath,
			[path]: null,
		};
		lastPointerPositionByPath = {
			...lastPointerPositionByPath,
			[path]: null,
		};
	}

	async function saveSelectionComment(
		path: string,
		action: "queue" | "submit",
	) {
		const selectedText = getSelectedDiffText(path).trim();
		const comment = getCommentDraft(path).trim();
		if (!selectedText || !comment) {
			commentErrorByPath = {
				...commentErrorByPath,
				[path]: `Select diff text and add a comment before ${action === "queue" ? "queueing" : "submitting"}.`,
			};
			return;
		}

		if (action === "queue") {
			queueingCommentByPath = {
				...queueingCommentByPath,
				[path]: true,
			};
		} else {
			submittingCommentByPath = {
				...submittingCommentByPath,
				[path]: true,
			};
		}
		commentErrorByPath = {
			...commentErrorByPath,
			[path]: null,
		};
		try {
			if (action === "queue") {
				await onQueueSelectionComment({ path, selectedText, comment });
			} else {
				await onSubmitSelectionComment({ path, selectedText, comment });
			}
			resetSelectionComment(path);
		} catch (error) {
			commentErrorByPath = {
				...commentErrorByPath,
				[path]:
					error instanceof Error
						? error.message
						: `Failed to ${action} diff comment`,
			};
		} finally {
			if (action === "queue") {
				queueingCommentByPath = {
					...queueingCommentByPath,
					[path]: false,
				};
			} else {
				submittingCommentByPath = {
					...submittingCommentByPath,
					[path]: false,
				};
			}
		}
	}

	function getRendererParams(
		path: string,
		state: ReadyDiffState,
	): DiffRendererParams | null {
		if (
			state.snapshotStatus !== "ready" ||
			!state.snapshot ||
			!state.oldFile ||
			!state.newFile
		) {
			return null;
		}

		if (!ignoringWhitespace(path)) {
			return {
				diffStyle,
				resolvedTheme,
				oldFile: state.oldFile,
				newFile: state.newFile,
				virtualized: useVirtualizedDiff(state),
			};
		}

		const whitespaceIgnoredDiff = buildWhitespaceIgnoredFileDiff(
			state.oldFile,
			state.newFile,
		);
		return {
			diffStyle,
			resolvedTheme,
			oldFile: state.oldFile,
			newFile: state.newFile,
			fileDiff: whitespaceIgnoredDiff,
			virtualized: useVirtualizedDiff(state),
		};
	}

	async function refreshPanel() {
		refreshing = true;
		try {
			await onRefresh();
		} finally {
			refreshing = false;
		}
	}

	async function applyDiffTarget(target: string) {
		const nextTarget = target.trim();
		diffTargetDraft = nextTarget === "HEAD" ? "" : nextTarget;
		refreshing = true;
		try {
			await onDiffTargetChange(nextTarget);
		} finally {
			refreshing = false;
		}
	}
</script>

<DockWindowChrome
	{dockMaximized}
	{onClose}
	{onToggleDockMaximized}
	{shiftWindowControlsForSidebar}
	closeLabel="Close diff review panel"
	minimizeLabel="Minimize diff review panel"
	{maximizeTitle}
	contentClass="flex min-h-0 flex-1 flex-col overflow-hidden p-3"
>
	{#snippet title()}
		<div class="flex min-w-0 items-center gap-2 text-xs">
			<p class="truncate text-sm font-medium">Diff review</p>
			<span class="truncate text-sidebar-foreground/70">
				{diffCount} changed {diffCount === 1 ? "file" : "files"}
			</span>
			{#if approvalsLoading}
				<span
					class="flex items-center gap-1 truncate text-sidebar-foreground/70"
				>
					<RefreshCwIcon class="size-3.5 animate-spin" />
					Loading approvals…
				</span>
			{:else if approvedCount > 0}
				<span class="truncate text-sidebar-foreground/70"
					>{approvedCount} approved</span
				>
			{/if}
		</div>
	{/snippet}

	{#snippet actions()}
		<div class="flex flex-wrap items-center gap-2">
			<DropdownMenu>
				<DropdownMenuTrigger>
					<Button variant="outline" size="sm" class="h-8 max-w-48">
						<span class="truncate">{diffTargetLabel}</span>
						<ChevronDownIcon class="size-4" />
					</Button>
				</DropdownMenuTrigger>
				<DropdownMenuContent align="end" class="w-72">
					<DropdownMenuItem onclick={() => void applyDiffTarget("")}>
						<span class={diffTarget === "" ? "font-medium" : ""}
							>Merge base</span
						>
					</DropdownMenuItem>
					<DropdownMenuItem onclick={() => void applyDiffTarget("HEAD")}>
						<span class={diffTarget === "HEAD" ? "font-medium" : ""}>HEAD</span>
					</DropdownMenuItem>
					<DropdownMenuSeparator />
					<div class="space-y-2 p-2">
						<div class="flex items-center gap-2">
							<Input
								class="h-8 min-w-0 flex-1 text-xs"
								placeholder="Commit, tag, branch, or range"
								value={diffTargetDraft}
								onkeydown={(event) => {
									event.stopPropagation();
									if (event.key === "Enter") {
										void applyDiffTarget(diffTargetDraft);
									}
								}}
								oninput={(event) => {
									diffTargetDraft = event.currentTarget.value;
								}}
							/>
							<Button
								variant={customDiffTargetActive ? "secondary" : "outline"}
								size="sm"
								class="h-8"
								onclick={() => void applyDiffTarget(diffTargetDraft)}
							>
								Apply
							</Button>
						</div>
					</div>
				</DropdownMenuContent>
			</DropdownMenu>
			<div
				class="inline-flex rounded-md border border-border bg-background p-0.5"
			>
				<Button
					variant={diffStyle === "unified" ? "secondary" : "ghost"}
					size="sm"
					class="h-8 rounded-r-none px-3"
					onclick={() => {
						diffStyle = "unified";
					}}
				>
					Unified
				</Button>
				<Button
					variant={diffStyle === "split" ? "secondary" : "ghost"}
					size="sm"
					class="h-8 rounded-l-none border-l border-border px-3"
					onclick={() => {
						diffStyle = "split";
					}}
				>
					Split
				</Button>
			</div>
			<Button
				variant="outline"
				size="sm"
				onclick={() => void refreshPanel()}
				disabled={refreshing}
			>
				{#if refreshing}
					<RefreshCwIcon class="size-4 animate-spin" />
				{:else}
					<RefreshCwIcon class="size-4" />
				{/if}
				Refresh
			</Button>
		</div>
	{/snippet}

	{#if diffCount === 0}
		<div
			class="rounded-md border border-sidebar-border bg-sidebar p-4 text-sm text-sidebar-foreground/60"
		>
			No changed files yet.
		</div>
	{:else}
		<div
			class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-md border border-sidebar-border bg-sidebar"
		>
			<div
				class="flex items-center justify-between gap-3 border-b border-sidebar-border px-3 py-2"
			>
				<div>
					<p
						class="text-xs font-medium uppercase tracking-[0.16em] text-sidebar-foreground/60"
					>
						{diffCount} changed {diffCount === 1 ? "file" : "files"}
					</p>
					{#if approvalsLoading}
						<p
							class="mt-1 flex items-center gap-1 text-xs text-sidebar-foreground/60"
						>
							<RefreshCwIcon class="size-3.5 animate-spin" />
							Loading approvals…
						</p>
					{:else if approvedCount > 0}
						<p class="mt-1 text-xs text-sidebar-foreground/60">
							{approvedCount} approved
						</p>
					{/if}
				</div>
				<div class="flex items-center gap-3">
					<div class="flex items-center gap-3 text-xs font-medium">
						<span class="text-green-500">+{diffStats.additions}</span>
						<span class="text-red-500">-{diffStats.deletions}</span>
					</div>
					<Button
						variant="ghost"
						size="sm"
						onclick={markAllApproved}
						disabled={allApproved || approvalsLoading}
					>
						<CheckIcon class="size-4" />
						Mark all approved
					</Button>
				</div>
			</div>

			<div class="min-h-0 flex-1 overflow-y-auto">
				{#if !listReady}
					<div class="px-3 py-4">
						<div
							class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
						>
							Preparing diff review…
						</div>
					</div>
				{:else}
					<div class="divide-y divide-sidebar-border">
						{#each sortedDiff as file (file.path)}
							{@const state = getDiffState(file.path)}
							{@const expanded = expandedPath === file.path}
							{@const approved = isApproved(file.path)}
							<section class={cn("flex flex-col", approved && "opacity-80")}>
								<button
									type="button"
									class="flex items-center justify-between gap-3 bg-sidebar/60 px-3 py-2 text-left transition hover:bg-sidebar-accent/70"
									onclick={() => toggleExpanded(file.path)}
								>
									<div class="flex min-w-0 items-center gap-2">
										{#if expanded}
											<ChevronDownIcon class="size-4 shrink-0" />
										{:else}
											<ChevronRightIcon class="size-4 shrink-0" />
										{/if}
										<Badge
											variant="outline"
											class={cn(
												"inline-grid grid-cols-1 place-items-center",
												statusBadgeClass(
													state?.status === "ready"
														? state.response.status
														: file.status,
												),
											)}
										>
											<span class="col-start-1 row-start-1"
												>{statusLabel(
													state?.status === "ready"
														? state.response.status
														: file.status,
												)}</span
											>
											<span
												class="invisible col-start-1 row-start-1"
												aria-hidden="true">Modified</span
											>
										</Badge>
										<div class="min-w-0">
											<p class="truncate font-mono text-xs text-foreground">
												{file.path}
											</p>
											{#if file.oldPath && file.oldPath !== file.path}
												<p class="truncate text-[11px] text-muted-foreground">
													{file.oldPath} → {file.path}
												</p>
											{/if}
										</div>
										{#if isApprovalStateLoading(file.path)}
											<span
												class="flex items-center gap-1 text-xs text-muted-foreground"
											>
												<RefreshCwIcon class="size-3.5 animate-spin" />
												Loading approval…
											</span>
										{:else if approved}
											<span
												class="flex items-center gap-1 text-xs text-green-500"
											>
												<CheckIcon class="size-3.5" />
												Approved
											</span>
										{/if}
									</div>
									<div
										class="flex shrink-0 items-center gap-2 text-xs font-medium"
									>
										{#if state?.status === "loading"}
											<span class="text-muted-foreground">Loading…</span>
										{:else if state?.status === "ready"}
											{#if state.response.additions > 0}
												<span class="text-green-500"
													>+{state.response.additions}</span
												>
											{/if}
											{#if state.response.deletions > 0}
												<span class="text-red-500"
													>-{state.response.deletions}</span
												>
											{/if}
										{/if}
									</div>
								</button>

								{#if expanded}
									<div class="space-y-3 px-3 py-3">
										<div
											class="flex flex-wrap items-center justify-between gap-2"
										>
											<div
												class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
											>
												{#if state?.status === "ready"}
													{#if state.response.binary}
														<span>Binary diff</span>
													{:else if state.lineCount > 0}
														<span
															>{state.lineCount.toLocaleString()} diff lines</span
														>
														{#if useVirtualizedDiff(state)}
															<span>Virtualized rendering enabled</span>
														{/if}
													{/if}
													{#if state.snapshotStatus === "ready" && state.snapshot?.source === "base-read"}
														<span>Deleted snapshot loaded from base</span>
													{:else if state.snapshotStatus === "error"}
														<span>{state.snapshotError}</span>
													{/if}
												{/if}
											</div>
											<div class="flex flex-wrap items-center gap-2">
												<label
													class="flex items-center gap-2 text-xs text-muted-foreground"
												>
													<Checkbox
														checked={ignoringWhitespace(file.path)}
														aria-label={`Ignore whitespace changes for ${file.path}`}
														onCheckedChange={(checked) =>
															toggleIgnoreWhitespace(
																file.path,
																checked === true,
															)}
													/>
													<span>Ignore whitespace</span>
												</label>
												<Button
													variant={approved ? "secondary" : "outline"}
													size="sm"
													onclick={() => toggleApproved(file.path)}
													disabled={!state || state.status !== "ready"}
												>
													<CheckIcon class="size-4" />
													{approved ? "Approved" : "Mark approved"}
												</Button>
												<Button
													variant="outline"
													size="sm"
													onclick={() => void onOpenFile(file.path)}
												>
													Open file
												</Button>
											</div>
										</div>

										{#if !state || state.status === "idle" || state.status === "loading"}
											<div
												class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
											>
												Loading diff…
											</div>
										{:else if state.status === "error"}
											<div
												class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-4 text-sm text-destructive"
											>
												{state.errorMessage}
											</div>
										{:else if state.response.binary}
											<div
												class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
											>
												This is a binary file, so the text diff cannot be
												rendered here.
											</div>
										{:else if showLargeDiffFallback(state)}
											<div
												class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
											>
												<p class="font-medium text-foreground">
													Large diff ({state.lineCount.toLocaleString()} lines)
												</p>
												<p class="mt-1">
													This file exceeds the inline rendering hard limit.
												</p>
												<div class="mt-3 flex flex-wrap items-center gap-2">
													<Button
														variant="outline"
														size="sm"
														onclick={() => void onOpenFile(file.path)}
													>
														Open file
													</Button>
												</div>
											</div>
										{:else if state.snapshotStatus !== "ready" || !state.snapshot}
											<div
												class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
											>
												Preparing interactive diff…
											</div>
										{:else if ignoringWhitespace(file.path) && hasOnlyWhitespaceChanges(state)}
											<div
												class="rounded-md border border-border bg-background px-3 py-4 text-sm text-muted-foreground"
											>
												This file only has whitespace changes.
											</div>
										{:else}
											{@const rendererParams = getRendererParams(
												file.path,
												state,
											)}
											{#if rendererParams}
												<div
													data-diff-review-file
													data-diff-review-path={file.path}
													role="presentation"
													class="overflow-hidden rounded-md border border-border bg-background"
													onpointerdown={(event) =>
														trackDiffPointer(file.path, event)}
													onpointerup={(event) =>
														trackDiffPointer(file.path, event)}
												>
													<DiffReviewFileRenderer
														params={rendererParams}
														selectedLines={getSelectedLines(file.path)}
														onLineSelected={(range) =>
															handleLineSelection(file.path, state, range)}
													/>
													{#if getSelectedDiffText(file.path) && getCommentPopoverPosition(file.path)}
														{@const popoverPosition = getCommentPopoverPosition(
															file.path,
														)}
														{#if popoverPosition}
															<div
																class="fixed z-50 w-[min(calc(100vw-1.5rem),24rem)] rounded-md border border-border bg-popover p-3 text-popover-foreground shadow-lg"
																style={`top: ${popoverPosition.top}px; left: ${popoverPosition.left}px;`}
															>
																<p
																	class="text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground"
																>
																	Selected diff text
																</p>
																<pre
																	class="mt-2 max-h-32 overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted/50 p-2 font-mono text-xs text-foreground">{getSelectedDiffText(
																		file.path,
																	)}</pre>
																<Textarea
																	class="mt-3 min-h-24"
																	placeholder="Add a comment for the assistant"
																	value={getCommentDraft(file.path)}
																	oninput={(event) =>
																		updateCommentDraft(
																			file.path,
																			event.currentTarget.value,
																		)}
																/>
																{#if getCommentError(file.path)}
																	<p class="mt-2 text-xs text-destructive">
																		{getCommentError(file.path)}
																	</p>
																{/if}
																<div
																	class="mt-3 flex flex-wrap items-center gap-2"
																>
																	<Button
																		size="sm"
																		onclick={() =>
																			void saveSelectionComment(
																				file.path,
																				"queue",
																			)}
																		disabled={queueingCommentByPath[
																			file.path
																		] === true ||
																			submittingCommentByPath[file.path] ===
																				true}
																		variant="outline"
																	>
																		{queueingCommentByPath[file.path] === true
																			? "Queueing…"
																			: "Queue"}
																	</Button>
																	<Button
																		size="sm"
																		onclick={() =>
																			void saveSelectionComment(
																				file.path,
																				"submit",
																			)}
																		disabled={queueingCommentByPath[
																			file.path
																		] === true ||
																			submittingCommentByPath[file.path] ===
																				true}
																	>
																		{submittingCommentByPath[file.path] === true
																			? "Submitting…"
																			: "Submit"}
																	</Button>
																	<Button
																		variant="ghost"
																		size="sm"
																		onclick={() =>
																			resetSelectionComment(file.path)}
																		disabled={queueingCommentByPath[
																			file.path
																		] === true ||
																			submittingCommentByPath[file.path] ===
																				true}
																	>
																		Clear selection
																	</Button>
																</div>
															</div>
														{/if}
													{/if}
												</div>
											{/if}
										{/if}
									</div>
								{/if}
							</section>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</DockWindowChrome>
