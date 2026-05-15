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

test("thread workspace reconnects when the session becomes available", () => {
	const source = readThreadWorkspaceSource();

	assert.match(source, /\$effect\(\(\) => \{/);
	assert.match(source, /if \(!visible \|\| !session\.current\) \{/);
	assert.match(source, /void thread\.start\(\);/);
	assert.doesNotMatch(
		source,
		/untrack\(\(\) => \{\s*void thread\.start\(\);\s*\}\);/,
	);
	assert.doesNotMatch(source, /currentThread\.disconnect\(\)/);
	assert.match(source, /onDestroy\(\(\) => \{/);
	assert.match(source, /thread\.dispose\(\);/);
});
