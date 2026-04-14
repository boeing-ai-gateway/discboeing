import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const APP_SHELL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppShell.svelte",
);

function readAppShellSource() {
	return readFileSync(APP_SHELL_COMPONENT, "utf-8");
}

test("app shell only preloads app-ui mounted sessions after they have been visited", () => {
	const source = readAppShellSource();

	assert.match(
		source,
		/const mountedSessionIds = \$derived\.by\(\(\) => app\.ui\.mountedSessionIds\)/,
	);
	assert.match(source, /let visitedSessionIds = \$state<string\[\]>\(\[\]\)/);
	assert.match(source, /const preloadSessionIds = \$derived\.by\(\(\) =>/);
	assert.match(
		source,
		/sessionId === app\.sessions\.selectedId \|\|\s*visitedSessionIds\.includes\(sessionId\)/,
	);
	assert.match(
		source,
		/if \(!selectedSessionId \|\| visitedSessionIds\.includes\(selectedSessionId\)\) \{/,
	);
	assert.match(
		source,
		/visitedSessionIds = \[\.\.\.visitedSessionIds, selectedSessionId\]/,
	);
	assert.match(
		source,
		/const renderedSessionIds = \$derived\.by\(\(\) => \{[\s\S]*const sessionIds = selectedSession\.isPending[\s\S]*\? preloadSessionIds[\s\S]*: \[selectedSessionId, \.\.\.preloadSessionIds\];[\s\S]*new Set\(sessionIds\)/,
	);
	assert.match(
		source,
		/const nextManagedIds = Array\.from\([\s\S]*new Set\(\[[\s\S]*currentSelectedSessionId,[\s\S]*\.\.\.preloadSessionIds,[\s\S]*\.\.\.managedSessionIds,[\s\S]*\]\),[\s\S]*\)\.slice\(0, mountedSessionIds\.length \|\| 1\);/,
	);
	assert.match(
		source,
		/for \(const sessionId of managedSessionIds\) \{[\s\S]*if \(nextManagedIds\.includes\(sessionId\)\) \{[\s\S]*continue;/,
	);
	assert.match(source, /managedSessionIds = nextManagedIds;/);
	assert.match(
		source,
		/\{#each renderedSessionIds as sessionId \(sessionId\)\}/,
	);
});

test("app shell restores dynamic desktop sidebar sizing", () => {
	const source = readAppShellSource();

	assert.match(source, /const SIDEBAR_MIN_WIDTH_PX = 300;/);
	assert.match(source, /const SIDEBAR_MIN_SIZE_FALLBACK = 10;/);
	assert.match(
		source,
		/let desktopPaneGroupElement = \$state<HTMLDivElement \| null>\(null\)/,
	);
	assert.match(
		source,
		/let desktopSidebarMinSize = \$state\(SIDEBAR_MIN_SIZE_FALLBACK\)/,
	);
	assert.match(source, /function updateDesktopSidebarMinSize\(width: number\)/);
	assert.match(source, /new ResizeObserver\(\(entries\) => \{/);
	assert.match(source, /resizeObserver\.observe\(desktopPaneGroupElement\)/);
	assert.match(source, /bind:this=\{desktopPaneGroupElement\}/);
	assert.match(source, /minSize=\{desktopSidebarMinSize\}/);
});

test("app shell renders the floating sidebar trigger when the desktop pane is collapsed", () => {
	const source = readAppShellSource();

	assert.match(source, /\{#if !sessionView\.desktopSidebarOpen\}/);
	assert.match(source, /<AppSidebar\s+mode="floating"\s+collapsed/);
	assert.match(
		source,
		/reserveSidebarSpace=\{!isMobile\.current && !sidebarOpen\(\)\}/,
	);
	assert.match(
		source,
		/<AppHeader \{showSessionToolbar\} onToggleSidebar=\{toggleSidebar\} \/>/,
	);
});

test("app shell re-syncs the desktop pane state when the selected session changes", () => {
	const source = readAppShellSource();

	assert.match(
		source,
		/selectedSession\.ui\.desktopSidebarOpen = !desktopSidebarPane\.isCollapsed\(\);/,
	);
	assert.match(
		source,
		/const paneCollapsed = desktopSidebarPane\.isCollapsed\(\)/,
	);
	assert.match(
		source,
		/if \(sessionView\.desktopSidebarOpen && paneCollapsed\) \{\s*desktopSidebarPane\.expand\(\);/,
	);
	assert.match(
		source,
		/if \(!sessionView\.desktopSidebarOpen && !paneCollapsed\) \{\s*desktopSidebarPane\.collapse\(\);/,
	);
});

test("app shell renders the extracted keyboard shortcut controller", () => {
	const source = readAppShellSource();

	assert.match(
		source,
		/import AppKeyboardShortcuts from "\$lib\/components\/app\/AppKeyboardShortcuts\.svelte";/,
	);
	assert.match(source, /<AppKeyboardShortcuts \/>/);
	assert.doesNotMatch(source, /function handleWindowKeydown/);
	assert.doesNotMatch(source, /\{#if keyboardHelpOpen\}/);
});
