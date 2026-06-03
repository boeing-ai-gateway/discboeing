import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const source = readFileSync("ui/src/lib/app/app-runtime.svelte.ts", "utf8");

test("runtime projections mount the selected session before existing contexts", () => {
	assert.match(
		source,
		/ctx\.view\.app\.navigation\.mountedSessionIds = \[\s*ctx\.view\.app\.selection\.sessionId,\s*\.\.\.sessionContexts\.keys\(\),\s*ctx\.view\.app\.selection\.pendingSessionId,\s*\]/,
	);
});
