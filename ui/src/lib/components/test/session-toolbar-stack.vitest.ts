import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const SESSION_TOOLBAR_STACK_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionToolbarStack.svelte",
);

function readSessionToolbarStackSource() {
	return readFileSync(SESSION_TOOLBAR_STACK_COMPONENT, "utf-8");
}

test("session toolbar stack only shows a toolbar for a real selected session", () => {
	const source = readSessionToolbarStackSource();

	assert.match(
		source,
		/const mountedSessionIds = \$derived\.by\(\s*\(\) => context\.view\.navigation\.mountedSessionIds,\s*\)/,
	);
	assert.match(
		source,
		/const selectedSessionId = \$derived\.by\(\(\) => context\.view\.selection\.sessionId\)/,
	);
	assert.match(
		source,
		/\{#each mountedSessionIds as sessionId \(sessionId\)\}/,
	);
	assert.match(
		source,
		/class=\{sessionId === selectedSessionId \? "contents" : "hidden"\}/,
	);
	assert.match(
		source,
		/\{#if shouldLoadSessionToolbar\(context, sessionId\)\}/,
	);
	assert.doesNotMatch(source, /function shouldRenderSessionToolbar/);
	assert.doesNotMatch(source, /selectedId \?\? app\.sessions\.pendingId/);
});
