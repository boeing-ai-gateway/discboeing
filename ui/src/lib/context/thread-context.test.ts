import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const THREAD_CONTEXT_SOURCE = path.resolve(
	import.meta.dirname,
	"../thread/create-thread-state.svelte.ts",
);

test("thread context treats startup transitions as streaming", () => {
	const source = readFileSync(THREAD_CONTEXT_SOURCE, "utf-8");

	assert.match(
		source,
		/isSessionTransitioningStatus\(session\.current\?\.sandboxStatus\)/,
	);
	assert.match(
		source,
		/!hasSession &&[\s\S]*isSessionTransitioningStatus\(session\.current\?\.sandboxStatus\)/,
	);
});

test("thread context keeps matching session running status streaming", () => {
	const source = readFileSync(THREAD_CONTEXT_SOURCE, "utf-8");

	assert.match(
		source,
		/export function isSessionThreadStatusRunningForThread\([\s\S]*status\?\.status === "running" && status\.threadId === threadId;/,
	);
	assert.match(
		source,
		/getThreadIsStreaming\(getThread\(\), conversation\.isStreaming\) \|\|[\s\S]*isSessionThreadStatusRunningForThread\([\s\S]*session\.current\?\.threadStatus,[\s\S]*threadId,/,
	);
});
