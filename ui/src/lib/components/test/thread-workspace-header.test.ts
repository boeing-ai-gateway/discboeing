import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const THREAD_WORKSPACE_HEADER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/ThreadWorkspaceHeader.svelte",
);

function readThreadWorkspaceHeaderSource() {
	return readFileSync(THREAD_WORKSPACE_HEADER_COMPONENT, "utf-8");
}

test("thread workspace header renders interrupted and cancelled badges", () => {
	const source = readThreadWorkspaceHeaderSource();

	assert.match(source, /function threadStateLabel/);
	assert.match(source, /value === "interrupted"/);
	assert.match(source, /value === "cancelled"/);
	assert.match(source, /\{threadStateLabel\(state\)\}/);
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
