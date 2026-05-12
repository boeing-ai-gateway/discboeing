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
	"../app/parts/RecentThreadSwitcherDialog.svelte",
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
		/import RecentThreadSwitcherDialog from "\$lib\/components\/app\/parts\/RecentThreadSwitcherDialog\.svelte";/,
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
	assert.match(
		source,
		/import \{[\s\S]*resolveThreadContextDisplayStatus,[\s\S]*resolveThreadDisplayStatus,[\s\S]*\} from "\$lib\/app\/thread-status";/,
	);
	assert.match(
		source,
		/const isMacPlatform = \$derived\.by\(\(\) => detectIsMacPlatform\(\)\)/,
	);
	assert.match(
		source,
		/const globalShortcuts = \$derived\.by\(\(\) => getGlobalShortcuts\(isMacPlatform\)\)/,
	);
	assert.match(source, /sessions: app\.sessions\.sessions,/);
	assert.match(source, /recentThreads: app\.sessions\.recentThreads,/);
	assert.match(source, /let tabSwitcherOpen = \$state\(false\)/);
	assert.match(
		source,
		/let tabSwitcherCommitModifier = \$state<SwitcherCommitModifier \| null>\(null\)/,
	);
	assert.match(
		source,
		/const selectedIndex = tabSwitcherOpen[\s\S]*: selectedThreadKey[\s\S]*\? 0[\s\S]*: -1;/,
	);
	assert.match(source, /const nextIndex =[\s\S]*selectedIndex >= 0[\s\S]*: 0;/);
	assert.match(
		source,
		/shouldCommitTabSwitcherOnKeyup\(event, tabSwitcherCommitModifier\)/,
	);
	assert.match(source, /function handleWindowKeydown\(event: KeyboardEvent\)/);
	assert.match(source, /function handleWindowKeyup\(event: KeyboardEvent\)/);
	assert.match(source, /app\.ui\.setMobileSidebarOpen\(false\);/);
	assert.match(source, /<svelte:window\s+onkeydown=\{handleWindowKeydown\}/);
	assert.match(source, /onkeyup=\{handleWindowKeyup\}/);
	assert.match(source, /onblur=\{closeOverlays\}/);
	assert.match(source, /<RecentThreadSwitcherDialog/);
	assert.match(source, /<KeyboardShortcutHelpDialog/);
	assert.doesNotMatch(source, /availableThreads/);
	assert.doesNotMatch(source, /sessionContext\.ui\.mobileSidebarOpen = false;/);
});

test("app keyboard shortcuts uses centralized thread status display", () => {
	const source = readSource(APP_KEYBOARD_SHORTCUTS_COMPONENT);

	assert.match(source, /function switcherThreadDisplayStatus/);
	assert.match(source, /resolveThreadContextDisplayStatus\(threadContext\)/);
	assert.match(source, /return resolveThreadDisplayStatus\(\{/);
	assert.match(source, /sessionStatus: session\.status/);
	assert.match(
		source,
		/sessionActivityStatus: session\.threadStatus\?\.status/,
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
		/import ThreadStatusIcon from "\$lib\/components\/app\/parts\/ThreadStatusIcon\.svelte";/,
	);
	assert.match(source, /threadStatuses\[threadKey\] \?\? "unknown"/);
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
