import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const SESSION_WORKSPACE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionWorkspace.svelte",
);

function readSessionWorkspaceSource() {
	return readFileSync(SESSION_WORKSPACE_COMPONENT, "utf-8");
}

test("session workspace computes desktop sidebar min size from a 40px target", () => {
	const source = readSessionWorkspaceSource();

	assert.match(source, /const SIDEBAR_MIN_WIDTH_PX = 40;/);
	assert.match(source, /const SIDEBAR_MIN_SIZE_FALLBACK = 4;/);
	assert.match(source, /function updateDesktopSidebarMinSize\(width: number\)/);
	assert.match(source, /\(SIDEBAR_MIN_WIDTH_PX \/ width\) \* 100/);
	assert.match(source, /Math\.min\(\s*48,/);
	assert.match(source, /Math\.max\(SIDEBAR_MIN_SIZE_FALLBACK,/);
	assert.match(source, /minSize=\{desktopSidebarMinSize\}/);
	assert.match(source, /bind:this=\{desktopPaneGroupElement\}/);
	assert.match(source, /new ResizeObserver/);
});
