import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const APP_KEYBOARD_SHORTCUTS_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppKeyboardShortcuts.svelte",
);
const KEYBOARD_SHORTCUT_HELP_DIALOG_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/KeyboardShortcutHelpDialog.svelte",
);
const RECENT_THREAD_SWITCHER_DIALOG_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/RecentThreadSwitcherDialog.svelte",
);
const GLOBAL_SHORTCUTS_MODULE = path.resolve(
	import.meta.dirname,
	"../../app/global-shortcuts.ts",
);

function readSource(filePath: string) {
	return readFileSync(filePath, "utf-8");
}

test("app keyboard shortcuts owns the global keyboard controller", () => {
	const source = readSource(APP_KEYBOARD_SHORTCUTS_COMPONENT);

	assert.match(
		source,
		/import RecentThreadSwitcherDialog from "\$lib\/components\/app\/RecentThreadSwitcherDialog\.svelte";/,
	);
	assert.match(
		source,
		/import KeyboardShortcutHelpDialog from "\$lib\/components\/app\/parts\/KeyboardShortcutHelpDialog\.svelte";/,
	);
	assert.match(
		source,
		/import \{[\s\S]*detectIsMacPlatform,[\s\S]*getGlobalShortcuts,[\s\S]*matchGlobalShortcutKeydown,[\s\S]*shouldCommitTabSwitcherOnKeyup,[\s\S]*\} from "\$lib\/app\/global-shortcuts";/,
	);
	assert.match(
		source,
		/import \{[\s\S]*getAvailableSwitcherThreads,[\s\S]*getThreadSwitcherThreads,[\s\S]*recentThreadKey,[\s\S]*\} from "\$lib\/app\/thread-switcher";/,
	);
	assert.doesNotMatch(source, /resolveThreadDisplayStatus/);
	assert.match(
		source,
		/const isMacPlatform = \$derived\.by\(\(\) => detectIsMacPlatform\(\)\)/,
	);
	assert.match(
		source,
		/const globalShortcuts = \$derived\.by\(\(\) => getGlobalShortcuts\(isMacPlatform\)\)/,
	);
	assert.match(source, /sessions: context\.data\.sessions\.items,/);
	assert.match(
		source,
		/recentThreads: context\.data\.sessions\.recentThreads,/,
	);
	assert.doesNotMatch(source, /sessions: app\.sessions\.sessions,/);
	assert.doesNotMatch(source, /recentThreads: app\.sessions\.recentThreads,/);
	assert.match(
		source,
		/import \{ useContext \} from "\$lib\/context\/context\.svelte";/,
	);
	assert.match(source, /const context = useContext\(\);/);
	assert.match(source, /context\.view\.app\.dialogs\.keyboardShortcuts/);
	assert.match(source, /context\.view\.app\.dialogs\.recentThreadSwitcher/);
	assert.match(
		source,
		/const selectedIndex = recentThreadSwitcherDialog\.open[\s\S]*: selectedThreadKey[\s\S]*\? 0[\s\S]*: -1;/,
	);
	assert.match(source, /const nextIndex =[\s\S]*selectedIndex >= 0[\s\S]*: 0;/);
	assert.match(
		source,
		/shouldCommitTabSwitcherOnKeyup\([\s\S]*event,[\s\S]*recentThreadSwitcherDialog\.commitModifier,[\s\S]*\)/,
	);
	assert.match(source, /function handleWindowKeydown\(event: KeyboardEvent\)/);
	assert.match(source, /function handleWindowKeyup\(event: KeyboardEvent\)/);
	assert.match(
		source,
		/import \{[\s\S]*createThread,[\s\S]*openThread,[\s\S]*setMobileSidebarOpen,[\s\S]*setRecentThreadSwitcherOpen,[\s\S]*startNewSession,[\s\S]*toggleKeyboardShortcutsOpen,[\s\S]*\} from "\$lib\/context\/commands\/app-view";/,
	);
	assert.match(source, /setMobileSidebarOpen\(false\);/);
	assert.match(source, /context\.view\.app\.selection\.sessionId/);
	assert.match(
		source,
		/openThread\(selectedThread\.sessionId, selectedThread\.threadId\)/,
	);
	assert.match(source, /startNewSession\(\);/);
	assert.match(source, /void createThread\(sessionId\);/);
	assert.doesNotMatch(source, /app\.sessions\.openThread/);
	assert.doesNotMatch(source, /app\.sessions\.startNew/);
	assert.doesNotMatch(source, /app\.sessions\.createThread/);
	assert.doesNotMatch(source, /let tabSwitcherOpen = \$state/);
	assert.doesNotMatch(source, /let keyboardHelpOpen = \$state/);
	assert.match(source, /<svelte:window\s+onkeydown=\{handleWindowKeydown\}/);
	assert.match(source, /onkeyup=\{handleWindowKeyup\}/);
	assert.match(source, /onblur=\{closeOverlays\}/);
	assert.match(source, /<RecentThreadSwitcherDialog/);
	assert.match(source, /<KeyboardShortcutHelpDialog/);
	assert.doesNotMatch(source, /availableThreads/);
	assert.doesNotMatch(source, /sessionContext\.ui\.mobileSidebarOpen = false;/);
});

