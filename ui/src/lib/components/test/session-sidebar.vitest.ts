import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const SESSION_SIDEBAR_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppSidebar.svelte",
);
const SESSION_HEADER_DROPDOWN_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionHeaderDropdown.svelte",
);

function readSessionSidebarSource() {
	return readFileSync(SESSION_SIDEBAR_COMPONENT, "utf-8");
}

function readSessionHeaderDropdownSource() {
	return readFileSync(SESSION_HEADER_DROPDOWN_COMPONENT, "utf-8");
}

test("session sidebar recent threads only render the saved display name", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /\{threadObj\.name \|\| "New Thread"\}/);
	assert.doesNotMatch(source, /function hasRecentThreadSubtitle/);
	assert.doesNotMatch(source, /threadObj\.lastMessage/);
	assert.doesNotMatch(source, /ThreadStateBadge/);
});

test("session sidebar keys session and recent thread rows", () => {
	const source = readSessionSidebarSource();

	assert.match(
		source,
		/\{#each sessionItems as sessionObj \(sessionObj\.id\)\}/,
	);
	assert.match(
		source,
		/\{#each visibleRecentThreads as threadObj \(`\$\{threadObj\.sessionId\}:\$\{threadObj\.threadId\}`\)\}/,
	);
});

test("session sidebar caps the visible recent thread list", () => {
	const source = readSessionSidebarSource();

	assert.doesNotMatch(
		source,
		/import \{ getVisibleRecentThreads \} from "\$lib\/app\/app-helpers";/,
	);
	assert.match(
		source,
		/const visibleRecentThreads = \$derived\(\s*context\.view\.app\.recentThreads\.visibleItems,\s*\);/,
	);
	assert.match(
		source,
		/const preferences = \$derived\(context\.view\.app\.preferences\)/,
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

test("session sidebar session actions include new thread", () => {
	const source = readSessionSidebarSource();

	assert.match(
		source,
		/async function handleCreateThread\(sessionId: string\)/,
	);
	assert.match(
		source,
		/context\.commands\.navigation\.selectSession\(sessionId\)/,
	);
	assert.match(source, /await context\.commands\.threads\.createThread\(/);
	assert.match(
		source,
		/await context\.commands\.sessions\.stopSession\(sessionId/,
	);
	assert.match(
		source,
		/await context\.commands\.sessions\.renameSession\(\s*renameSessionId,\s*renameDraft/,
	);
	assert.match(
		source,
		/await context\.commands\.sessions\.deleteSession\(deleteSessionId/,
	);
	assert.doesNotMatch(source, /sessions\.select\(sessionId\)/);
	assert.doesNotMatch(source, /app\.ensureSession\(sessionId\)/);
	assert.doesNotMatch(source, /sessions\.createThread\(sessionId\)/);
	assert.doesNotMatch(source, /sessions\.stop\(sessionId\)/);
	assert.doesNotMatch(source, /sessions\.rename\(/);
	assert.doesNotMatch(source, /sessions\.remove\(/);
	assert.doesNotMatch(
		source,
		/app\.ensureSession\(sessionId\)\.threads\.create\(\)/,
	);
	assert.match(source, /New thread/);
});

test("session sidebar keeps thread children visible from root context data", () => {
	const source = readSessionSidebarSource();

	assert.match(
		source,
		/function visibleThreadsForSession\(sessionId: string\): Thread\[] \{/,
	);
	assert.match(
		source,
		/const threads = context\.data\.sessions\.byId\[sessionId\]\?\.threads;/,
	);
	assert.doesNotMatch(source, /app\.sessions\.sessionContexts/);
	assert.doesNotMatch(source, /void sessionContext\?\.threads\.refresh\(\);/);
	assert.doesNotMatch(source, /syncAppNavigationFromBridge\(\);/);
	assert.match(
		source,
		/context\.commands\.navigation\.openThread\(sessionId, threadId\);/,
	);
	assert.match(source, /context\.commands\.navigation\.startNewSession\(\);/);
	assert.doesNotMatch(source, /void sessions\.reloadSession\(sessionId\);/);
	assert.doesNotMatch(
		source,
		/if \(!session \|\| session\.status === SessionStatusValue\.STOPPED\) \{/,
	);
	assert.doesNotMatch(
		source,
		/sessions\.setAwaitingInitialStatus\(sessionId\);/,
	);
	assert.match(source, /\{#if sessionHasNestedThreads\(sessionObj\.id\)\}/);
	assert.match(
		source,
		/\{#each visibleRootThreadsForSession\(sessionObj\.id\) as threadObj \(threadObj\.id\)\}/,
	);
});

test("session sidebar thread rows render context-aware status", () => {
	const source = readSessionSidebarSource();

	assert.match(
		source,
		/import AppThreadStatus from "\$lib\/components\/app\/AppThreadStatus\.svelte";/,
	);
	assert.match(source, /<AppThreadStatus[\s\S]*\{sessionId\}/);
	assert.match(source, /threadId=\{threadObj\.id\}/);
	assert.doesNotMatch(source, /function threadDisplayStatus/);
	assert.doesNotMatch(source, /activeCommand[\s\S]*return "running"/);
});

test("session sidebar recent rows render context-aware status", () => {
	const source = readSessionSidebarSource();

	assert.doesNotMatch(source, /function recentThreadDisplayStatus/);
	assert.doesNotMatch(source, /resolveThreadDisplayStatus/);
	assert.match(
		source,
		/<AppThreadStatus[\s\S]*sessionId=\{threadObj\.sessionId\}/,
	);
	assert.match(source, /threadId=\{threadObj\.threadId\}/);
});

test("session sidebar nests task threads and renders a status icon", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /type TaskThreadMetadata = \{/);
	assert.match(source, /function threadMetadata\(threadObj: Thread\)/);
	assert.match(source, /function threadParentId\(threadObj: Thread\)/);
	assert.match(source, /threadMetadata\(threadObj\)\?\.parentThreadId/);
	assert.match(
		source,
		/visibleChildThreadsForSession\(sessionId, threadObj\.id\)\.length > 0/,
	);
	assert.match(
		source,
		/\{#each visibleChildThreadsForSession\(sessionId, threadObj\.id\) as childThreadObj \(childThreadObj\.id\)\}/,
	);
	assert.match(
		source,
		/\{@render threadItem\(sessionId, childThreadObj, depth \+ 1\)\}/,
	);
	assert.match(source, /<AppThreadStatus[\s\S]*threadId=\{threadObj\.id\}/);
});

test("session sidebar thread rows include rename and delete actions", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /function openRenameThreadDialog\(threadId: string\)/);
	assert.match(source, /function openDeleteThreadDialog\(threadId: string\)/);
	assert.match(
		source,
		/await context\.commands\.threads\.renameThread\(\s*selectedSessionId,\s*renameThreadId,\s*renameThreadDraft,/,
	);
	assert.match(
		source,
		/await context\.commands\.threads\.deleteThread\(\s*selectedSessionId,\s*deleteThreadId,/,
	);
	assert.doesNotMatch(source, /selectedSessionContext\?\.threads\.rename\(/);
	assert.doesNotMatch(source, /selectedSessionContext\?\.threads\.remove\(/);
	assert.match(source, /Thread actions for/);
	assert.match(source, /Rename thread/);
	assert.match(source, /Delete thread\?/);
});

test("session sidebar hides delete for the primary session thread", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /function isPrimaryThread\(threadId: string\)/);
	assert.match(source, /threadId === selectedSessionId/);
	assert.match(source, /isPrimaryThread\(threadId\) \|\|/);
	assert.match(source, /\{#if !isPrimaryThread\(threadObj\.id\)\}/);
});

test("session sidebar supports dropdown reuse and closes after creating a session", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /mode\?: "panel" \| "dropdown" \| "floating"/);
	assert.match(source, /onPinSidebar\?: \(\) => void/);
	assert.match(source, /const dropdownMode = \$derived\(mode === "dropdown"\)/);
	assert.match(source, /const floatingMode = \$derived\(mode === "floating"\)/);
	assert.match(source, /function handleStartNewSession\(\)/);
	assert.match(source, /onclick=\{handleStartNewSession\}/);
	assert.match(source, /import PinIcon from "@lucide\/svelte\/icons\/pin"/);
	assert.match(
		source,
		/\{#if \(dropdownMode \|\| floatingMode\) && onPinSidebar\}/,
	);
	assert.match(source, /onclick=\{onPinSidebar\}/);
	assert.match(source, /aria-label="Pin sessions sidebar"/);
	assert.match(
		source,
		/<PinIcon class="size-3\.5" \/>[\s\S]*<\/Button>[\s\S]*<Button[\s\S]*onclick=\{handleStartNewSession\}/,
	);
});

test("session header dropdown can pin the sidebar open", () => {
	const source = readSessionHeaderDropdownSource();

	assert.match(source, /onPinSidebar\?: \(\) => void/);
	assert.match(source, /function pinSidebar\(\)/);
	assert.match(source, /if \(onPinSidebar\) \{\s*onPinSidebar\(\);/);
	assert.match(source, /setDesktopSidebarOpen\(true\)/);
	assert.match(source, /onPinSidebar=\{pinSidebar\}/);
});

test("session header dropdown styles the empty Sessions label like new session", () => {
	const source = readSessionHeaderDropdownSource();

	assert.match(
		source,
		/const showingFallbackLabel = \$derived\(triggerLabel === "Sessions"\)/,
	);
	assert.match(
		source,
		/text-xs font-medium uppercase tracking-\[0\.16em\] text-foreground\/50/,
	);
	assert.match(source, /hover:text-foreground\/80/);
	assert.match(source, /class=\{triggerClass\}/);
});

test("session sidebar owns the collapsed floating overlay state", () => {
	const source = readSessionSidebarSource();

	assert.match(source, /collapsed\?: boolean/);
	assert.match(source, /let floatingOpen = \$state\(false\)/);
	assert.match(source, /function toggleFloatingSidebar\(\)/);
	assert.match(source, /\{#if onToggleSidebar && !dropdownMode\}/);
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
		/onCheckedChange=\{\(checked\) =>\s*void context\.commands\.preferences\.setSidebarAllGroupedByWorkspace\(\s*checked === true,\s*\)\}/,
	);
	assert.match(
		source,
		/context\.commands\.preferences\.setSidebarRecentOpen\(open\)/,
	);
	assert.match(
		source,
		/context\.commands\.preferences\.setSidebarAllOpen\(open\)/,
	);
	assert.match(
		source,
		/const workspaceSessionGroups = \$derived\.by\(\(\) => \{/,
	);
	assert.match(source, /workspace\.sourceType === "managed"/);
	assert.match(
		source,
		/const sessionItems = \$derived\(\s*context\.data\.sessions\.allIds[\s\S]*context\.data\.sessions\.byId\[sessionId\]\?\.value/,
	);
	assert.match(
		source,
		/const sessionsById = \$derived\(\s*Object\.fromEntries\(/,
	);
	assert.match(
		source,
		/const workspacesById = \$derived\(context\.data\.workspaces\.byId\)/,
	);
	assert.match(
		source,
		/const selectedSessionId = \$derived\(context\.view\.selection\.sessionId\)/,
	);
	assert.match(
		source,
		/\{#each sessionItems as sessionObj \(sessionObj\.id\)\}/,
	);
	assert.doesNotMatch(source, /sessions\.list/);
	assert.doesNotMatch(source, /sessions\.selectedId/);
	assert.doesNotMatch(source, /app\.ui\.visibleRecentThreads/);
	assert.doesNotMatch(source, /app\.workspaces\.peek/);
	assert.doesNotMatch(source, /app\.workspaces\.update/);
	assert.doesNotMatch(source, /app\.workspaces\.remove/);
	assert.doesNotMatch(source, /const preferences = app\.preferences/);
	assert.match(
		source,
		/await context\.commands\.workspaces\.renameWorkspace\(\s*renameWorkspaceId,\s*renameWorkspaceDraft,?\s*\)/,
	);
	assert.match(
		source,
		/await context\.commands\.workspaces\.deleteWorkspace\(deleteWorkspaceId/,
	);
	assert.match(source, /function trimWorkspacePrefix/);
	assert.match(source, /<GitBranchIcon class="size-3 shrink-0" \/>/);
	assert.match(source, /<FolderIcon class="size-3 shrink-0" \/>/);
	assert.match(source, /<PackageIcon class="size-3 shrink-0" \/>/);
	assert.match(source, /Unnamed Workspace/);
	assert.match(source, /Rename workspace/);
	assert.match(source, /Delete workspace\?/);
	assert.match(source, /<Switch/);
});
