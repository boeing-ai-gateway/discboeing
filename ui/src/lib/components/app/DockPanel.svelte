<script lang="ts">
	import { FileConflictError, api } from "$lib/api-client";
	import type {
		ServiceOutputEvent,
		SessionSingleFileDiffResponse,
	} from "$lib/api-types";
	import DesktopPanel from "$lib/components/app/parts/DesktopPanel.svelte";
	import DiffReviewPanel from "$lib/components/app/parts/DiffReviewPanel.svelte";
	import FilesPanel from "$lib/components/app/parts/FilesPanel.svelte";
	import type {
		FilesPanelActions,
		FilesPanelView,
		SessionFilesData,
		SessionFileTreeNode,
	} from "$lib/components/app/parts/FilesPanel.svelte";
	import ServicePanel from "$lib/components/app/parts/ServicePanel.svelte";
	import TerminalPanel from "$lib/components/app/parts/TerminalPanel.svelte";
	import VSCodePanel from "$lib/components/app/parts/VSCodePanel.svelte";
	import { useContext } from "$lib/context";
	import { getProjectEventSocket } from "$lib/project-events";
	import { connectServiceOutput } from "$lib/context/service-output-subscription";
	import type { ResolvedTheme } from "$lib/theme";
	import type { DiffStyle } from "$lib/pierre-diff";
	import { requestVSCodeOpenFile } from "$lib/editor-control";
	import { renderServiceOutputText } from "$lib/service-output";
	import {
		buildUserMessageParts,
		formatConversationComments,
	} from "$lib/conversation-helpers";
	import { DESKTOP_SERVICE_ID, VSCODE_SERVICE_ID } from "$lib/service-ids";

	type SessionActiveViewKind =
		| "chat"
		| "terminal"
		| "desktop"
		| "vscode"
		| "diff-review"
		| "file"
		| "services";
	type SessionActiveView =
		| { kind: Exclude<SessionActiveViewKind, "file"> }
		| { kind: "file"; path: string };
	type DockPanelKind = Exclude<SessionActiveViewKind, "chat">;
	type RenderedServiceOutputEvent = ServiceOutputEvent & {
		displayText: string;
	};
	type Props = {
		sessionId: string;
		threadId: string;
	};

	let { sessionId, threadId }: Props = $props();
	const context = useContext();
	const emptyFileData: SessionFilesData = {
		diff: [],
		diffStats: {
			additions: 0,
			deletions: 0,
			filesChanged: 0,
		},
		diffTarget: "",
		tree: [],
	};
	const emptyFileView: FilesPanelView = {
		activePath: "",
		openPaths: [],
		showChangedOnly: false,
		expandedPaths: [],
		loadingPaths: {},
		buffers: {},
		diffTarget: "",
		diffFilesByTarget: {},
	};
	const sessionRecord = $derived(context.data.sessions.byId[sessionId] ?? null);
	const sessionView = $derived(context.view.sessions[sessionId]);
	const fileView = $derived(sessionView?.files ?? emptyFileView);
	const activeView = $derived.by<SessionActiveView>(() => {
		const activeWorkspaceView = sessionView?.workspace.activeView;
		if (activeWorkspaceView === "file") {
			return { kind: "file", path: sessionView?.files.activePath ?? "" };
		}
		if (activeWorkspaceView === "conversation" || !activeWorkspaceView) {
			return { kind: "chat" };
		}
		return {
			kind: activeWorkspaceView as Exclude<SessionActiveViewKind, "file">,
		};
	});
	const dockMaximized = $derived(sessionView?.workspace.dockMaximized ?? false);
	const terminalRootEnabled = $derived(
		sessionView?.workspace.terminalRootEnabled ?? false,
	);
	const activeServiceViewMode = $derived(
		sessionView?.services.activeViewMode ?? "preview",
	);
	const serviceItems = $derived.by(() =>
		(sessionRecord?.services.allIds ?? [])
			.map((id) => sessionRecord?.services.byId[id])
			.filter((service) => Boolean(service))
			.map((service) => ({
				...service,
				label: service.name,
				target:
					service.http !== undefined
						? `http:${service.http}`
						: service.https !== undefined
							? `https:${service.https}`
							: service.path,
			})),
	);
	const fileData = $derived.by<SessionFilesData>(() => {
		if (!sessionRecord) return emptyFileData;
		const diffFiles =
			fileView.diffTarget === ""
				? sessionRecord.diff.files
				: fileView.diffFilesByTarget[fileView.diffTarget];
		const diff = diffFiles?.files ?? [];
		const statusByPath = Object.fromEntries(
			diff.map((entry) => [entry.path, entry.status]),
		);
		const tree = buildFileTree("", statusByPath);
		return {
			diff,
			diffStats: diffFiles?.stats ?? emptyFileData.diffStats,
			diffTarget: fileView.diffTarget,
			tree: fileView.showChangedOnly ? filterChangedTree(tree) : tree,
		};
	});
	const filePanelActions: FilesPanelActions = {
		acceptConflict: (path) => acceptFileConflict(path),
		close: () => openChat(),
		closeFile: (path) => closeFile(path),
		collapseTree: () => collapseFileTree(),
		discardBuffer: (path) => discardFileBuffer(path),
		expandTree: () => expandFileTree(),
		forceSaveFile: (path) => forceSaveFile(path),
		getEditorModel: (path) => sessionView?.files.editorModels[path] ?? null,
		getEditorViewState: (path) =>
			sessionView?.files.editorViewStates[path] ?? null,
		openFile: (path) => openFile(path),
		refreshFiles: () =>
			context.commands.files.refreshFileSubtree(sessionId, "", { wait: true }),
		deleteFile: (path) => deleteFile(path),
		renameFile: (path, nextName) => renameFile(path, nextName),
		saveFile: (path) => saveFile(path),
		setEditorModel: (path, model) => {
			if (sessionView) sessionView.files.editorModels[path] = model;
		},
		setEditorViewState: (path, viewState) => {
			if (sessionView) sessionView.files.editorViewStates[path] = viewState;
		},
		toggleChangedOnly: () => toggleFilesChangedOnly(),
		toggleDirectory: (path) => toggleFileDirectory(path),
		updateBuffer: (path, content) => updateFileBuffer(path, content),
		toggleDockMaximized: () => toggleDockMaximized(),
	};
	const visibleServices = $derived.by(() =>
		serviceItems.filter(
			(service) =>
				service.id !== DESKTOP_SERVICE_ID && service.id !== VSCODE_SERVICE_ID,
		),
	);
	const activeServiceId = $derived.by(() => {
		const selected =
			sessionView?.workspace.activeServiceId ??
			sessionView?.services.activeServiceId;
		return selected &&
			visibleServices.some((service) => service.id === selected)
			? selected
			: (visibleServices[0]?.id ?? null);
	});
	const activeService = $derived.by(
		() =>
			visibleServices.find((service) => service.id === activeServiceId) ?? null,
	);
	const desktopAvailable = $derived.by(() =>
		serviceItems.some((service) => service.id === DESKTOP_SERVICE_ID),
	);
	const vscodeAvailable = $derived.by(() =>
		serviceItems.some((service) => service.id === VSCODE_SERVICE_ID),
	);
	const vscodeService = $derived.by(
		() =>
			serviceItems.find((service) => service.id === VSCODE_SERVICE_ID) ?? null,
	);
	const sessionFileContents = $derived.by(() =>
		Object.fromEntries(
			Object.entries(fileView.buffers).map(([path, buffer]) => [
				path,
				buffer.content,
			]),
		),
	);
	const sessionFileDiff = $derived.by(() => fileData.diff);
	const sessionFileDiffStats = $derived.by(() => fileData.diffStats);
	const shiftWindowControlsForSidebar = $derived.by(
		() => !context.view.navigation.desktopSidebarOpen && dockMaximized,
	);
	const preferences = $derived(context.view.app.preferences);
	const resolvedTheme = $derived(preferences.resolvedTheme as ResolvedTheme);
	const activeDockPanelKind = $derived.by<DockPanelKind | null>(() => {
		const { kind } = activeView;
		return kind === "chat" ? null : kind;
	});
	let mountedDockPanelKinds = $state<DockPanelKind[]>([]);
	let serviceLogEvents = $state<RenderedServiceOutputEvent[]>([]);
	let serviceLogsConnected = $state(false);
	const diffReviewApprovals = $derived(context.view.app.diffReview.approvals);
	const diffReviewStyle = $derived(context.view.app.diffReview.style);

	$effect(() => {
		const activeKind = activeDockPanelKind;
		if (!activeKind || mountedDockPanelKinds.includes(activeKind)) {
			return;
		}

		mountedDockPanelKinds = [...mountedDockPanelKinds, activeKind];
	});

	function openWorkspaceView(kind: string) {
		if (sessionView) sessionView.workspace.activeView = kind;
	}

	function openChat() {
		if (!sessionView) return;
		sessionView.workspace.dockMaximized = false;
		sessionView.workspace.activeView = "conversation";
	}

	function openVSCode() {
		openWorkspaceView("vscode");
	}

	function openService(serviceId: string, viewMode?: "preview" | "logs") {
		if (!sessionView) return;
		sessionView.workspace.activeServiceId = serviceId;
		sessionView.services.activeServiceId = serviceId;
		if (viewMode) {
			sessionView.services.activeViewMode = viewMode;
		}
		sessionView.workspace.activeView = "services";
	}

	function setTerminalRootEnabled(value: boolean) {
		if (sessionView) sessionView.workspace.terminalRootEnabled = value;
	}

	function toggleDockMaximized() {
		if (sessionView) {
			sessionView.workspace.dockMaximized =
				!sessionView.workspace.dockMaximized;
		}
	}

	function buildFileTree(
		path: string,
		statusByPath: Record<string, SessionFileTreeNode["status"]>,
	): SessionFileTreeNode[] {
		const node = sessionRecord?.files.nodesByPath[path];
		return (node?.childrenPaths ?? []).map((childPath) => {
			const child = sessionRecord?.files.nodesByPath[childPath];
			const name = childPath.split("/").at(-1) ?? childPath;
			const type = child?.entry?.type ?? "directory";
			const children =
				type === "directory"
					? buildFileTree(childPath, statusByPath)
					: undefined;
			const status = statusByPath[childPath];
			return {
				name,
				path: childPath,
				type,
				size: child?.entry?.size,
				status,
				changed: Boolean(status || children?.some((entry) => entry.changed)),
				children,
			};
		});
	}

	function filterChangedTree(
		nodes: SessionFileTreeNode[],
	): SessionFileTreeNode[] {
		return nodes
			.map((node) => ({
				...node,
				children: node.children ? filterChangedTree(node.children) : undefined,
			}))
			.filter((node) => node.changed || Boolean(node.children?.length));
	}

	function closeFile(path: string) {
		fileView.openPaths = fileView.openPaths.filter(
			(openPath) => openPath !== path,
		);
		if (fileView.activePath === path) {
			fileView.activePath = fileView.openPaths.at(-1) ?? "";
		}
	}

	function collapseFileTree() {
		fileView.expandedPaths = [];
	}

	async function expandFileTree() {
		const directories = Object.values(sessionRecord?.files.nodesByPath ?? {})
			.filter((node) => node.entry?.type === "directory")
			.map((node) => node.path);
		fileView.expandedPaths = Array.from(new Set(["", ...directories]));
	}

	function discardFileBuffer(path: string) {
		const buffer = fileView.buffers[path];
		if (!buffer) return;
		buffer.content = buffer.originalContent;
		buffer.isDirty = false;
		buffer.saveError = null;
		buffer.hasConflict = false;
		buffer.conflictContent = null;
	}

	function acceptFileConflict(path: string) {
		const buffer = fileView.buffers[path];
		if (!buffer?.conflictContent) return;
		buffer.content = buffer.conflictContent;
		buffer.originalContent = buffer.conflictContent;
		buffer.isDirty = false;
		buffer.saveError = null;
		buffer.hasConflict = false;
		buffer.conflictContent = null;
	}

	async function forceSaveFile(path: string): Promise<boolean> {
		return saveFile(path, { force: true });
	}

	async function openFile(path?: string): Promise<void> {
		if (!path) return;
		fileView.activePath = path;
		if (!fileView.openPaths.includes(path)) {
			fileView.openPaths = [...fileView.openPaths, path];
		}
		if (fileView.buffers[path]) return;
		fileView.loadingPaths[path] = true;
		try {
			const fromBase =
				fileData.diff.find((entry) => entry.path === path)?.status ===
				"deleted";
			const record = await api.readSessionFile(sessionId, path, { fromBase });
			fileView.buffers[path] = {
				content: record.content,
				originalContent: record.content,
				encoding: record.encoding,
				isDirty: false,
				isSaving: false,
				saveError: null,
				hasConflict: false,
				conflictContent: null,
				fromBase,
			};
		} finally {
			fileView.loadingPaths[path] = false;
		}
	}

	async function saveFile(
		path: string,
		options: { force?: boolean } = {},
	): Promise<boolean> {
		const buffer = fileView.buffers[path];
		if (!buffer || buffer.encoding !== "utf8") return false;
		buffer.isSaving = true;
		buffer.saveError = null;
		try {
			await context.commands.files.saveFile(sessionId, path, buffer.content, {
				wait: true,
				encoding: buffer.encoding,
				originalContent: options.force ? undefined : buffer.originalContent,
			});
			buffer.originalContent = buffer.content;
			buffer.isDirty = false;
			buffer.hasConflict = false;
			buffer.conflictContent = null;
			return true;
		} catch (error) {
			if (error instanceof FileConflictError) {
				buffer.hasConflict = true;
				buffer.conflictContent = error.currentContent;
			}
			buffer.saveError =
				error instanceof Error ? error.message : "Failed to save file";
			return false;
		} finally {
			buffer.isSaving = false;
		}
	}

	async function renameFile(path: string, nextName: string): Promise<boolean> {
		const parent = path.split("/").slice(0, -1).join("/");
		const nextPath = parent ? `${parent}/${nextName}` : nextName;
		try {
			await context.commands.files.renameFile(sessionId, path, nextPath, {
				wait: true,
			});
			if (fileView.buffers[path]) {
				fileView.buffers[nextPath] = fileView.buffers[path];
				delete fileView.buffers[path];
			}
			fileView.openPaths = fileView.openPaths.map((openPath) =>
				openPath === path ? nextPath : openPath,
			);
			if (fileView.activePath === path) fileView.activePath = nextPath;
			return true;
		} catch (error) {
			console.error("Failed to rename file", error);
			return false;
		}
	}

	async function deleteFile(path: string): Promise<boolean> {
		try {
			await context.commands.files.deleteFile(sessionId, path, { wait: true });
			delete fileView.buffers[path];
			closeFile(path);
			return true;
		} catch (error) {
			console.error("Failed to delete file", error);
			return false;
		}
	}

	async function toggleFilesChangedOnly(): Promise<void> {
		fileView.showChangedOnly = !fileView.showChangedOnly;
	}

	async function toggleFileDirectory(path: string): Promise<void> {
		if (fileView.expandedPaths.includes(path)) {
			fileView.expandedPaths = fileView.expandedPaths.filter(
				(entry) => entry !== path,
			);
			return;
		}
		fileView.expandedPaths = [...fileView.expandedPaths, path];
		if (!sessionRecord?.files.nodesByPath[path]?.childrenPaths) {
			fileView.loadingPaths[path] = true;
			try {
				await context.commands.files.refreshFileSubtree(sessionId, path, {
					wait: true,
				});
			} finally {
				fileView.loadingPaths[path] = false;
			}
		}
	}

	function updateFileBuffer(path: string, content: string) {
		const buffer = fileView.buffers[path];
		if (!buffer) return;
		buffer.content = content;
		buffer.isDirty = content !== buffer.originalContent;
		buffer.saveError = null;
	}

	function setDiffReviewApprovals(
		nextApprovals: Record<string, Record<string, string>>,
	) {
		void context.commands.preferences.setDiffReviewApprovals(nextApprovals);
	}

	function setDiffReviewStyle(nextStyle: DiffStyle) {
		void context.commands.preferences.setDiffReviewStyle(nextStyle);
	}

	async function loadDiffReviewEntry(
		sessionId: string,
		params: { path: string; target: string },
	): Promise<SessionSingleFileDiffResponse> {
		return (await api.getSessionDiff(
			sessionId,
			params,
		)) as SessionSingleFileDiffResponse;
	}

	function readDiffReviewFile(
		sessionId: string,
		path: string,
		options?: { fromBase?: boolean },
	) {
		return api.readSessionFile(sessionId, path, options);
	}

	function getRenderedLogEvent(
		event: ServiceOutputEvent,
	): RenderedServiceOutputEvent {
		return {
			...event,
			displayText:
				typeof event.data === "string"
					? renderServiceOutputText(event.data)
					: event.type === "exit"
						? `Process exited with code ${event.exitCode ?? "unknown"}`
						: (event.error ?? ""),
		};
	}

	$effect(() => {
		const service = activeService;
		if (
			!mountedDockPanelKinds.includes("services") ||
			service?.passive ||
			typeof window === "undefined" ||
			!service
		) {
			serviceLogEvents = [];
			serviceLogsConnected = false;
			return;
		}

		void service.status;
		serviceLogEvents = [];
		serviceLogsConnected = false;
		const socket = getProjectEventSocket(context);
		if (!socket) {
			return;
		}

		const subscription = connectServiceOutput({
			socket,
			sessionId,
			serviceId: service.id,
			onOpen: () => {
				serviceLogsConnected = true;
			},
			onClose: () => {
				serviceLogsConnected = false;
			},
			onError: () => {
				serviceLogsConnected = false;
			},
			onMessage: (data) => {
				if (data === "[DONE]") {
					serviceLogsConnected = false;
					return;
				}

				try {
					const parsed = JSON.parse(data) as ServiceOutputEvent;
					serviceLogEvents = [...serviceLogEvents, getRenderedLogEvent(parsed)];
				} catch (error) {
					console.error("Failed to parse service output event:", error);
				}
			},
		});

		void subscription.open().catch(() => {
			serviceLogsConnected = false;
		});

		return () => {
			subscription.close();
			serviceLogsConnected = false;
		};
	});

	function buildDiffSelectionSnippet({
		path,
		selectedText,
	}: {
		path: string;
		selectedText: string;
	}) {
		return `Diff excerpt from \`${path}\`:
\`\`\`diff
${selectedText}
\`\`\``;
	}

	function handleQueueDiffSelectionComment(payload: {
		path: string;
		selectedText: string;
		comment: string;
	}) {
		void context.commands.threadComposer.addThreadPendingComment(
			sessionId,
			threadId,
			{
				snippet: buildDiffSelectionSnippet(payload),
				comment: payload.comment,
			},
		);
	}

	async function handleSubmitDiffSelectionComment(payload: {
		path: string;
		selectedText: string;
		comment: string;
	}) {
		const text = formatConversationComments([
			{
				snippet: buildDiffSelectionSnippet(payload),
				comment: payload.comment,
			},
		]);
		await context.commands.threads.sendMessage(sessionId, threadId, {
			messages: [
				{
					id: crypto.randomUUID(),
					role: "user",
					parts: buildUserMessageParts(text),
				},
			],
		});
	}

	function handleDiffTargetChange(target: string) {
		return context.commands.files.setDiffTarget(sessionId, target);
	}

	function refreshDiffReview() {
		if (fileView.diffTarget) {
			return context.commands.files.setDiffTarget(
				sessionId,
				fileView.diffTarget,
			);
		}
		return context.commands.files.refreshFileSubtree(sessionId, "");
	}

	async function handleOpenDiffFile(path: string) {
		if (!vscodeAvailable) {
			await openFile(path);
			return;
		}

		try {
			await requestVSCodeOpenFile(sessionId, path);
			openVSCode();
		} catch (error) {
			console.error("Failed to request editor open file", error);
			await openFile(path);
		}
	}
