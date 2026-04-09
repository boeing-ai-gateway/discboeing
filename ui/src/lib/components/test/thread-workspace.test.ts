import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const THREAD_WORKSPACE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ThreadWorkspace.svelte",
);

function readThreadWorkspaceSource() {
	return readFileSync(THREAD_WORKSPACE_COMPONENT, "utf-8");
}

test("thread workspace no longer renders session sidebar controls directly", () => {
	const source = readThreadWorkspaceSource();

	assert.doesNotMatch(source, /let sessionsMenuOpen = \$state\(false\)/);
	assert.doesNotMatch(source, /<SessionSidebar/);
	assert.doesNotMatch(source, /<Popover\.Root/);
	assert.match(
		source,
		/let \{[\s\S]*threadId,[\s\S]*mainClass,[\s\S]*showSidebarToggle,[\s\S]*reserveSidebarSpace,[\s\S]*onToggleSidebar,[\s\S]*mode,[\s\S]*\}: Props = \$props\(\);/,
	);
	assert.match(
		source,
		/const thread = setThreadContext\(untrack\(\(\) => threadId\)\)/,
	);
	assert.match(source, /showSidebarToggle=\{showSidebarToggle \?\? false\}/);
});
