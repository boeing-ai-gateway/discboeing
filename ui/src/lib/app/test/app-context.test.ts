import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const APP_CONTEXT_SOURCE = path.resolve(
	import.meta.dirname,
	"../../context/app-context.svelte.ts",
);

function readAppContextSource() {
	return readFileSync(APP_CONTEXT_SOURCE, "utf-8");
}

test("session-level thread updates refresh session artifacts", () => {
	const source = readAppContextSource();

	assert.match(
		source,
		/const refreshSessionArtifacts = \(sessionId: string\) => \{/,
	);
	assert.match(source, /sessionContext\.services\.invalidate\(\);/);
	assert.match(source, /sessionContext\.hooks\.invalidate\(\);/);
	assert.match(source, /sessionContext\.files\.refresh\(\)\.catch/);
	assert.match(
		source,
		/refreshSessionArtifacts\(threadData\.sessionId\);\n\t\t\tvoid sessionContext\.threads\.refresh\(\);/,
	);
});
