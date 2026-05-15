import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

function readSource() {
	return readFileSync(
		path.resolve(import.meta.dirname, "app-sessions.svelte.ts"),
		"utf-8",
	);
}

test("app sessions prune stale recent-session entries after a successful session list refresh", () => {
	const source = readSource();

	assert.match(source, /function purgeMissingRecentSessions\(\): void \{/);
	assert.match(source, /const validSessionIds = Object\.fromEntries\(/);
	assert.match(
		source,
		/if \(!validSessionIds\[entry\.sessionId\]\) \{[\s\S]*trackStaleSessionId\(entry\.sessionId\);[\s\S]*\}/,
	);
	assert.match(
		source,
		/await store\.fetch\(\);\s*purgeMissingRecentSessions\(\);/,
	);
});

test("app sessions does not expose a session-load predicate", () => {
	const source = readSource();

	assert.doesNotMatch(source, /function shouldLoadSession/);
	assert.doesNotMatch(source, /shouldLoadSession,/);
});

test("app sessions does not track a separate materialization state", () => {
	const source = readSource();

	assert.doesNotMatch(source, /awaitingInitialStatus/);
	assert.doesNotMatch(source, /INITIAL_SESSION_STATUS_RETRY_DELAYS_MS/);
	assert.doesNotMatch(source, /stageOptimisticMessages/);
});