test("app keyboard shortcuts lets the switcher render thread status", () => {
	const source = readSource(APP_KEYBOARD_SHORTCUTS_COMPONENT);

	assert.doesNotMatch(source, /switcherThreadStatuses/);
	assert.doesNotMatch(source, /function switcherThreadDisplayStatus/);
	assert.doesNotMatch(source, /threadStatuses=/);
});

test("app keyboard shortcuts handles global shortcuts inside editable targets when they match", () => {
	const source = readSource(APP_KEYBOARD_SHORTCUTS_COMPONENT);

	assert.match(
		source,
		/const shortcutAction = matchGlobalShortcutKeydown\(event, isMacPlatform\);[\s\S]*if \(!shortcutAction && isEditableShortcutTarget\(event\.target\)\) \{/,
	);
});

test("keyboard shortcut help dialog renders multiple key groups", () => {
	const source = readSource(KEYBOARD_SHORTCUT_HELP_DIALOG_COMPONENT);

	assert.match(
		source,
		/import \{ Kbd, KbdGroup \} from "\$lib\/components\/ui\/kbd";/,
	);
	assert.match(source, /\{#each shortcuts as shortcut \(shortcut\.id\)\}/);
	assert.match(
		source,
		/\{#each shortcut\.keyGroups as keyGroup, index \(keyGroup\.join\("\+"\)\)\}/,
	);
	assert.match(source, /<KbdGroup>/);
	assert.match(source, /<Kbd>\{key\}<\/Kbd>/);
	assert.match(source, /text-muted-foreground">or</);
});

test("recent thread switcher dialog renders the reusable overlay UI", () => {
	const source = readSource(RECENT_THREAD_SWITCHER_DIALOG_COMPONENT);

	assert.match(
		source,
		/import AppThreadStatus from "\$lib\/components\/app\/AppThreadStatus\.svelte";/,
	);
	assert.match(source, /<AppThreadStatus/);
	assert.match(source, /sessionId=\{thread\.sessionId\}/);
	assert.match(source, /threadId=\{thread\.threadId\}/);
	assert.match(source, /type Props = \{/);
	assert.match(source, /helpText: string;/);
	assert.match(
		source,
		/onHover: \(sessionId: string, threadId: string\) => void;/,
	);
	assert.match(
		source,
		/onSelect: \(sessionId: string, threadId: string\) => void;/,
	);
	assert.match(source, /Threads/);
	assert.match(source, /\{helpText\}/);
	assert.match(
		source,
		/onmouseenter=\{\(\) => onHover\(thread\.sessionId, thread\.threadId\)\}/,
	);
	assert.match(
		source,
		/onclick=\{\(\) => onSelect\(thread\.sessionId, thread\.threadId\)\}/,
	);
});

test("global shortcuts module centralizes platform-aware shortcut definitions", () => {
	const source = readSource(GLOBAL_SHORTCUTS_MODULE);

	assert.match(source, /export type GlobalShortcut = \{/);
	assert.match(source, /export function detectIsMacPlatform\(\): boolean/);
	assert.match(
		source,
		/export function getSwitcherShortcutHints\(isMacPlatform: boolean\): string\[\]\[\]/,
	);
	assert.match(source, /\["Ctrl", "Tab"\]/);
	assert.match(source, /\["Cmd", "Shift", "\]"\]/);
	assert.match(source, /\["Ctrl", "K"\]/);
	assert.match(
		source,
		/export function getGlobalShortcuts\(isMacPlatform: boolean\): GlobalShortcut\[\]/,
	);
	assert.match(source, /export function matchGlobalShortcutKeydown\(/);
	assert.match(source, /export function shouldCommitTabSwitcherOnKeyup\(/);
});
