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

test("app shell renders mounted sessions without owning their contexts", () => {
	const source = readAppShellSource();

	assert.match(
		source,
		/const mountedSessionIds = \$derived\.by\(\(\) => app\.ui\.mountedSessionIds\)/,
	);
	assert.match(
		source,
		/\{#each mountedSessionIds as sessionId \(sessionId\)\}/,
	);
	assert.match(
		source,
		/\{#if app\.sessions\.shouldLoadSession\(sessionId, \{ includePending: true \}\)\}/,
	);
	assert.doesNotMatch(source, /function shouldRenderSessionWorkspace/);
	assert.match(source, /visible=\{sessionId === currentSelectedSessionId\}/);
	assert.doesNotMatch(source, /renderedSessionIds/);
	assert.doesNotMatch(source, /ensureSessionContext/);
	assert.doesNotMatch(source, /managedSessionIds/);
	assert.doesNotMatch(source, /sessionContexts\.delete/);
	assert.doesNotMatch(source, /session\.dispose\(/);
	assert.doesNotMatch(source, /onDestroy\(/);
	assert.doesNotMatch(source, /selectedSession\.isPending/);
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

test("app shell moves the desktop sidebar trigger into the header when collapsed", () => {
	const source = readAppShellSource();

	assert.match(
		source,
		/const showDesktopSidebarToggle = \$derived\.by\(\s*\(\) => !isMobile\.current && !app\.ui\.desktopSidebarOpen,\s*\);/,
	);
	assert.doesNotMatch(source, /reserveSidebarSpace=/);
	assert.match(
		source,
		/<AppHeader\s+\{showSessionToolbar\}\s+\{showDesktopSidebarToggle\}\s+onToggleSidebar=\{toggleSidebar\}\s+\/>/,
	);
	assert.doesNotMatch(source, /<AppSidebar\s+mode="floating"\s+collapsed/);
});

test("app shell re-syncs the desktop pane state when the selected session changes", () => {
	const source = readAppShellSource();

	assert.match(
		source,
		/app\.ui\.setDesktopSidebarOpen\(!desktopSidebarPane\.isCollapsed\(\)\);/,
	);
	assert.match(
		source,
		/const paneCollapsed = desktopSidebarPane\.isCollapsed\(\)/,
	);
	assert.match(
		source,
		/if \(app\.ui\.desktopSidebarOpen && paneCollapsed\) \{\s*desktopSidebarPane\.expand\(\);/,
	);
	assert.match(
		source,
		/if \(!app\.ui\.desktopSidebarOpen && !paneCollapsed\) \{\s*desktopSidebarPane\.collapse\(\);/,
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
