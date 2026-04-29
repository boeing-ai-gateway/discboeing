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

test("setAwaitingInitialStatus immediately kicks off a session reload", () => {
	const source = readSource();

	assert.match(source, /setAwaitingInitialStatus: \(sessionId\) => \{/);
	assert.match(source, /clearInitialStatusRetry\(\);/);
	assert.match(source, /if \(sessionId\) \{/);
	assert.match(source, /void reloadSession\(sessionId\);/);
});
