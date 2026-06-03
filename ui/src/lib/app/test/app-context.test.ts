import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const APP_CONTEXT_SOURCE = path.resolve(
	import.meta.dirname,
	"../app-runtime.svelte.ts",
);

function readAppContextSource() {
	return readFileSync(APP_CONTEXT_SOURCE, "utf-8");
}

test("session-level thread updates refresh session artifacts", () => {
	const source = readAppContextSource();

	assert.match(
		source,
		/function refreshSessionArtifacts\(sessionId: string\): void \{/,
	);
	assert.match(source, /sessionContext\.services\.invalidate\(\);/);
	assert.match(source, /sessionContext\.hooks\.invalidate\(\);/);
	assert.match(source, /sessionContext\.files\.refresh\(\)\.catch/);
	assert.match(
		source,
		/refreshSessionArtifacts\(threadData\.sessionId\);\s*void sessionContext\.threads\.refresh\(\)\.then\(syncRuntimeProjections\);/,
	);
});

test("app runtime initializes and filters pending session ids", () => {
	const source = readAppContextSource();

	assert.match(source, /function ensurePendingSessionId\(\): string \{/);
	assert.match(
		source,
		/if \(!selection\.pendingSessionId\) \{\s*selection\.pendingSessionId = generateId\(\);/,
	);
	assert.match(
		source,
		/export function initializeAppRuntime[\s\S]*ensurePendingSessionId\(\);/,
	);
	assert.match(
		source,
		/\.filter\(\(sessionId\): sessionId is string => !!sessionId\)/,
	);
	assert.match(source, /if \(!sessionId\) \{\s*return false;/);
	assert.match(
		source,
		/const resolvedSessionId = sessionId \|\| ensurePendingSessionId\(\);/,
	);
});
