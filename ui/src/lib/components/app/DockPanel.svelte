<script lang="ts">
	import { api } from "$lib/api-client";
	import type {
		ServiceOutputEvent,
		SessionSingleFileDiffResponse,
	} from "$lib/api-types";
	import DesktopPanel from "$lib/components/app/parts/DesktopPanel.svelte";
	import DiffReviewPanel from "$lib/components/app/parts/DiffReviewPanel.svelte";
	import FilesPanel from "$lib/components/app/parts/FilesPanel.svelte";
	import ServicePanel from "$lib/components/app/parts/ServicePanel.svelte";
	import TerminalPanel from "$lib/components/app/parts/TerminalPanel.svelte";
	import VSCodePanel from "$lib/components/app/parts/VSCodePanel.svelte";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import { useThreadContext } from "$lib/context/thread-context.svelte";
	import { writeStorage } from "$lib/local-storage";
	import type { DiffStyle } from "$lib/pierre-diff";
	import { requestVSCodeOpenFile } from "$lib/editor-control";
	import { renderServiceOutputText } from "$lib/service-output";
	import {
		buildUserMessageParts,
		formatConversationComments,
	} from "$lib/session/domains/session-domain.helpers";
	import type { SessionActiveView } from "$lib/session/session-view.types";
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

	const app = useAppContext();
	const session = useSessionContext();
	const thread = useThreadContext();
	const sessionView = session.ui;
	const visibleServices = $derived.by(() =>
		session.services.list.filter(
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
		session.services.list.some((service) => service.id === DESKTOP_SERVICE_ID),
	);
	const vscodeAvailable = $derived.by(() =>
		session.services.list.some((service) => service.id === VSCODE_SERVICE_ID),
	);
	const vscodeService = $derived.by(
		() =>
			session.services.list.find(
				(service) => service.id === VSCODE_SERVICE_ID,
			) ?? null,
	);
	const sessionFileContents = $derived.by(() => session.files.contents);
	const sessionFileDiff = $derived.by(() => session.files.diff);
	const sessionFileDiffStats = $derived.by(() => session.files.diffStats);
	const shiftWindowControlsForSidebar = $derived.by(
		() => !app.ui.desktopSidebarOpen && sessionView.dockMaximized,
	);
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
		const subscription = app.chatStreams.subscribeServiceOutput({
			sessionId: session.sessionId,
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
		thread.addPendingComment({
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
		await thread.submit({
			parts: buildUserMessageParts(text),
		});
	}

	async function handleOpenDiffFile(path: string) {
		if (!vscodeAvailable) {
			await session.files.open(path);
			return;
		}

		try {
			await requestVSCodeOpenFile(session.sessionId, path);
			sessionView.openVSCode();
		} catch (error) {
			console.error("Failed to request editor open file", error);
			await session.files.open(path);
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
				sessionId={session.sessionId}
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
				sessionId={session.sessionId}
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
				resolvedTheme={app.preferences.resolvedTheme}
				sessionId={session.sessionId}
				service={vscodeService}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}

	{#if mountedDockPanelKinds.includes("file")}
		<div class={sessionView.activeView.kind === "file" ? "contents" : "hidden"}>
			<FilesPanel
				files={session.files}
				onClose={sessionView.openChat}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				dockMaximized={sessionView.dockMaximized}
				colorScheme={app.preferences.colorScheme}
				resolvedTheme={app.preferences.resolvedTheme}
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
				onDiffTargetChange={session.files.setDiffTarget}
				onLoadDiff={loadDiffReviewEntry}
				onReadFile={readDiffReviewFile}
				onApprovalStateChange={setDiffReviewApprovals}
				onDiffStyleChange={setDiffReviewStyle}
				onOpenFile={handleOpenDiffFile}
				onRefresh={() => session.files.refresh()}
				onQueueSelectionComment={handleQueueDiffSelectionComment}
				onSubmitSelectionComment={handleSubmitDiffSelectionComment}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				sessionId={session.sessionId}
				diff={sessionFileDiff}
				diffTarget={session.files.diffTarget}
				fileContents={sessionFileContents}
				diffStats={sessionFileDiffStats}
				approvedBySession={diffReviewApprovals}
				diffStyle={diffReviewStyle}
				resolvedTheme={app.preferences.resolvedTheme}
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
				sessionId={session.sessionId}
				logEvents={serviceLogEvents}
				logsConnected={serviceLogsConnected}
				services={visibleServices}
				activeServiceId={sessionView.activeServiceId}
				requestedViewMode={sessionView.activeServiceViewMode}
				onSelectService={session.services.open}
				onClose={sessionView.openChat}
				onStart={session.services.start}
				onStop={session.services.stop}
				onBindLocalhost={session.services.bindLocalhost}
				onUnbindLocalhost={session.services.unbindLocalhost}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				{shiftWindowControlsForSidebar}
			/>
		</div>
	{/if}
</div>
