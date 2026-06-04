<script lang="ts">
	import KeyboardShortcutHelpDialog from "$lib/components/app/parts/KeyboardShortcutHelpDialog.svelte";
	import RecentThreadSwitcherDialog from "$lib/components/app/RecentThreadSwitcherDialog.svelte";
	import {
		getAvailableSwitcherThreads,
		getThreadSwitcherThreads,
		recentThreadKey,
	} from "$lib/app/thread-switcher";
	import {
		getGlobalShortcuts,
		type GlobalShortcut,
		matchGlobalShortcutKeydown,
		shouldCommitTabSwitcherOnKeyup,
	} from "$lib/app/global-shortcuts";
	import {
		closeKeyboardShortcutOverlays,
		createThread,
		openThread,
		setKeyboardShortcutsOpen,
		setMobileSidebarOpen,
		setRecentThreadSwitcherCommitModifier,
		setRecentThreadSwitcherOpen,
		setRecentThreadSwitcherSelectedKey,
		startNewSession,
		toggleKeyboardShortcutsOpen,
		toggleSelectedSessionView,
	} from "$lib/context/commands/app-view";
	import { useContext } from "$lib/context/context.svelte";

	const context = useContext();
	const appEnvironment = context.view.app.environment;
	const globalShortcuts = $derived.by(() =>
		getGlobalShortcuts(appEnvironment.isMacPlatform),
	);
	const keyboardShortcutsDialog = context.view.app.dialogs.keyboardShortcuts;
	const recentThreadSwitcherDialog =
		context.view.app.dialogs.recentThreadSwitcher;
	const selectedThreadKey = $derived.by(() => {
		const sessionId = context.view.app.selection.sessionId;
		if (!sessionId) {
			return null;
		}
		return recentThreadKey(
			sessionId,
			context.view.app.selection.threadId ?? sessionId,
		);
	});
	const availableSwitcherThreads = $derived.by(() =>
		getAvailableSwitcherThreads({
			sessions: context.data.sessions.items,
			recentThreads: context.data.sessions.recentThreads,
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

	function getTabSwitcherSelectedIndex() {
		if (!switcherThreads.length) {
			return -1;
		}
		if (!recentThreadSwitcherDialog.selectedKey) {
			return 0;
		}
		return switcherThreads.findIndex(
			(thread) =>
				recentThreadKey(thread.sessionId, thread.threadId) ===
				recentThreadSwitcherDialog.selectedKey,
		);
	}

	function advanceTabSwitcherSelection(reverse = false) {
		if (!switcherThreads.length) {
			closeTabSwitcher();
			return;
		}

		const selectedIndex = recentThreadSwitcherDialog.open
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
		setRecentThreadSwitcherSelectedKey(
			nextThread
				? recentThreadKey(nextThread.sessionId, nextThread.threadId)
				: null,
		);
		setRecentThreadSwitcherOpen(nextThread !== undefined);
	}

	function closeTabSwitcher() {
		setRecentThreadSwitcherOpen(false);
		setRecentThreadSwitcherSelectedKey(null);
		setRecentThreadSwitcherCommitModifier(null);
	}

	function closeOverlays() {
		closeKeyboardShortcutOverlays();
	}

	function commitTabSwitcherSelection() {
		if (!recentThreadSwitcherDialog.open) {
			return;
		}

		const selectedThread =
			switcherThreads[Math.max(0, getTabSwitcherSelectedIndex())];
		closeTabSwitcher();
		if (!selectedThread) {
			return;
		}
		openThread(selectedThread.sessionId, selectedThread.threadId);
	}

	function handleTabSwitcherHover(sessionId: string, threadId: string) {
		setRecentThreadSwitcherSelectedKey(recentThreadKey(sessionId, threadId));
	}

	function handleTabSwitcherSelect(sessionId: string, threadId: string) {
		closeTabSwitcher();
		openThread(sessionId, threadId);
	}

	function handleStartNewSessionShortcut() {
		closeOverlays();
		startNewSession();
		if (appEnvironment.isMobile) {
			setMobileSidebarOpen(false);
		}
	}

	function handleStartNewThreadShortcut() {
		const sessionId = context.view.app.selection.sessionId;
		if (!sessionId) {
			return;
		}

		closeOverlays();
		void createThread(sessionId);
		if (appEnvironment.isMobile) {
			setMobileSidebarOpen(false);
		}
	}

	function toggleKeyboardHelp() {
		closeTabSwitcher();
		toggleKeyboardShortcutsOpen();
	}

	function handleToggleSelectedSessionView(
		viewKind:
			| "terminal"
			| "desktop"
			| "vscode"
			| "file"
			| "diff-review"
			| "services",
	) {
		closeOverlays();
		toggleSelectedSessionView(viewKind);
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
		if (
			event.key === "Escape" &&
			(recentThreadSwitcherDialog.open || keyboardShortcutsDialog.open)
		) {
			event.preventDefault();
			closeOverlays();
			return;
		}

		const shortcutAction = matchGlobalShortcutKeydown(
			event,
			appEnvironment.isMacPlatform,
		);
		if (!shortcutAction && isEditableShortcutTarget(event.target)) {
			return;
		}
		if (!shortcutAction) {
			return;
		}

		if (shortcutAction.id === "switch-recent-thread") {
			event.preventDefault();
			setKeyboardShortcutsOpen(false);
			setRecentThreadSwitcherCommitModifier(shortcutAction.commitModifier);
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
			handleToggleSelectedSessionView("terminal");
			return;
		}
		if (shortcutAction.id === "toggle-desktop") {
			handleToggleSelectedSessionView("desktop");
			return;
		}
		if (shortcutAction.id === "toggle-editor") {
			handleToggleSelectedSessionView("vscode");
			return;
		}
		if (shortcutAction.id === "toggle-files") {
			handleToggleSelectedSessionView("file");
			return;
		}
		if (shortcutAction.id === "toggle-diff-review") {
			handleToggleSelectedSessionView("diff-review");
			return;
		}
		if (shortcutAction.id === "toggle-services") {
			handleToggleSelectedSessionView("services");
			return;
		}
		toggleKeyboardHelp();
	}

	function handleWindowKeyup(event: KeyboardEvent) {
		if (
			recentThreadSwitcherDialog.open &&
			shouldCommitTabSwitcherOnKeyup(
				event,
				recentThreadSwitcherDialog.commitModifier,
			)
		) {
			event.preventDefault();
			commitTabSwitcherSelection();
		}
	}

	$effect(() => {
		if (recentThreadSwitcherDialog.open && switcherThreads.length === 0) {
			closeTabSwitcher();
			return;
		}
		if (
			recentThreadSwitcherDialog.open &&
			recentThreadSwitcherDialog.selectedKey &&
			!switcherThreads.some(
				(thread) =>
					recentThreadKey(thread.sessionId, thread.threadId) ===
					recentThreadSwitcherDialog.selectedKey,
			)
		) {
			const firstThread = switcherThreads[0];
			setRecentThreadSwitcherSelectedKey(
				firstThread
					? recentThreadKey(firstThread.sessionId, firstThread.threadId)
					: null,
			);
		}
	});
</script>

<svelte:window
	onkeydown={handleWindowKeydown}
	onkeyup={handleWindowKeyup}
	onblur={closeOverlays}
/>

<RecentThreadSwitcherDialog
	open={recentThreadSwitcherDialog.open}
	threads={switcherThreads}
	selectedKey={recentThreadSwitcherDialog.selectedKey}
	helpText={switcherHelpText}
	onHover={handleTabSwitcherHover}
	onSelect={handleTabSwitcherSelect}
	onClose={closeTabSwitcher}
/>

<KeyboardShortcutHelpDialog
	open={keyboardShortcutsDialog.open}
	shortcuts={globalShortcuts as GlobalShortcut[]}
/>
