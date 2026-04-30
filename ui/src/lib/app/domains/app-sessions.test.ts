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

test("app sessions retries the initial session reload after materializing a pending session", () => {
	const source = readSource();

	assert.match(
		source,
		/const INITIAL_SESSION_STATUS_RETRY_DELAYS_MS = \[150, 300, 500, 1000\];/,
	);
	assert.match(
		source,
		/function scheduleInitialStatusRetry\(sessionId: string, attempt = 0\): void \{/,
	);
	assert.match(source, /void reloadSession\(sessionId, attempt \+ 1\);/);
	assert.match(source, /scheduleInitialStatusRetry\(sessionId, attempt\);/);
});

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

test("setAwaitingInitialStatus immediately kicks off a session reload", () => {
	const source = readSource();

	assert.match(source, /setAwaitingInitialStatus: \(sessionId\) => \{/);
	assert.match(source, /clearInitialStatusRetry\(\);/);
	assert.match(source, /if \(sessionId\) \{/);
	assert.match(source, /void reloadSession\(sessionId\);/);
});
