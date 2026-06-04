<script lang="ts">
	import { api } from "$lib/api-client";
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
	} from "$lib/components/app/parts/FilesPanel.svelte";
	import ServicePanel from "$lib/components/app/parts/ServicePanel.svelte";
	import TerminalPanel from "$lib/components/app/parts/TerminalPanel.svelte";
	import VSCodePanel from "$lib/components/app/parts/VSCodePanel.svelte";
	import {
		acceptFileConflict,
		addThreadPendingComment,
		bindServiceLocalhost,
		closeFile,
		collapseFileTree,
		discardFileBuffer,
		expandFileTree,
		forceSaveFile,
		getFileEditorModel,
		getFileEditorViewState,
		openFile,
		openService,
		refreshFiles,
		renameFile,
		removeFile,
		saveFile,
		setFileDiffTarget,
		setFileEditorModel,
		setFileEditorViewState,
		toggleFileDirectory,
		toggleFilesChangedOnly,
		updateFileBuffer,
		startService,
		stopService,
		submitThread,
		subscribeServiceOutput,
		unbindServiceLocalhost,
	} from "$lib/context/commands/app-view";
	import { useContext } from "$lib/context/context.svelte";
	import { writeStorage } from "$lib/local-storage";
	import type { DiffStyle } from "$lib/pierre-diff";
	import type { SessionFilesData } from "$lib/context/context.types";
	import { requestVSCodeOpenFile } from "$lib/editor-control";
	import { renderServiceOutputText } from "$lib/service-output";
	import {
		buildUserMessageParts,
		formatConversationComments,
	} from "$lib/session/domains/session-domain.helpers";
	import type { SessionActiveView } from "$lib/session/session-view.types";
	import type { SessionViewState } from "$lib/session/view/create-session-view-state.svelte";
	import {
		DESKTOP_SERVICE_ID,
		VSCODE_SERVICE_ID,
	} from "$lib/session/service-ids";

	const APPROVAL_STORAGE_KEY = "discobot.ui.diff-review.approved";
	const DIFF_STYLE_STORAGE_KEY = "discobot.ui.diff-review.style";

	type DockPanelKind = Exclude<SessionActiveView["kind"], "chat">;
	type RenderedServiceOutputEvent = ServiceOutputEvent & {
		displayText: string;
	};
	type Props = {
		sessionId: string;
		threadId: string;
		sessionView: SessionViewState;
	};

	let { sessionId, threadId, sessionView }: Props = $props();
	const context = useContext();
	const emptyFileData: SessionFilesData = {
		list: [],
		searchable: [],
		diff: [],
		diffStats: {
			additions: 0,
			deletions: 0,
			filesChanged: 0,
		},
		diffTarget: "working",
		contents: {},
		tree: [],
		status: "idle",
		error: null,
	};
	const emptyFileView: FilesPanelView = {
		activePath: "",
		openPaths: [],
		showChangedOnly: false,
		expandedPaths: [],
		loadingPaths: {},
		buffers: {},
	};
	const serviceData = $derived(context.data.services.bySessionId[sessionId]);
	const fileData = $derived(
		context.data.files.bySessionId[sessionId] ?? emptyFileData,
	);
	const fileView = $derived(
		context.view.sessions[sessionId]?.files ?? emptyFileView,
	);
	const filePanelActions: FilesPanelActions = {
		acceptConflict: (path) => acceptFileConflict(sessionId, path),
		close: () => sessionView.openChat(),
		closeFile: (path) => closeFile(sessionId, path),
		collapseTree: () => collapseFileTree(sessionId),
		discardBuffer: (path) => discardFileBuffer(sessionId, path),
		expandTree: () => expandFileTree(sessionId),
		forceSaveFile: (path) => forceSaveFile(sessionId, path),
		getEditorModel: (path) => getFileEditorModel(sessionId, path),
		getEditorViewState: (path) => getFileEditorViewState(sessionId, path),
		openFile: (path) => openFile(sessionId, path),
		refreshFiles: () => refreshFiles(sessionId),
		removeFile: (path) => removeFile(sessionId, path),
		renameFile: (path, nextName) => renameFile(sessionId, path, nextName),
		saveFile: (path) => saveFile(sessionId, path),
		setEditorModel: (path, model) => setFileEditorModel(sessionId, path, model),
		setEditorViewState: (path, viewState) =>
			setFileEditorViewState(sessionId, path, viewState),
		toggleChangedOnly: () => toggleFilesChangedOnly(sessionId),
		toggleDirectory: (path) => toggleFileDirectory(sessionId, path),
		updateBuffer: (path, content) => updateFileBuffer(sessionId, path, content),
		toggleDockMaximized: () => sessionView.toggleDockMaximized(),
	};
	const visibleServices = $derived.by(() =>
		(serviceData?.items ?? []).filter(
			(service) =>
				service.id !== DESKTOP_SERVICE_ID && service.id !== VSCODE_SERVICE_ID,
		),
	);
	const activeService = $derived.by(
		() =>
			visibleServices.find(
				(service) => service.id === sessionView.activeServiceId,
			) ??
			visibleServices[0] ??
			null,
	);
	const desktopAvailable = $derived.by(() =>
		(serviceData?.items ?? []).some(
			(service) => service.id === DESKTOP_SERVICE_ID,
		),
	);
	const vscodeAvailable = $derived.by(() =>
		(serviceData?.items ?? []).some(
			(service) => service.id === VSCODE_SERVICE_ID,
		),
	);
	const vscodeService = $derived.by(
		() =>
			(serviceData?.items ?? []).find(
				(service) => service.id === VSCODE_SERVICE_ID,
			) ?? null,
	);
	const sessionFileContents = $derived.by(() =>
		Object.fromEntries(
			Object.entries(fileData?.contents ?? {}).map(([path, record]) => [
				path,
				fileView.buffers[path]?.content ?? record.content,
			]),
		),
	);
	const sessionFileDiff = $derived.by(() => fileData?.diff ?? []);
	const sessionFileDiffStats = $derived.by(
		() =>
			fileData?.diffStats ?? {
				additions: 0,
				deletions: 0,
				filesChanged: 0,
			},
	);
	const shiftWindowControlsForSidebar = $derived.by(
		() =>
			!context.view.app.navigation.desktopSidebarOpen &&
			sessionView.dockMaximized,
	);
	const preferences = $derived(context.view.app.preferences);
	const activeDockPanelKind = $derived.by<DockPanelKind | null>(() => {
		const { kind } = sessionView.activeView;
		return kind === "chat" ? null : kind;
	});
	let mountedDockPanelKinds = $state<DockPanelKind[]>([]);
	let serviceLogEvents = $state<RenderedServiceOutputEvent[]>([]);
	let serviceLogsConnected = $state(false);
	let diffReviewApprovals = $state<Record<string, Record<string, string>>>({});
	let diffReviewStyle = $state<DiffStyle>("unified");

	diffReviewApprovals = readDiffReviewApprovals();
	diffReviewStyle = readDiffReviewStyle();

	$effect(() => {
		const activeKind = activeDockPanelKind;
		if (!activeKind || mountedDockPanelKinds.includes(activeKind)) {
			return;
		}

		mountedDockPanelKinds = [...mountedDockPanelKinds, activeKind];
	});

	$effect(() => {
		writeStorage(APPROVAL_STORAGE_KEY, JSON.stringify(diffReviewApprovals));
	});

	$effect(() => {
		writeStorage(DIFF_STYLE_STORAGE_KEY, diffReviewStyle);
	});

	function readDiffReviewApprovals(): Record<string, Record<string, string>> {
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

	function readDiffReviewStyle(): DiffStyle {
		if (typeof window === "undefined") {
			return "unified";
		}
		const stored = window.localStorage.getItem(DIFF_STYLE_STORAGE_KEY);
		return stored === "split" ? "split" : "unified";
	}

	function setDiffReviewApprovals(
		nextApprovals: Record<string, Record<string, string>>,
	) {
		diffReviewApprovals = nextApprovals;
	}

	function setDiffReviewStyle(nextStyle: DiffStyle) {
		diffReviewStyle = nextStyle;
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
		const subscription = subscribeServiceOutput({
			sessionId: sessionId,
			serviceId: service.id,
			onOpen: () => {
				serviceLogsConnected = true;
			},
			onError: () => {
				serviceLogsConnected = false;
			},
		});

		const handleMessage = (event: MessageEvent<string>) => {
			if (event.data === "[DONE]") {
				serviceLogsConnected = false;
				return;
			}

			try {
				const parsed = JSON.parse(event.data) as ServiceOutputEvent;
				serviceLogEvents = [...serviceLogEvents, getRenderedLogEvent(parsed)];
			} catch (error) {
				console.error("Failed to parse service output event:", error);
			}
		};

		subscription.eventSource.addEventListener("message", handleMessage);

		return () => {
			subscription.eventSource.removeEventListener("message", handleMessage);
			subscription.unsubscribe();
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
		addThreadPendingComment(sessionId, threadId, {
			snippet: buildDiffSelectionSnippet(payload),
			comment: payload.comment,
		});
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
		await submitThread(sessionId, threadId, {
			parts: buildUserMessageParts(text),
		});
	}

	async function handleOpenDiffFile(path: string) {
		if (!vscodeAvailable) {
			await openFile(sessionId, path);
			return;
		}

		try {
			await requestVSCodeOpenFile(sessionId, path);
			sessionView.openVSCode();
		} catch (error) {
			console.error("Failed to request editor open file", error);
			await openFile(sessionId, path);
		}
	}
</script>

<div class="h-full overflow-auto bg-background px-3 pb-3 pt-0">
	{#if mountedDockPanelKinds.includes("terminal")}
		<div
			class={sessionView.activeView.kind === "terminal" ? "contents" : "hidden"}
		>
			<TerminalPanel
				onClose={sessionView.openChat}
				{sessionId}
				rootEnabled={sessionView.terminalRootEnabled}
				onRootEnabledChange={sessionView.setTerminalRootEnabled}
				dockMaximized={sessionView.dockMaximized}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("desktop")}
		<div
			class={sessionView.activeView.kind === "desktop" ? "contents" : "hidden"}
		>
			<DesktopPanel
				{sessionId}
				{desktopAvailable}
				onClose={sessionView.openChat}
				dockMaximized={sessionView.dockMaximized}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("vscode")}
		<div
			class={sessionView.activeView.kind === "vscode" ? "contents" : "hidden"}
		>
			<VSCodePanel
				dockMaximized={sessionView.dockMaximized}
				onClose={sessionView.openChat}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				resolvedTheme={preferences.resolvedTheme}
				{sessionId}
				service={vscodeService}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("file")}
		<div class={sessionView.activeView.kind === "file" ? "contents" : "hidden"}>
			<FilesPanel
				{fileData}
				{fileView}
				actions={filePanelActions}
				dockMaximized={sessionView.dockMaximized}
				colorScheme={preferences.colorScheme}
				resolvedTheme={preferences.resolvedTheme}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("diff-review")}
		<div
			class={sessionView.activeView.kind === "diff-review"
				? "contents"
				: "hidden"}
		>
			<DiffReviewPanel
				dockMaximized={sessionView.dockMaximized}
				onClose={sessionView.openChat}
				onDiffTargetChange={(target) => setFileDiffTarget(sessionId, target)}
				onLoadDiff={loadDiffReviewEntry}
				onReadFile={readDiffReviewFile}
				onApprovalStateChange={setDiffReviewApprovals}
				onDiffStyleChange={setDiffReviewStyle}
				onOpenFile={handleOpenDiffFile}
				onRefresh={() => refreshFiles(sessionId)}
				onQueueSelectionComment={handleQueueDiffSelectionComment}
				onSubmitSelectionComment={handleSubmitDiffSelectionComment}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				{sessionId}
				diff={sessionFileDiff}
				diffTarget={fileData?.diffTarget ?? "HEAD"}
				fileContents={sessionFileContents}
				diffStats={sessionFileDiffStats}
				approvedBySession={diffReviewApprovals}
				diffStyle={diffReviewStyle}
				resolvedTheme={preferences.resolvedTheme}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if visibleServices.length > 0 && mountedDockPanelKinds.includes("services")}
		<div
			class={sessionView.activeView.kind === "services" ? "contents" : "hidden"}
		>
			<ServicePanel
				dockMaximized={sessionView.dockMaximized}
				{sessionId}
				logEvents={serviceLogEvents}
				logsConnected={serviceLogsConnected}
				services={visibleServices}
				activeServiceId={sessionView.activeServiceId}
				requestedViewMode={sessionView.activeServiceViewMode}
				onSelectService={(serviceId) => openService(sessionId, serviceId)}
				onClose={sessionView.openChat}
				onStart={(serviceId) => startService(sessionId, serviceId)}
				onStop={(serviceId) => stopService(sessionId, serviceId)}
				onBindLocalhost={(serviceId, port) =>
					bindServiceLocalhost(sessionId, serviceId, port)}
				onUnbindLocalhost={(serviceId) =>
					unbindServiceLocalhost(sessionId, serviceId)}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}
</div>
