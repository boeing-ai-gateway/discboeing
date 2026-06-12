import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const THREAD_WORKSPACE_HEADER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/ThreadWorkspaceHeader.svelte",
);

function readThreadWorkspaceHeaderSource() {
	return readFileSync(THREAD_WORKSPACE_HEADER_COMPONENT, "utf-8");
}

test("thread workspace header renders the shared state badge", () => {
	const source = readThreadWorkspaceHeaderSource();

	assert.match(source, /import ThreadStateBadge/);
	assert.match(
		source,
		/<ThreadStateBadge \{state\} class="px-2 text-\[11px\]" \/>/,
	);
	assert.doesNotMatch(source, /function threadStateLabel/);
});

test("thread workspace header can reserve space for the floating sidebar", () => {
	const source = readThreadWorkspaceHeaderSource();

	assert.match(source, /reserveSidebarSpace\?: boolean/);
	assert.match(source, /\{#if reserveSidebarSpace\}/);
	assert.match(source, /w-\[10\.75rem\]/);
});

test("thread workspace header no longer renders a mobile sidebar toggle", () => {
	const source = readThreadWorkspaceHeaderSource();

	assert.doesNotMatch(source, /showSidebarToggle\?: boolean/);
	assert.doesNotMatch(source, /Expand sessions panel/);
	assert.doesNotMatch(source, /<PanelLeftIcon/);
});

test("thread workspace header keeps the intended non-drag layout and avoids desktop drag regions", () => {
	const source = readThreadWorkspaceHeaderSource();

	assert.match(
		source,
		/class="flex h-10 min-w-0 items-center gap-1 bg-background px-3"/,
	);
	assert.doesNotMatch(source, /data-desktop-drag-region/);
	assert.doesNotMatch(source, /desktop-no-drag/);
});

test("thread workspace header accepts optional title content without owning selector data", () => {
	const source = readThreadWorkspaceHeaderSource();

	assert.match(source, /import type \{ Snippet \} from "svelte"/);
	assert.match(source, /titleContent\?: Snippet/);
	assert.match(source, /\{#if titleContent\}/);
	assert.match(source, /\{@render titleContent\(\)\}/);
	assert.doesNotMatch(source, /displayThreadDropdown\?: boolean/);
	assert.doesNotMatch(source, /onThreadChange\?: \(threadId: string\) => void/);
	assert.doesNotMatch(source, /threadList\?: Pick<Thread, "id" \| "name">\[\]/);
	assert.doesNotMatch(source, /Select,/);
});