</script>

<div class="h-full overflow-auto bg-background px-3 pb-3 pt-0">
	{#if mountedDockPanelKinds.includes("terminal")}
		<div class={activeView.kind === "terminal" ? "contents" : "hidden"}>
			<TerminalPanel
				onClose={openChat}
				{sessionId}
				rootEnabled={terminalRootEnabled}
				onRootEnabledChange={setTerminalRootEnabled}
				{dockMaximized}
				onToggleDockMaximized={toggleDockMaximized}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("desktop")}
		<div class={activeView.kind === "desktop" ? "contents" : "hidden"}>
			<DesktopPanel
				{sessionId}
				{desktopAvailable}
				onClose={openChat}
				{dockMaximized}
				onToggleDockMaximized={toggleDockMaximized}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("vscode")}
		<div class={activeView.kind === "vscode" ? "contents" : "hidden"}>
			<VSCodePanel
				{dockMaximized}
				onClose={openChat}
				onToggleDockMaximized={toggleDockMaximized}
				{resolvedTheme}
				{sessionId}
				service={vscodeService}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("file")}
		<div class={activeView.kind === "file" ? "contents" : "hidden"}>
			<FilesPanel
				{fileData}
				{fileView}
				actions={filePanelActions}
				{dockMaximized}
				colorScheme={preferences.colorScheme}
				{resolvedTheme}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("diff-review")}
		<div class={activeView.kind === "diff-review" ? "contents" : "hidden"}>
			<DiffReviewPanel
				{dockMaximized}
				onClose={openChat}
				onDiffTargetChange={handleDiffTargetChange}
				onLoadDiff={loadDiffReviewEntry}
				onReadFile={readDiffReviewFile}
				onApprovalStateChange={setDiffReviewApprovals}
				onDiffStyleChange={setDiffReviewStyle}
				onOpenFile={handleOpenDiffFile}
				onRefresh={refreshDiffReview}
				onQueueSelectionComment={handleQueueDiffSelectionComment}
				onSubmitSelectionComment={handleSubmitDiffSelectionComment}
				onToggleDockMaximized={toggleDockMaximized}
				{sessionId}
				diff={sessionFileDiff}
				diffTarget={fileData?.diffTarget ?? "HEAD"}
				fileContents={sessionFileContents}
				diffStats={sessionFileDiffStats}
				approvedBySession={diffReviewApprovals}
				diffStyle={diffReviewStyle}
				{resolvedTheme}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if visibleServices.length > 0 && mountedDockPanelKinds.includes("services")}
		<div class={activeView.kind === "services" ? "contents" : "hidden"}>
			<ServicePanel
				{dockMaximized}
				{sessionId}
				logEvents={serviceLogEvents}
				logsConnected={serviceLogsConnected}
				services={visibleServices}
				{activeServiceId}
				requestedViewMode={activeServiceViewMode}
				onSelectService={(serviceId) => openService(serviceId)}
				onClose={openChat}
				onStart={(serviceId) =>
					context.commands.services.startService(sessionId, serviceId)}
				onStop={(serviceId) =>
					context.commands.services.stopService(sessionId, serviceId)}
				onBindLocalhost={(serviceId, port) =>
					context.commands.services.bindServiceLocalhost(
						sessionId,
						serviceId,
						port,
						{
							wait: true,
						},
					)}
				onUnbindLocalhost={(serviceId) =>
					context.commands.services.unbindServiceLocalhost(
						sessionId,
						serviceId,
						{
							wait: true,
						},
					)}
				onToggleDockMaximized={toggleDockMaximized}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}
</div>
