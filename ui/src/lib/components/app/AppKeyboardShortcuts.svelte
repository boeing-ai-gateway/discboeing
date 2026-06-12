<script lang="ts">
	import KeyboardShortcutHelpDialog from "$lib/components/app/parts/KeyboardShortcutHelpDialog.svelte";
	import GlobalFindBar from "$lib/components/app/parts/GlobalFindBar.svelte";
	import RecentThreadSwitcherDialog from "$lib/components/app/RecentThreadSwitcherDialog.svelte";
	import {
		getAvailableSwitcherThreads,
		getThreadSwitcherThreads,
		recentThreadKey,
	} from "$lib/context/view/thread-switcher";
	import {
		getGlobalShortcuts,
		type GlobalShortcut,
		matchGlobalShortcutKeydown,
		shouldCommitTabSwitcherOnKeyup,
	} from "$lib/shortcuts/global-shortcuts";
	import type { Session } from "$lib/api-types";
	import {
		findInPage,
		onFindInPageResult,
		stopFindInPage,
		supportsFindInPage,
	} from "$lib/desktop/electron-adapter";
	import { useContext } from "$lib/context";
	import { generateId } from "ai";

	const context = useContext();
	const appEnvironment = context.view.app.environment;
	const globalShortcuts = $derived.by(() =>
		getGlobalShortcuts(appEnvironment.isMacPlatform),
	);
	const keyboardShortcutsDialog = context.view.app.dialogs.keyboardShortcuts;
	const recentThreadSwitcherDialog =
		context.view.app.dialogs.recentThreadSwitcher;
	const selectedThreadKey = $derived.by(() => {
		const sessionId = context.view.selection.sessionId;
		if (!sessionId) {
			return null;
		}
		return recentThreadKey(
			sessionId,
			context.view.selection.threadId ?? sessionId,
		);
	});
	const sessionItems = $derived.by(() =>
		context.data.sessions.allIds
			.map((sessionId) => context.data.sessions.byId[sessionId]?.value ?? null)
			.filter((session): session is Session => session !== null),
	);
	const availableSwitcherThreads = $derived.by(() =>
		getAvailableSwitcherThreads({
			sessions: sessionItems,
			recentThreads: context.view.app.recentThreads.visibleItems,
		}),
	);
	const switcherThreads = $derived.by(() =>
		getThreadSwitcherThreads({
			threads: availableSwitcherThreads,
			selectedThreadKey,
		}),
	);
	const switcherOpen = $derived.by(
		() => recentThreadSwitcherDialog.open && switcherThreads.length > 0,
	);
	const effectiveSwitcherSelectedKey = $derived.by(() => {
		const firstThread = switcherThreads[0];
		const firstKey = firstThread
			? recentThreadKey(firstThread.sessionId, firstThread.threadId)
			: null;
		const selectedKey = recentThreadSwitcherDialog.selectedKey;
		if (!selectedKey) {
			return firstKey;
		}
		return switcherThreads.some(
			(thread) =>
				recentThreadKey(thread.sessionId, thread.threadId) === selectedKey,
		)
			? selectedKey
			: firstKey;
	});
	const switcherHelpText =
		"Hold the shortcut modifier, tap to cycle, release to switch";
	let findOpen = $state(false);
	let findQuery = $state("");
	let findActiveMatch = $state(0);
	let findMatchCount = $state(0);
	let findRequestToken = 0;
	let latestFindRequestId = $state<number | null>(null);

	function getTabSwitcherSelectedIndex() {
		if (!switcherThreads.length) {
			return -1;
		}
		if (!effectiveSwitcherSelectedKey) {
			return 0;
		}
		return switcherThreads.findIndex(
			(thread) =>
				recentThreadKey(thread.sessionId, thread.threadId) ===
				effectiveSwitcherSelectedKey,
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
		void context.commands.dialogs.setRecentThreadSwitcherSelectedKey(
			nextThread
				? recentThreadKey(nextThread.sessionId, nextThread.threadId)
				: null,
		);
		void context.commands.dialogs.setRecentThreadSwitcherOpen(
			nextThread !== undefined,
		);
	}

	function closeTabSwitcher() {
		void context.commands.dialogs.setRecentThreadSwitcherOpen(false);
		void context.commands.dialogs.setRecentThreadSwitcherSelectedKey(null);
		void context.commands.dialogs.setRecentThreadSwitcherCommitModifier(null);
	}

	function closeOverlays() {
		void context.commands.dialogs.closeKeyboardShortcutOverlays();
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
		void context.commands.navigation.openThread(
			selectedThread.sessionId,
			selectedThread.threadId,
		);
	}

	function handleTabSwitcherHover(sessionId: string, threadId: string) {
		void context.commands.dialogs.setRecentThreadSwitcherSelectedKey(
			recentThreadKey(sessionId, threadId),
		);
	}

	function handleTabSwitcherSelect(sessionId: string, threadId: string) {
		closeTabSwitcher();
		void context.commands.navigation.openThread(sessionId, threadId);
	}

	function handleStartNewSessionShortcut() {
		closeOverlays();
		void context.commands.navigation.startNewSession();
		if (appEnvironment.isMobile) {
			void context.commands.navigation.setMobileSidebarOpen(false);
		}
	}

	function handleStartNewThreadShortcut() {
		const sessionId = context.view.selection.sessionId;
		if (!sessionId) {
			return;
		}

		closeOverlays();
		const threadId = generateId();
		void context.commands.threads
			.createThread(sessionId, { id: threadId }, { wait: true })
			.then(() => context.commands.navigation.openThread(sessionId, threadId));
		if (appEnvironment.isMobile) {
			void context.commands.navigation.setMobileSidebarOpen(false);
		}
	}

	function toggleKeyboardHelp() {
		closeTabSwitcher();
		void context.commands.dialogs.toggleKeyboardShortcutsOpen();
	}

	function openFind() {
		closeOverlays();
		findOpen = true;
		searchFindQuery(findQuery);
	}

	function closeFind() {
		findOpen = false;
		findActiveMatch = 0;
		findMatchCount = 0;
		findRequestToken += 1;
		latestFindRequestId = null;
		void stopFindInPage("clearSelection");
	}

	function searchFindQuery(query: string, findNext = false, forward = true) {
		const requestToken = (findRequestToken += 1);
		if (!query || !supportsFindInPage()) {
			findActiveMatch = 0;
			findMatchCount = 0;
			latestFindRequestId = null;
			if (!query) {
				void stopFindInPage("clearSelection");
			}
			return;
		}
		void findInPage(query, { findNext, forward })
			.then((requestId) => {
				if (requestToken === findRequestToken) {
					latestFindRequestId = requestId;
				}
			})
			.catch(() => {
				if (requestToken === findRequestToken) {
					findActiveMatch = 0;
					findMatchCount = 0;
					latestFindRequestId = null;
				}
			});
	}

	function handleFindQueryChange(query: string) {
		findQuery = query;
		searchFindQuery(query);
	}

	function handleFindNext() {
		searchFindQuery(findQuery, true, true);
	}

	function handleFindPrevious() {
		searchFindQuery(findQuery, true, false);
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
		void context.commands.navigation.toggleSelectedSessionView(viewKind);
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
			(recentThreadSwitcherDialog.open ||
				keyboardShortcutsDialog.open ||
				findOpen)
		) {
			event.preventDefault();
			if (findOpen) {
				closeFind();
			} else {
				closeOverlays();
			}
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

		if (shortcutAction.id === "find-in-page" && !supportsFindInPage()) {
			return;
		}

		if (shortcutAction.id === "switch-recent-thread") {
			event.preventDefault();
			void context.commands.dialogs.setKeyboardShortcutsOpen(false);
			void context.commands.dialogs.setRecentThreadSwitcherCommitModifier(
				shortcutAction.commitModifier,
			);
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
		if (shortcutAction.id === "find-in-page") {
			openFind();
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
		if (!supportsFindInPage()) {
			return;
		}

		let unsubscribe: (() => void) | null = null;
		let canceled = false;
		void onFindInPageResult((result) => {
			if (result.requestId !== latestFindRequestId) {
				return;
			}
			findActiveMatch = result.activeMatchOrdinal;
			findMatchCount = result.matches;
		}).then((nextUnsubscribe) => {
			if (canceled) {
				nextUnsubscribe();
				return;
			}
			unsubscribe = nextUnsubscribe;
		});

		return () => {
			canceled = true;
			unsubscribe?.();
		};
	});
</script>

<svelte:window
	onkeydown={handleWindowKeydown}
	onkeyup={handleWindowKeyup}
	onblur={closeOverlays}
/>

<RecentThreadSwitcherDialog
	open={switcherOpen}
	threads={switcherThreads}
	selectedKey={effectiveSwitcherSelectedKey}
	helpText={switcherHelpText}
	onHover={handleTabSwitcherHover}
	onSelect={handleTabSwitcherSelect}
	onClose={closeTabSwitcher}
/>

<KeyboardShortcutHelpDialog
	open={keyboardShortcutsDialog.open}
	shortcuts={globalShortcuts as GlobalShortcut[]}
/>

<GlobalFindBar
	open={findOpen}
	query={findQuery}
	activeMatch={findActiveMatch}
	matchCount={findMatchCount}
	onQueryChange={handleFindQueryChange}
	onNext={handleFindNext}
	onPrevious={handleFindPrevious}
	onClose={closeFind}
/>
