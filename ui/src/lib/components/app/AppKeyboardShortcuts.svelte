<script lang="ts">
	import KeyboardShortcutHelpDialog from "$lib/components/app/parts/KeyboardShortcutHelpDialog.svelte";
	import RecentThreadSwitcherDialog from "$lib/components/app/RecentThreadSwitcherDialog.svelte";
	import {
		getAvailableSwitcherThreads,
		getThreadSwitcherThreads,
		recentThreadKey,
	} from "$lib/app/thread-switcher";
	import {
		DESKTOP_SERVICE_ID,
		VSCODE_SERVICE_ID,
	} from "$lib/session/service-ids";
	import {
		detectIsMacPlatform,
		getGlobalShortcuts,
		type GlobalShortcut,
		matchGlobalShortcutKeydown,
		shouldCommitTabSwitcherOnKeyup,
		type SwitcherCommitModifier,
	} from "$lib/app/global-shortcuts";
	import { useAppContext } from "$lib/context/app-context.svelte";
	import { IsMobile } from "$lib/hooks/is-mobile.svelte.js";

	const app = useAppContext();
	const isMobile = new IsMobile(1024);
	const isMacPlatform = $derived.by(() => detectIsMacPlatform());
	const globalShortcuts = $derived.by(() => getGlobalShortcuts(isMacPlatform));
	const selectedThreadKey = $derived.by(() => {
		const sessionId = app.sessions.selectedId;
		if (!sessionId) {
			return null;
		}
		const sessionContext = app.sessions.sessionContexts.get(sessionId);
		return recentThreadKey(
			sessionId,
			sessionContext?.threads.selectedId ?? sessionId,
		);
	});
	const availableSwitcherThreads = $derived.by(() =>
		getAvailableSwitcherThreads({
			sessions: app.sessions.sessions,
			recentThreads: app.sessions.recentThreads,
		}),
	);
	const switcherThreads = $derived.by(() =>
		getThreadSwitcherThreads({
			threads: availableSwitcherThreads,
			selectedThreadKey,
		}),
	);
	const switcherHelpText =
		"Hold the shortcut modifier, tap to cycle, release to switch";
	let tabSwitcherOpen = $state(false);
	let tabSwitcherSelectedKey = $state<string | null>(null);
	let tabSwitcherCommitModifier = $state<SwitcherCommitModifier | null>(null);
	let keyboardHelpOpen = $state(false);

	function getTabSwitcherSelectedIndex() {
		if (!switcherThreads.length) {
			return -1;
		}
		if (!tabSwitcherSelectedKey) {
			return 0;
		}
		return switcherThreads.findIndex(
			(thread) =>
				recentThreadKey(thread.sessionId, thread.threadId) ===
				tabSwitcherSelectedKey,
		);
	}

	function advanceTabSwitcherSelection(reverse = false) {
		if (!switcherThreads.length) {
			closeTabSwitcher();
			return;
		}

		const selectedIndex = tabSwitcherOpen
			? getTabSwitcherSelectedIndex()
			: selectedThreadKey
				? 0
				: -1;
		const nextIndex =
			selectedIndex >= 0
				? (selectedIndex + (reverse ? switcherThreads.length - 1 : 1)) %
					switcherThreads.length
				: 0;
		const nextThread = switcherThreads[nextIndex];
		tabSwitcherSelectedKey = nextThread
			? recentThreadKey(nextThread.sessionId, nextThread.threadId)
			: null;
		tabSwitcherOpen = nextThread !== undefined;
	}

	function closeTabSwitcher() {
		tabSwitcherOpen = false;
		tabSwitcherSelectedKey = null;
		tabSwitcherCommitModifier = null;
	}

	function closeOverlays() {
		closeTabSwitcher();
		keyboardHelpOpen = false;
	}

	function commitTabSwitcherSelection() {
		if (!tabSwitcherOpen) {
			return;
		}

		const selectedThread =
			switcherThreads[Math.max(0, getTabSwitcherSelectedIndex())];
		closeTabSwitcher();
		if (!selectedThread) {
			return;
		}
		app.sessions.openThread(selectedThread.sessionId, selectedThread.threadId);
	}

	function handleTabSwitcherHover(sessionId: string, threadId: string) {
		tabSwitcherSelectedKey = recentThreadKey(sessionId, threadId);
	}

	function handleTabSwitcherSelect(sessionId: string, threadId: string) {
		closeTabSwitcher();
		app.sessions.openThread(sessionId, threadId);
	}

	function handleStartNewSessionShortcut() {
		closeOverlays();
		app.sessions.startNew();
		if (isMobile.current) {
			app.ui.setMobileSidebarOpen(false);
		}
	}

	function handleStartNewThreadShortcut() {
		const sessionId = app.sessions.selectedId;
		if (!sessionId) {
			return;
		}

		closeOverlays();
		void app.sessions.createThread(sessionId);
		if (isMobile.current) {
			app.ui.setMobileSidebarOpen(false);
		}
	}

	function toggleKeyboardHelp() {
		const nextOpen = !keyboardHelpOpen;
		closeTabSwitcher();
		keyboardHelpOpen = nextOpen;
	}

	function getSelectedSessionContext() {
		const sessionId = app.sessions.selectedId;
		if (!sessionId) {
			return null;
		}
		return app.sessions.sessionContexts.get(sessionId) ?? null;
	}

	function toggleSelectedSessionView(
		viewKind:
			| "terminal"
			| "desktop"
			| "vscode"
			| "file"
			| "diff-review"
			| "services",
	) {
		const sessionContext = getSelectedSessionContext();
		if (!sessionContext) {
			return;
		}

		closeOverlays();
		const sessionView = sessionContext.ui;
		if (sessionView.activeView.kind === viewKind) {
			sessionView.openChat();
			return;
		}

		if (viewKind === "terminal") {
			sessionView.openTerminal();
			return;
		}
		if (viewKind === "desktop") {
			sessionView.openDesktop();
			return;
		}
		if (viewKind === "vscode") {
			if (
				sessionContext.services.list.some(
					(service) => service.id === VSCODE_SERVICE_ID,
				)
			) {
				sessionView.openVSCode();
			}
			return;
		}
		if (viewKind === "file") {
			void sessionContext.files.open();
			return;
		}
		if (viewKind === "diff-review") {
			sessionView.openDiffReview();
			return;
		}

		const sessionServices = sessionContext.services.list.filter(
			(service) =>
				service.id !== DESKTOP_SERVICE_ID && service.id !== VSCODE_SERVICE_ID,
		);
		if (sessionServices.length > 0) {
			sessionView.openServices();
		}
	}

	function isEditableShortcutTarget(target: EventTarget | null) {
		if (!(target instanceof HTMLElement)) {
			return false;
		}

		return Boolean(
			target.closest(
				'input, textarea, select, [contenteditable]:not([contenteditable="false"])',
			),
		);
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (event.key === "Escape" && (tabSwitcherOpen || keyboardHelpOpen)) {
			event.preventDefault();
			closeOverlays();
			return;
		}

		if (isEditableShortcutTarget(event.target)) {
			return;
		}

		const shortcutAction = matchGlobalShortcutKeydown(event, isMacPlatform);
		if (!shortcutAction) {
			return;
		}

		if (shortcutAction.id === "switch-recent-thread") {
			event.preventDefault();
			keyboardHelpOpen = false;
			tabSwitcherCommitModifier = shortcutAction.commitModifier;
			advanceTabSwitcherSelection(shortcutAction.reverse);
			return;
		}

		event.preventDefault();
		if (shortcutAction.id === "new-session") {
			handleStartNewSessionShortcut();
			return;
		}
		if (shortcutAction.id === "new-thread") {
			handleStartNewThreadShortcut();
			return;
		}
		if (shortcutAction.id === "toggle-terminal") {
			toggleSelectedSessionView("terminal");
			return;
		}
		if (shortcutAction.id === "toggle-desktop") {
			toggleSelectedSessionView("desktop");
			return;
		}
		if (shortcutAction.id === "toggle-editor") {
			toggleSelectedSessionView("vscode");
			return;
		}
		if (shortcutAction.id === "toggle-files") {
			toggleSelectedSessionView("file");
			return;
		}
		if (shortcutAction.id === "toggle-diff-review") {
			toggleSelectedSessionView("diff-review");
			return;
		}
		if (shortcutAction.id === "toggle-services") {
			toggleSelectedSessionView("services");
			return;
		}
		toggleKeyboardHelp();
	}

	function handleWindowKeyup(event: KeyboardEvent) {
		if (
			tabSwitcherOpen &&
			shouldCommitTabSwitcherOnKeyup(event, tabSwitcherCommitModifier)
		) {
			event.preventDefault();
			commitTabSwitcherSelection();
		}
	}

	$effect(() => {
		if (tabSwitcherOpen && switcherThreads.length === 0) {
			closeTabSwitcher();
			return;
		}
		if (
			tabSwitcherOpen &&
			tabSwitcherSelectedKey &&
			!switcherThreads.some(
				(thread) =>
					recentThreadKey(thread.sessionId, thread.threadId) ===
					tabSwitcherSelectedKey,
			)
		) {
			const firstThread = switcherThreads[0];
			tabSwitcherSelectedKey = firstThread
				? recentThreadKey(firstThread.sessionId, firstThread.threadId)
				: null;
		}
	});
</script>

<svelte:window
	onkeydown={handleWindowKeydown}
	onkeyup={handleWindowKeyup}
	onblur={closeOverlays}
/>

<RecentThreadSwitcherDialog
	open={tabSwitcherOpen}
	threads={switcherThreads}
	selectedKey={tabSwitcherSelectedKey}
	helpText={switcherHelpText}
	onHover={handleTabSwitcherHover}
	onSelect={handleTabSwitcherSelect}
	onClose={closeTabSwitcher}
/>

<KeyboardShortcutHelpDialog
	open={keyboardHelpOpen}
	shortcuts={globalShortcuts as GlobalShortcut[]}
/>
