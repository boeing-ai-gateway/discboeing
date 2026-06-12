<script lang="ts">
	import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
	import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
	import EllipsisIcon from "@lucide/svelte/icons/ellipsis";
	import FolderIcon from "@lucide/svelte/icons/folder";
	import GitBranchIcon from "@lucide/svelte/icons/git-branch";
	import PackageIcon from "@lucide/svelte/icons/package";
	import PanelLeftIcon from "@lucide/svelte/icons/panel-left";
	import PinIcon from "@lucide/svelte/icons/pin";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import { generateId } from "ai";
	import { Switch } from "$lib/components/ui/switch";
	import type { Session, Thread, Workspace } from "$lib/api-types";
	import AppSessionStatus from "$lib/components/app/AppSessionStatus.svelte";
	import AppThreadStatus from "$lib/components/app/AppThreadStatus.svelte";
	import AppSidebarDeleteDialog from "$lib/components/app/parts/AppSidebarDeleteDialog.svelte";
	import AppSidebarRenameDialog from "$lib/components/app/parts/AppSidebarRenameDialog.svelte";
	import * as Collapsible from "$lib/components/ui/collapsible";
	import { Button } from "$lib/components/ui/button";
	import {
		DropdownMenu,
		DropdownMenuContent,
		DropdownMenuItem,
		DropdownMenuTrigger,
	} from "$lib/components/ui/dropdown-menu";
	import { useContext } from "$lib/context";

	type Props = {
		onThreadSelect?: () => void;
		onPinSidebar?: () => void;
		onToggleSidebar?: () => void;
		mode?: "panel" | "dropdown" | "floating";
		collapsed?: boolean;
	};
	let {
		onThreadSelect,
		onPinSidebar,
		onToggleSidebar,
		mode = "panel",
		collapsed = false,
	}: Props = $props();

	const dropdownMode = $derived(mode === "dropdown");
	const floatingMode = $derived(mode === "floating");
	const floatingCollapsed = $derived(floatingMode && collapsed);

	const context = useContext();
	const preferences = $derived(context.view.app.preferences);
	const sessionItems = $derived(
		context.data.sessions.allIds
			.map((sessionId) => context.data.sessions.byId[sessionId]?.value ?? null)
			.filter((session): session is Session => session !== null)
			.sort(compareSessionsByCreatedAtDesc),
	);
	const sessionsById = $derived(
		Object.fromEntries(
			Object.entries(context.data.sessions.byId)
				.map(([sessionId, record]) => [sessionId, record.value] as const)
				.filter((entry): entry is [string, Session] => entry[1] !== null),
		),
	);
	const workspacesById = $derived(context.data.workspaces.byId);
	const selectedSessionId = $derived(context.view.selection.sessionId);
	const selectedThreadId = $derived(context.view.selection.threadId);
	const pendingSessionId = $derived(context.view.selection.pendingSessionId);
	type SidebarSession = (typeof sessionItems)[number];
	type SessionGroup = {
		key: string;
		workspaceId: string | null;
		label: string;
		sourceType: string;
		sessions: SidebarSession[];
	};
	type TaskThreadMetadata = {
		type?: string;
		parentThreadId?: string;
	};

	let renameDialogOpen = $state(false);
	let renameSessionId = $state<string | null>(null);
	let renameDraft = $state("");
	let renamingSession = $state(false);
	let renameThreadDialogOpen = $state(false);
	let renameThreadId = $state<string | null>(null);
	let renameThreadDraft = $state("");
	let renamingThread = $state(false);
	let deleteDialogOpen = $state(false);
	let deleteSessionId = $state<string | null>(null);
	let deletingSession = $state(false);
	let deleteThreadDialogOpen = $state(false);
	let deleteThreadId = $state<string | null>(null);
	let deletingThread = $state(false);
	let renameWorkspaceDialogOpen = $state(false);
	let renameWorkspaceId = $state<string | null>(null);
	let renameWorkspaceDraft = $state("");
	let renamingWorkspace = $state(false);
	let deleteWorkspaceDialogOpen = $state(false);
	let deleteWorkspaceId = $state<string | null>(null);
	let deletingWorkspace = $state(false);
	let floatingOpen = $state(false);
	let shellRef = $state<HTMLElement | null>(null);
	const showSidebarBody = $derived(!floatingCollapsed || floatingOpen);
	const visibleRecentThreads = $derived(
		context.view.app.recentThreads.visibleItems,
	);
	const showRecentThreads = $derived(
		preferences.recentThreadsVisibleLimit > 1 &&
			visibleRecentThreads.length > 0,
	);
	const showAllSessionsHeader = $derived(showRecentThreads);
	const floatingSidebarPortalSelector = [
		'[data-slot="dropdown-menu-content"]',
		'[data-slot="dialog-content"]',
		'[data-slot="alert-dialog-content"]',
	].join(", ");

	function sessionCreatedAtTime(session: Session) {
		const createdAtTime = Date.parse(session.createdAt);
		return Number.isNaN(createdAtTime) ? 0 : createdAtTime;
	}

	function compareSessionsByCreatedAtDesc(
		sessionA: Session,
		sessionB: Session,
	) {
		return sessionCreatedAtTime(sessionB) - sessionCreatedAtTime(sessionA);
	}

	function closeFloatingSidebar() {
		if (!floatingMode) {
			return;
		}
		floatingOpen = false;
	}

	function toggleFloatingSidebar() {
		if (!floatingCollapsed) {
			return;
		}
		floatingOpen = !floatingOpen;
	}

	function shouldKeepFloatingSidebarOpen(target: Node) {
		if (shellRef?.contains(target)) {
			return true;
		}

		return (
			target instanceof Element &&
			target.closest(floatingSidebarPortalSelector) !== null
		);
	}

	$effect(() => {
		if (!floatingCollapsed) {
			floatingOpen = false;
		}
	});

	$effect(() => {
		if (
			!floatingCollapsed ||
			!floatingOpen ||
			typeof document === "undefined"
		) {
			return;
		}

		function handlePointerDown(event: PointerEvent) {
			const target = event.target;
			if (!(target instanceof Node)) {
				return;
			}
			if (shouldKeepFloatingSidebarOpen(target)) {
				return;
			}
			floatingOpen = false;
		}

		document.addEventListener("pointerdown", handlePointerDown);
		return () => {
			document.removeEventListener("pointerdown", handlePointerDown);
		};
	});

	function sessionById(sessionId: string) {
		return sessionsById[sessionId] ?? null;
	}

	function visibleThreadsForSession(sessionId: string): Thread[] {
		const threads = context.data.sessions.byId[sessionId]?.threads;
		if (!threads) {
			return [];
		}
		return threads.allIds
			.map((threadId) => threads.byId[threadId]?.value)
			.filter(
				(thread): thread is Thread => thread !== null && thread !== undefined,
			);
	}

	function threadById(sessionId: string | null, threadId: string) {
		if (!sessionId) {
			return null;
		}
		return (
			context.data.sessions.byId[sessionId]?.threads.byId[threadId]?.value ??
			null
		);
	}

	function sessionHasNestedThreads(sessionId: string) {
		return visibleThreadsForSession(sessionId).length > 1;
	}

	function threadMetadata(threadObj: Thread): TaskThreadMetadata | null {
		const metadata = threadObj.metadata;
		if (!metadata || Array.isArray(metadata)) {
			return null;
		}
		return metadata as TaskThreadMetadata;
	}

	function threadParentId(threadObj: Thread) {
		const parentThreadId = threadMetadata(threadObj)?.parentThreadId;
		return typeof parentThreadId === "string" && parentThreadId.length > 0
			? parentThreadId
			: null;
	}

	function visibleRootThreadsForSession(sessionId: string): Thread[] {
		const threads = visibleThreadsForSession(sessionId);
		const threadIDs = new Set(threads.map((threadObj) => threadObj.id));
		return threads.filter((threadObj) => {
			const parentThreadId = threadParentId(threadObj);
			return !parentThreadId || !threadIDs.has(parentThreadId);
		});
	}

	function visibleChildThreadsForSession(
		sessionId: string,
		parentThreadId: string,
	): Thread[] {
		return visibleThreadsForSession(sessionId).filter(
			(threadObj) => threadParentId(threadObj) === parentThreadId,
		);
	}

	function isSessionThreadSelected(sessionId: string, threadId: string) {
		return selectedSessionId === sessionId && selectedThreadId === threadId;
	}

	function workspaceById(workspaceId: string | null) {
		if (!workspaceId) {
			return null;
		}
		return workspacesById[workspaceId] ?? null;
	}

	function handleSelectSession(sessionId: string) {
		void context.commands.navigation.selectSession(sessionId);
		closeFloatingSidebar();
		onThreadSelect?.();
	}

	function handleSelectRecentThread(sessionId: string, threadId: string) {
		void context.commands.navigation.openThread(sessionId, threadId);
		closeFloatingSidebar();
		onThreadSelect?.();
	}

	function handleStartNewSession() {
		void context.commands.navigation.startNewSession();
		closeFloatingSidebar();
		onThreadSelect?.();
	}

	async function handleCreateThread(sessionId: string) {
		if (!sessionById(sessionId)) {
			return;
		}

		await context.commands.navigation.selectSession(sessionId);
		const createdThreadId = generateId();
		await context.commands.threads.createThread(
			sessionId,
			{ id: createdThreadId },
			{ wait: true },
		);
		await context.commands.navigation.openThread(sessionId, createdThreadId);
		closeFloatingSidebar();
		onThreadSelect?.();
	}

	async function handleStopSession(sessionId: string) {
		await context.commands.sessions.stopSession(sessionId);
	}

	function openRenameDialog(sessionId: string) {
		const sessionItem = sessionById(sessionId);
		if (!sessionItem) {
			return;
		}
		renameSessionId = sessionId;
		renameDraft = sessionItem.name;
		renameDialogOpen = true;
	}

	function openRenameThreadDialog(threadId: string) {
		const threadItem = threadById(selectedSessionId, threadId);
		if (!threadItem) {
			return;
		}
		renameThreadId = threadId;
		renameThreadDraft = threadItem.name;
		renameThreadDialogOpen = true;
	}

	function closeRenameDialog() {
		renameDialogOpen = false;
		renameSessionId = null;
		renameDraft = "";
		renamingSession = false;
	}

	function closeRenameThreadDialog() {
		renameThreadDialogOpen = false;
		renameThreadId = null;
		renameThreadDraft = "";
		renamingThread = false;
	}

	async function handleRenameSession() {
		if (!renameSessionId || renamingSession) {
			return;
		}
		renamingSession = true;
		await context.commands.sessions.renameSession(
			renameSessionId,
			renameDraft,
			{
				wait: true,
			},
		);
		renamingSession = false;
		closeRenameDialog();
	}

	async function handleRenameThread() {
		if (!selectedSessionId || !renameThreadId || renamingThread) {
			return;
		}
		renamingThread = true;
		await context.commands.threads.renameThread(
			selectedSessionId,
			renameThreadId,
			renameThreadDraft,
			{ wait: true },
		);
		renamingThread = false;
		closeRenameThreadDialog();
	}

	function openDeleteDialog(sessionId: string) {
		if (!sessionById(sessionId)) {
			return;
		}
		deleteSessionId = sessionId;
		deleteDialogOpen = true;
	}

	function openDeleteThreadDialog(threadId: string) {
		if (isPrimaryThread(threadId) || !threadById(selectedSessionId, threadId)) {
			return;
		}
		deleteThreadId = threadId;
		deleteThreadDialogOpen = true;
	}

	function closeDeleteDialog() {
		deleteDialogOpen = false;
		deleteSessionId = null;
		deletingSession = false;
	}

	function closeDeleteThreadDialog() {
		deleteThreadDialogOpen = false;
		deleteThreadId = null;
		deletingThread = false;
	}

	async function handleDeleteSession() {
		if (!deleteSessionId || deletingSession) {
			return;
		}
		deletingSession = true;
		await context.commands.sessions.deleteSession(deleteSessionId, {
			wait: true,
		});
		deletingSession = false;
		closeDeleteDialog();
	}

	async function handleDeleteThread() {
		if (!selectedSessionId || !deleteThreadId || deletingThread) {
			return;
		}
		deletingThread = true;
		await context.commands.threads.deleteThread(
			selectedSessionId,
			deleteThreadId,
			{
				wait: true,
			},
		);
		deletingThread = false;
		closeDeleteThreadDialog();
	}

	function deleteDialogSessionName() {
		if (!deleteSessionId) {
			return "this session";
		}
		return sessionById(deleteSessionId)?.name ?? "this session";
	}

	function deleteDialogThreadName() {
		if (!deleteThreadId) {
			return "this thread";
		}
		return threadById(selectedSessionId, deleteThreadId)?.name ?? "this thread";
	}

	function isPrimaryThread(threadId: string) {
		return threadId === selectedSessionId;
	}

	function isRecentThreadSelected(sessionId: string, threadId: string) {
		const activeSessionId = selectedSessionId ?? pendingSessionId;
		const activeThreadId = selectedThreadId ?? activeSessionId;
		return activeSessionId === sessionId && activeThreadId === threadId;
	}

	function shortenHomePath(path: string) {
		return path.replace(/\\/g, "/").replace(/^\/home\/[^/]+/, "~");
	}

	function trimWorkspacePrefix(
		path: string,
		sourceType: Workspace["sourceType"],
	) {
		const normalizedPath = shortenHomePath(path).replace(/\/+$/, "");
		if (sourceType === "git") {
			return normalizedPath
				.replace(/^https?:\/\/github\.com\//i, "")
				.replace(/^ssh:\/\/git@github\.com\//i, "")
				.replace(/^git@github\.com:/i, "")
				.replace(/\.git$/i, "");
		}
		return normalizedPath;
	}

	function workspaceGroupLabel(workspace: Workspace | null) {
		const displayName = workspace?.displayName?.trim();
		if (displayName) {
			return displayName;
		}

		if (!workspace || workspace.sourceType === "managed") {
			return "Unnamed Workspace";
		}

		return trimWorkspacePrefix(workspace.path, workspace.sourceType);
	}

	function openRenameWorkspaceDialog(workspaceId: string) {
		const workspace = workspaceById(workspaceId);
		if (!workspace) {
			return;
		}
		renameWorkspaceId = workspaceId;
		renameWorkspaceDraft = workspace.displayName ?? "";
		renameWorkspaceDialogOpen = true;
	}

	function closeRenameWorkspaceDialog() {
		renameWorkspaceDialogOpen = false;
		renameWorkspaceId = null;
		renameWorkspaceDraft = "";
		renamingWorkspace = false;
	}

	async function handleRenameWorkspace() {
		if (!renameWorkspaceId || renamingWorkspace) {
			return;
		}
		renamingWorkspace = true;
		await context.commands.workspaces.renameWorkspace(
			renameWorkspaceId,
			renameWorkspaceDraft,
		);
		renamingWorkspace = false;
		closeRenameWorkspaceDialog();
	}

	function openDeleteWorkspaceDialog(workspaceId: string) {
		if (!workspaceById(workspaceId)) {
			return;
		}
		deleteWorkspaceId = workspaceId;
		deleteWorkspaceDialogOpen = true;
	}

	function closeDeleteWorkspaceDialog() {
		deleteWorkspaceDialogOpen = false;
		deleteWorkspaceId = null;
		deletingWorkspace = false;
	}

	async function handleDeleteWorkspace() {
		if (!deleteWorkspaceId || deletingWorkspace) {
			return;
		}
		deletingWorkspace = true;
		await context.commands.workspaces.deleteWorkspace(deleteWorkspaceId);
		deletingWorkspace = false;
		closeDeleteWorkspaceDialog();
	}

	function deleteDialogWorkspaceName() {
		const workspace = workspaceById(deleteWorkspaceId);
		if (!workspace) {
			return "this workspace";
		}
		return workspaceGroupLabel(workspace);
	}

	const workspaceSessionGroups = $derived.by(() => {
		const groups: SessionGroup[] = [];

		for (const sessionObj of sessionItems) {
			const workspace = sessionObj.workspaceId
				? workspaceById(sessionObj.workspaceId)
				: null;
			const sourceType = workspace?.sourceType ?? "managed";
			const key =
				workspace?.id ?? `managed:${sessionObj.workspaceId ?? "none"}`;
			const existingGroup = groups.find((group) => group.key === key);

			if (existingGroup) {
				existingGroup.sessions.push(sessionObj);
				continue;
			}

			groups.push({
				key,
				workspaceId: workspace?.id ?? null,
				label: workspaceGroupLabel(workspace),
				sourceType,
				sessions: [sessionObj],
			});
		}

		return groups;
	});
</script>

{#snippet threadItem(sessionId: string, threadObj: Thread, depth: number)}
	<div class="space-y-0.5">
		<div class="group flex min-w-0 items-center gap-0.5">
			<button
				type="button"
				onclick={() => handleSelectRecentThread(sessionId, threadObj.id)}
				class={`flex min-h-8 min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1 text-left text-sm transition-colors ${isSessionThreadSelected(sessionId, threadObj.id) ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-inner" : "text-sidebar-foreground/75 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"}`}
			>
				<AppThreadStatus {sessionId} threadId={threadObj.id} class="shrink-0" />
				<span class="min-w-0 flex-1 truncate"
					>{threadObj.name || "New Thread"}</span
				>
			</button>

			<DropdownMenu>
				<DropdownMenuTrigger>
					<Button
						variant="ghost"
						size="icon-xs"
						class={`h-8 w-7 rounded-md transition-colors ${isSessionThreadSelected(sessionId, threadObj.id) ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-inner hover:bg-sidebar-accent hover:text-sidebar-accent-foreground" : "text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"}`}
						aria-label={`Thread actions for ${threadObj.name || "New Thread"}`}
						onclick={(event) => event.stopPropagation()}
					>
						<EllipsisIcon
							class="size-3 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
						/>
					</Button>
				</DropdownMenuTrigger>
				<DropdownMenuContent align="end" class="w-32">
					<DropdownMenuItem
						onclick={() => openRenameThreadDialog(threadObj.id)}
					>
						Rename
					</DropdownMenuItem>
					{#if !isPrimaryThread(threadObj.id)}
						<DropdownMenuItem
							variant="destructive"
							onclick={() => openDeleteThreadDialog(threadObj.id)}
						>
							Delete
						</DropdownMenuItem>
					{/if}
				</DropdownMenuContent>
			</DropdownMenu>
		</div>

		{#if visibleChildThreadsForSession(sessionId, threadObj.id).length > 0}
			<div
				class="space-y-0.5 border-l border-sidebar-border/70 pl-2"
				style={`margin-left: ${Math.max(depth, 0) + 1}rem;`}
			>
				{#each visibleChildThreadsForSession(sessionId, threadObj.id) as childThreadObj (childThreadObj.id)}
					{@render threadItem(sessionId, childThreadObj, depth + 1)}
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

{#snippet sessionItem(sessionObj: SidebarSession, isSelected: boolean)}
	<div class="space-y-0.5">
		<div class="group flex min-w-0 items-center gap-0.5">
			<button
				type="button"
				onclick={() => handleSelectSession(sessionObj.id)}
				class={`flex h-8 min-w-0 flex-1 items-center gap-2 rounded-md px-2 text-sm font-medium transition-colors ${isSelected ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-inner" : "text-sidebar-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"}`}
			>
				<AppSessionStatus
					sessionId={sessionObj.id}
					showLabel={false}
					class="shrink-0"
				/>
				<span class={floatingMode ? "whitespace-nowrap" : "truncate"}
					>{sessionObj.displayName || sessionObj.name || "New Session"}</span
				>
			</button>

			<DropdownMenu>
				<DropdownMenuTrigger>
					<Button
						variant="ghost"
						size="icon-xs"
						class={`h-8 w-7 rounded-md transition-colors ${isSelected ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-inner hover:bg-sidebar-accent hover:text-sidebar-accent-foreground" : "text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"}`}
						aria-label={`Session actions for ${sessionObj.displayName || sessionObj.name || "New Session"}`}
						onclick={(event) => event.stopPropagation()}
					>
						<EllipsisIcon
							class="size-3.5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
						/>
					</Button>
				</DropdownMenuTrigger>
				<DropdownMenuContent align="end" class="w-32">
					<DropdownMenuItem
						onclick={() => void handleCreateThread(sessionObj.id)}
					>
						New thread
					</DropdownMenuItem>
					<DropdownMenuItem onclick={() => openRenameDialog(sessionObj.id)}>
						Rename
					</DropdownMenuItem>
					{#if sessionObj.sandboxStatus !== "stopped"}
						<DropdownMenuItem
							onclick={() => void handleStopSession(sessionObj.id)}
						>
							Stop
						</DropdownMenuItem>
					{/if}
					<DropdownMenuItem
						variant="destructive"
						onclick={() => openDeleteDialog(sessionObj.id)}
					>
						Delete
					</DropdownMenuItem>
				</DropdownMenuContent>
			</DropdownMenu>
		</div>

		{#if sessionHasNestedThreads(sessionObj.id)}
			<div class="ml-5 space-y-0.5 border-l border-sidebar-border pl-2">
				{#each visibleRootThreadsForSession(sessionObj.id) as threadObj (threadObj.id)}
					{@render threadItem(sessionObj.id, threadObj, 0)}
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

{#snippet recentThreadItem(
	threadObj: (typeof visibleRecentThreads)[number],
	isSelected: boolean,
)}
	<button
		type="button"
		onclick={() =>
			handleSelectRecentThread(threadObj.sessionId, threadObj.threadId)}
		class={`flex min-h-8 w-full min-w-0 items-center gap-2 overflow-hidden rounded-md px-2 py-1 text-left text-sm transition-colors ${isSelected ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-inner" : "text-sidebar-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"}`}
	>
		<AppThreadStatus
			sessionId={threadObj.sessionId}
			threadId={threadObj.threadId}
			class="shrink-0"
		/>
		<span
			class={`min-w-0 flex-1 font-medium ${floatingMode ? "whitespace-nowrap" : "truncate"}`}
			>{threadObj.name || "New Thread"}</span
		>
	</button>
{/snippet}

{#snippet workspaceGroup(group: SessionGroup)}
	<div class="space-y-1.5">
		<div
			class="group flex items-center gap-1.5 px-2 pt-1 text-xs font-medium uppercase tracking-[0.16em] text-sidebar-foreground/60"
		>
			{#if group.sourceType === "git"}
				<GitBranchIcon class="size-3 shrink-0" />
			{:else if group.sourceType === "local"}
				<FolderIcon class="size-3 shrink-0" />
			{:else}
				<PackageIcon class="size-3 shrink-0" />
			{/if}
			<span class="min-w-0 flex-1 truncate">{group.label}</span>
			{#if group.workspaceId}
				<DropdownMenu>
					<DropdownMenuTrigger>
						<Button
							variant="ghost"
							size="icon-xs"
							class="h-6 w-6 rounded-md text-sidebar-foreground/60 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
							aria-label={`Workspace actions for ${group.label}`}
							onclick={(event) => event.stopPropagation()}
						>
							<EllipsisIcon
								class="size-3 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
							/>
						</Button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end" class="w-32">
						<DropdownMenuItem
							onclick={() => openRenameWorkspaceDialog(group.workspaceId!)}
						>
							Rename
						</DropdownMenuItem>
						<DropdownMenuItem
							variant="destructive"
							onclick={() => openDeleteWorkspaceDialog(group.workspaceId!)}
						>
							Delete
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			{/if}
		</div>
		<div class="space-y-0.5">
			{#each group.sessions as sessionObj (sessionObj.id)}
				{@render sessionItem(sessionObj, selectedSessionId === sessionObj.id)}
			{/each}
		</div>
	</div>
{/snippet}

<aside
	bind:this={shellRef}
	class={`flex min-h-0 flex-col overflow-hidden text-sidebar-foreground ${dropdownMode ? "max-h-[min(70vh,32rem)] min-w-64 bg-sidebar" : floatingMode ? `${showSidebarBody ? "max-h-[calc(100vh-7rem)] w-fit max-w-[calc(100vw-1.5rem)] rounded-md border border-sidebar-border bg-sidebar shadow-sm" : "w-fit bg-transparent shadow-none border-transparent"} pointer-events-auto` : "h-full min-w-64 w-full rounded-md border border-sidebar-border bg-sidebar shadow-sm"}`}
>
	<div
		class={`flex h-10 items-center justify-between px-3 ${showSidebarBody ? "border-b border-sidebar-border" : ""}`}
	>
		<div class="flex min-w-0 items-center gap-1">
			{#if onToggleSidebar && !dropdownMode}
				<Button
					variant="ghost"
					size="icon-xs"
					onclick={onToggleSidebar}
					aria-label={floatingMode
						? "Expand sessions panel"
						: "Collapse sessions panel"}
					title={floatingMode
						? "Expand sessions panel"
						: "Collapse sessions panel"}
					class="text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
				>
					<PanelLeftIcon class="size-3.5" />
				</Button>
			{/if}
			{#if floatingCollapsed}
				<button
					type="button"
					onclick={toggleFloatingSidebar}
					aria-expanded={floatingOpen}
					class="inline-flex items-center gap-1 rounded-md py-0 pr-0.5 text-xs font-medium uppercase tracking-[0.16em] text-sidebar-foreground/70 transition-colors hover:text-sidebar-accent-foreground"
				>
					<span>Sessions</span>
					<ChevronDownIcon
						class={`size-3 shrink-0 transition-transform ${floatingOpen ? "rotate-180" : ""}`}
					/>
				</button>
			{:else}
				<p
					class="text-xs font-medium uppercase tracking-[0.16em] text-sidebar-foreground/70"
				>
					Sessions
				</p>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			{#if !dropdownMode && !floatingCollapsed && sessionItems.length > 0}
				<label
					class="flex items-center gap-2 text-xs text-sidebar-foreground/70"
				>
					<span>workspace</span>
					<Switch
						checked={preferences.sidebarAllGroupedByWorkspace}
						class="data-[state=checked]:bg-muted data-[state=unchecked]:bg-muted/70 [&_[data-slot=switch-thumb]]:bg-foreground dark:[&_[data-slot=switch-thumb]]:bg-foreground focus-visible:ring-muted-foreground/20"
						onCheckedChange={(checked) =>
							void context.commands.preferences.setSidebarAllGroupedByWorkspace(
								checked === true,
							)}
					/>
				</label>
			{/if}
			{#if dropdownMode && onPinSidebar}
				<Button
					variant="ghost"
					size="icon-xs"
					onclick={onPinSidebar}
					aria-label="Pin sessions sidebar"
					title="Pin sessions sidebar"
					class="text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
				>
					<PinIcon class="size-3.5" />
				</Button>
			{/if}
			<Button
				variant="ghost"
				size="icon-xs"
				onclick={handleStartNewSession}
				aria-label="New session"
				title="New session"
				class="text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
			>
				<PlusIcon class="size-3.5" />
			</Button>
		</div>
	</div>

	{#if showSidebarBody}
		<div class="flex-1 overflow-y-auto p-2">
			<div class="space-y-0.5">
				{#if sessionItems.length === 0}
					<p class="px-2 text-xs text-sidebar-foreground/50">No sessions</p>
				{:else}
					{#if showRecentThreads}
						<Collapsible.Root
							open={preferences.sidebarRecentOpen}
							onOpenChange={(open) =>
								void context.commands.preferences.setSidebarRecentOpen(open)}
						>
							<Collapsible.Trigger
								class="flex w-full items-center gap-1 px-2 pb-1 pt-1 text-xs font-medium uppercase tracking-[0.16em] text-sidebar-foreground/70 transition-colors hover:text-sidebar-accent-foreground"
							>
								{#if preferences.sidebarRecentOpen}
									<ChevronDownIcon class="size-3 shrink-0" />
								{:else}
									<ChevronRightIcon class="size-3 shrink-0" />
								{/if}
								Recent
							</Collapsible.Trigger>
							<Collapsible.Content class="space-y-0.5">
								{#each visibleRecentThreads as threadObj (`${threadObj.sessionId}:${threadObj.threadId}`)}
									{@render recentThreadItem(
										threadObj,
										isRecentThreadSelected(
											threadObj.sessionId,
											threadObj.threadId,
										),
									)}
								{/each}
							</Collapsible.Content>
						</Collapsible.Root>
					{/if}

					{#if sessionItems.length > 0}
						{#if showAllSessionsHeader}
							<Collapsible.Root
								open={preferences.sidebarAllOpen}
								onOpenChange={(open) =>
									void context.commands.preferences.setSidebarAllOpen(open)}
							>
								<Collapsible.Trigger
									class="flex w-full items-center gap-1 px-2 pb-1 pt-2 text-xs font-medium uppercase tracking-[0.16em] text-sidebar-foreground/70 transition-colors hover:text-sidebar-accent-foreground"
								>
									{#if preferences.sidebarAllOpen}
										<ChevronDownIcon class="size-3 shrink-0" />
									{:else}
										<ChevronRightIcon class="size-3 shrink-0" />
									{/if}
									All sessions
								</Collapsible.Trigger>
								<Collapsible.Content
									class={preferences.sidebarAllGroupedByWorkspace
										? "space-y-3"
										: "space-y-0.5"}
								>
									{#if preferences.sidebarAllGroupedByWorkspace}
										{#each workspaceSessionGroups as group (group.key)}
											{@render workspaceGroup(group)}
										{/each}
									{:else}
										{#each sessionItems as sessionObj (sessionObj.id)}
											{@render sessionItem(
												sessionObj,
												selectedSessionId === sessionObj.id,
											)}
										{/each}
									{/if}
								</Collapsible.Content>
							</Collapsible.Root>
						{:else}
							<div
								class={preferences.sidebarAllGroupedByWorkspace
									? "space-y-3 pt-1"
									: "space-y-0.5 pt-1"}
							>
								{#if preferences.sidebarAllGroupedByWorkspace}
									{#each workspaceSessionGroups as group (group.key)}
										{@render workspaceGroup(group)}
									{/each}
								{:else}
									{#each sessionItems as sessionObj (sessionObj.id)}
										{@render sessionItem(
											sessionObj,
											selectedSessionId === sessionObj.id,
										)}
									{/each}
								{/if}
							</div>
						{/if}
					{/if}
				{/if}
			</div>
		</div>
	{/if}

	<AppSidebarRenameDialog
		bind:open={renameDialogOpen}
		title="Rename session"
		description="Choose a new name for this session."
		label="Session name"
		value={renameDraft}
		placeholder="Session name"
		saving={renamingSession}
		saveDisabled={renameDraft.trim().length === 0}
		onValueChange={(value) => {
			renameDraft = value;
		}}
		onCancel={closeRenameDialog}
		onSave={() => {
			void handleRenameSession();
		}}
	/>

	<AppSidebarRenameDialog
		bind:open={renameThreadDialogOpen}
		title="Rename thread"
		description="Choose a new name for this thread."
		label="Thread name"
		value={renameThreadDraft}
		placeholder="Thread name"
		saving={renamingThread}
		saveDisabled={renameThreadDraft.trim().length === 0}
		onValueChange={(value) => {
			renameThreadDraft = value;
		}}
		onCancel={closeRenameThreadDialog}
		onSave={() => {
			void handleRenameThread();
		}}
	/>

	<AppSidebarRenameDialog
		bind:open={renameWorkspaceDialogOpen}
		title="Rename workspace"
		description="Choose a new name for this workspace."
		label="Workspace name"
		value={renameWorkspaceDraft}
		placeholder="Workspace name"
		saving={renamingWorkspace}
		onValueChange={(value) => {
			renameWorkspaceDraft = value;
		}}
		onCancel={closeRenameWorkspaceDialog}
		onSave={() => {
			void handleRenameWorkspace();
		}}
	/>

	<AppSidebarDeleteDialog
		bind:open={deleteDialogOpen}
		title="Delete session?"
		description={`This will permanently delete ${deleteDialogSessionName()}.`}
		deleting={deletingSession}
		onCancel={closeDeleteDialog}
		onDelete={() => {
			void handleDeleteSession();
		}}
	/>

	<AppSidebarDeleteDialog
		bind:open={deleteThreadDialogOpen}
		title="Delete thread?"
		description={`Delete "${deleteDialogThreadName()}"? This action cannot be undone.`}
		deleting={deletingThread}
		onCancel={closeDeleteThreadDialog}
		onDelete={() => {
			void handleDeleteThread();
		}}
	/>

	<AppSidebarDeleteDialog
		bind:open={deleteWorkspaceDialogOpen}
		title="Delete workspace?"
		description={`Delete "${deleteDialogWorkspaceName()}"? This action cannot be undone.`}
		deleting={deletingWorkspace}
		onCancel={closeDeleteWorkspaceDialog}
		onDelete={() => {
			void handleDeleteWorkspace();
		}}
	/>
</aside>
