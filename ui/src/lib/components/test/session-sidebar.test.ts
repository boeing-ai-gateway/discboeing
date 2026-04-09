import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const SESSION_SIDEBAR_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppSidebar.svelte",
);

function readSessionSidebarSource() {
	return readFileSync(SESSION_SIDEBAR_COMPONENT, "utf-8");
}

test("session sidebar recent threads render lastMessage as the subtitle", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /function hasRecentThreadSubtitle/);
	assert.match(source, /threadObj\.lastMessage \?\? ""/);
	assert.match(source, /\{threadObj\.lastMessage \?\? ""\}/);
	assert.doesNotMatch(source, /\{threadObj\.sessionName \|\| "New Session"\}/);
});

test("session sidebar recent threads render a state badge when present", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /function recentThreadStateLabel/);
	assert.match(source, /threadObj\.state === "interrupted"/);
	assert.match(source, /threadObj\.state === "cancelled"/);
	assert.match(source, /\{recentThreadStateLabel\(threadObj\)\}/);
});

test("session sidebar keys session and recent thread rows", () => {
	const source = readSessionSidebarSource();

	assert.match(
		source,
		/\{#each sessions\.list as sessionObj \(sessionObj\.id\)\}/,
	);
	assert.match(
		source,
		/\{#each visibleRecentThreads as threadObj \(`\$\{threadObj\.sessionId\}:\$\{threadObj\.threadId\}`\)\}/,
	);
});

test("session sidebar caps the visible recent thread list", () => {
	const source = readSessionSidebarSource();

	assert.match(
		source,
		/const visibleRecentThreads = \$derived\.by\(\(\) =>\s*sessions\.recentThreads\.slice\(0, preferences\.recentThreadsVisibleLimit\),\s*\)/,
	);
	assert.match(
		source,
		/const showRecentThreads = \$derived\([\s\S]*preferences\.recentThreadsVisibleLimit > 1[\s\S]*visibleRecentThreads\.length > 0,[\s\S]*\)/,
	);
});

test("session sidebar hides the recent section when the visible limit is disabled", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /\{#if showRecentThreads\}/);
	assert.doesNotMatch(source, /\{#if visibleRecentThreads\.length > 0\}/);
});

test("session sidebar hides the all sessions header and keeps sessions visible when recents are disabled", () => {
	const source = readSessionSidebarSource();

	assert.match(
		source,
		/const showAllSessionsHeader = \$derived\(showRecentThreads\)/,
	);
	assert.match(source, /\{#if showAllSessionsHeader\}/);
	assert.match(
		source,
		/\{:else\}\s*<div\s*\n\s*class=\{preferences\.sidebarAllGroupedByWorkspace/,
	);
	assert.doesNotMatch(
		source,
		/All sessions\s*<\/Collapsible\.Trigger>\s*<Collapsible\.Content[\s\S]*\{:else if sessions\.list\.length > 0\}/,
	);
});

test("session sidebar supports dropdown reuse and closes after creating a session", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /mode\?: "panel" \| "dropdown" \| "floating"/);
	assert.match(source, /const dropdownMode = \$derived\(mode === "dropdown"\)/);
	assert.match(source, /const floatingMode = \$derived\(mode === "floating"\)/);
	assert.match(source, /function handleStartNewSession\(\)/);
	assert.match(source, /onclick=\{handleStartNewSession\}/);
});

test("session sidebar owns the collapsed floating overlay state", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /collapsed\?: boolean/);
	assert.match(source, /let floatingOpen = \$state\(false\)/);
	assert.match(source, /function toggleFloatingSidebar\(\)/);
	assert.match(
		source,
		/const showSidebarBody = \$derived\(!floatingCollapsed \|\| floatingOpen\)/,
	);
	assert.match(source, /aria-expanded=\{floatingOpen\}/);
});

test("session sidebar can group all sessions by workspace type", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /sidebarAllGroupedByWorkspace/);
	assert.match(
		source,
		/const workspaceSessionGroups = \$derived\.by\(\(\) => \{/,
	);
	assert.match(source, /workspace\.sourceType === "managed"/);
	assert.match(source, /function trimWorkspacePrefix/);
	assert.match(source, /<GitBranchIcon class="size-3 shrink-0" \/>/);
	assert.match(source, /<FolderIcon class="size-3 shrink-0" \/>/);
	assert.match(source, /<PackageIcon class="size-3 shrink-0" \/>/);
	assert.match(source, /New Workspace/);
	assert.match(source, /Rename workspace/);
	assert.match(source, /Delete workspace\?/);
	assert.match(source, /<Switch/);
});
