import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const THREAD_WORKSPACE_ACTIVE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ThreadWorkspaceActive.svelte",
);

function readThreadWorkspaceActiveSource() {
	return readFileSync(THREAD_WORKSPACE_ACTIVE_COMPONENT, "utf-8");
}

test("thread workspace active reconnects when the session becomes available", () => {
	const source = readThreadWorkspaceActiveSource();

	assert.match(source, /\$effect\(\(\) => \{/);
	assert.match(source, /if \(!props\.visible \|\| !session\.current\) \{/);
	assert.match(
		source,
		/connectThread\(session\.sessionId, thread\.threadId\);/,
	);
	assert.doesNotMatch(
		source,
		/untrack\(\(\) => \{\s*void thread\.connect\(\);\s*\}\);/,
	);
	assert.doesNotMatch(source, /currentThread\.disconnect\(\)/);
	assert.match(source, /onDestroy\(\(\) => \{/);
	assert.match(source, /releaseThreadState\(session\.sessionId, thread\);/);
});

test("thread workspace active passes ids and view into dock panel", () => {
	const source = readThreadWorkspaceActiveSource();

	assert.match(
		source,
		/<DockPanel\s+sessionId=\{session\.sessionId\}\s+threadId=\{thread\.threadId\}\s+sessionView=\{session\.ui\}/,
	);
	assert.doesNotMatch(source, /<DockPanel \{session\} \{thread\}/);
});
