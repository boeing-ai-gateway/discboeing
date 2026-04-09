import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const APP_SHELL_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppShell.svelte",
);

function readAppShellSource() {
	return readFileSync(APP_SHELL_COMPONENT, "utf-8");
}

test("app shell only preloads recent sessions after they have been visited", () => {
	const source = readAppShellSource();

	assert.match(source, /let visitedSessionIds = \$state<string\[\]>\(\[\]\)/);
	assert.match(source, /const preloadSessionIds = \$derived\.by\(\(\) =>/);
	assert.match(
		source,
		/sessionId === app\.sessions\.selectedId \|\|\s*visitedSessionIds\.includes\(sessionId\)/,
	);
	assert.match(
		source,
		/if \(!selectedSessionId \|\| visitedSessionIds\.includes\(selectedSessionId\)\) \{/,
	);
	assert.match(
		source,
		/visitedSessionIds = \[\.\.\.visitedSessionIds, selectedSessionId\]/,
	);
	assert.match(
		source,
		/new Set\(\[\.\.\.preloadSessionIds, currentSelectedSessionId\]\)/,
	);
	assert.match(
		source,
		/\{#each preloadSessionIds as sessionId \(sessionId\)\}/,
	);
});
