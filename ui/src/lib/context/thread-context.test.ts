import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const THREAD_CONTEXT_SOURCE = path.resolve(
	import.meta.dirname,
	"./thread-context.svelte.ts",
);

test("thread context treats startup transitions as streaming", () => {
	const source = readFileSync(THREAD_CONTEXT_SOURCE, "utf-8");

	assert.match(
		source,
		/isSessionTransitioningStatus\(session\.current\?\.status\)/,
	);
	assert.match(
		source,
		/!hasSession &&[\s\S]*isSessionTransitioningStatus\(session\.current\?\.status\)/,
	);
});
