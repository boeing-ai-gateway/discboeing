<script lang="ts">
	import DesktopPanel from "$lib/components/app/parts/DesktopPanel.svelte";
	import DiffReviewPanel from "$lib/components/app/parts/DiffReviewPanel.svelte";
	import FilesPanel from "$lib/components/app/parts/FilesPanel.svelte";
	import ServicePanel from "$lib/components/app/parts/ServicePanel.svelte";
	import TerminalPanel from "$lib/components/app/parts/TerminalPanel.svelte";
	import VSCodePanel from "$lib/components/app/parts/VSCodePanel.svelte";
	import { buildUserMessageParts } from "$lib/session/domains/session-domain.helpers";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { useSessionContext } from "$lib/context/session-context.svelte";
	import { useThreadContext } from "$lib/context/thread-context.svelte";
	import type { SessionActiveView } from "$lib/session/session-view.types";
	import { DESKTOP_SERVICE_ID, VSCODE_SERVICE_ID } from "$lib/shell-types";

	type DockPanelKind = Exclude<SessionActiveView["kind"], "chat">;

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
	const desktopAvailable = $derived.by(() =>
		session.services.list.some((service) => service.id === DESKTOP_SERVICE_ID),
	);
	const editorEnabled = $derived.by(() => app.preferences.showEditorButton);
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
	const activeDockPanelKind = $derived.by<DockPanelKind | null>(() => {
		const { kind } = sessionView.activeView;
		return kind === "chat" ? null : kind;
	});
	let mountedDockPanelKinds = $state<DockPanelKind[]>([]);

	$effect(() => {
		const activeKind = activeDockPanelKind;
		if (!activeKind || mountedDockPanelKinds.includes(activeKind)) {
			return;
		}

		mountedDockPanelKinds = [...mountedDockPanelKinds, activeKind];
	});

	$effect(() => {
		if (!editorEnabled && sessionView.activeView.kind === "vscode") {
			sessionView.openChat();
		}
	});

	function buildDiffSelectionPrompt({
		path,
		selectedText,
		comment,
	}: {
		path: string;
		selectedText: string;
		comment: string;
	}) {
		return `Please help with this selected diff excerpt from \`${path}\`.

Comment:
${comment}

Selected diff text:
\`\`\`diff
${selectedText}
\`\`\``;
	}

	async function handleSubmitDiffSelectionComment(payload: {
		path: string;
		selectedText: string;
		comment: string;
	}) {
		const prompt = buildDiffSelectionPrompt(payload);
		await thread.submit({
			parts: buildUserMessageParts(prompt, []),
		});
	}

	function handleOpenDiffFile(path: string) {
		void session.files.open(path);
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
			/>
		</div>
	{/if}

	{#if editorEnabled && mountedDockPanelKinds.includes("vscode")}
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
				onOpenFile={handleOpenDiffFile}
				onRefresh={() => session.files.refresh()}
				onSubmitSelectionComment={handleSubmitDiffSelectionComment}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
				sessionId={session.sessionId}
				diff={sessionFileDiff}
				fileContents={sessionFileContents}
				diffStats={sessionFileDiffStats}
				resolvedTheme={app.preferences.resolvedTheme}
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
				streamManager={app.chatStreams}
				services={visibleServices}
				activeServiceId={sessionView.activeServiceId}
				onSelectService={session.services.open}
				onClose={sessionView.openChat}
				onStart={session.services.start}
				onStop={session.services.stop}
				onToggleDockMaximized={sessionView.toggleDockMaximized}
			/>
		</div>
	{/if}
</div>
