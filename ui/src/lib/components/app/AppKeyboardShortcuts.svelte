<script lang="ts">
	import KeyboardShortcutHelpDialog from "$lib/components/app/parts/KeyboardShortcutHelpDialog.svelte";
	import RecentThreadSwitcherDialog from "$lib/components/app/parts/RecentThreadSwitcherDialog.svelte";
	import {
		getAvailableSwitcherThreads,
		getThreadSwitcherThreads,
		recentThreadKey,
	} from "$lib/app/thread-switcher";
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

		const selectedIndex = tabSwitcherOpen ? getTabSwitcherSelectedIndex() : 0;
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

	function toggleKeyboardHelp() {
		const nextOpen = !keyboardHelpOpen;
		closeTabSwitcher();
		keyboardHelpOpen = nextOpen;
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (event.key === "Escape" && (tabSwitcherOpen || keyboardHelpOpen)) {
			event.preventDefault();
			closeOverlays();
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
/>

<KeyboardShortcutHelpDialog
	open={keyboardHelpOpen}
	shortcuts={globalShortcuts as GlobalShortcut[]}
/>
