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
	assert.match(source, /if \(!session\.current\) \{/);
	assert.match(source, /void thread\.connect\(\);/);
	assert.doesNotMatch(
		source,
		/untrack\(\(\) => \{\s*void thread\.connect\(\);\s*\}\);/,
	);
	assert.doesNotMatch(source, /if \(!props\.visible\) \{/);
	assert.doesNotMatch(source, /currentThread\.disconnect\(\)/);
	assert.match(source, /onDestroy\(\(\) => \{/);
	assert.match(source, /thread\.dispose\(\);/);
});
