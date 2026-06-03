import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

function readSource(fileName: string) {
	return readFileSync(path.resolve(import.meta.dirname, fileName), "utf-8");
}

test("RecentThreadStore can prune a single deleted thread", () => {
	const source = readSource("recent-threads.store.svelte.ts");

	assert.match(
		source,
		/pruneThread\(sessionId: string, threadId: string\): void \{/,
	);
	assert.match(
		source,
		/this\.#entries\.filter\([\s\S]*entry\.sessionId !== sessionId \|\| entry\.threadId !== threadId[\s\S]*\)/,
	);
	assert.match(
		source,
		/if \(this\.#lastRecordedKey === recentThreadKey\(sessionId, threadId\)\) \{/,
	);
	assert.match(source, /this\.#lastRecordedKey = null;/);
});

test("initial thread restore is separate from recent entries", () => {
	const source = readSource("recent-threads.store.svelte.ts");

	assert.match(
		source,
		/const ACTIVE_THREAD_SELECTION_STORAGE_KEY = "active\.thread";/,
	);
	assert.match(source, /readStorage\(ACTIVE_THREAD_SELECTION_STORAGE_KEY\)/);
	assert.doesNotMatch(
		source,
		/readInitialThreadSelection\(\):[\s\S]*const entries = readEntries\(\);/,
	);
	assert.match(source, /writeActiveThreadSelection\(entry\);/);
	assert.match(source, /writeActiveThreadSelection\(null\);/);
});

test("session context prunes recent-thread entries when a thread is removed", () => {
	const source = readSource("../session/create-session-state.svelte.ts");

	assert.match(source, /onThreadRemoved: \(threadId\) => \{/);
	assert.match(
		source,
		/app\.stores\.recentThreads\.pruneThread\(sessionId, threadId\);/,
	);
});
