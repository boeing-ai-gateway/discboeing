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
		/isSessionTransitioningStatus\(session\.current\?\.sandboxStatus\)/,
	);
	assert.doesNotMatch(source, /hasSession/);
	assert.doesNotMatch(source, /canStreamConversation/);
	assert.doesNotMatch(source, /SessionStatus\.READY/);
	assert.match(
		source,
		/isSessionTransitioningStatus\(session\.current\?\.sandboxStatus\) \|\|[\s\S]*conversation\.messages\.some/,
	);
});

test("thread context does not refresh the conversation before submit", () => {
	const source = readFileSync(THREAD_CONTEXT_SOURCE, "utf-8");
	const submitStart = source.indexOf(
		'const submit: ThreadContextValue["submit"]',
	);
	const submitCall = source.indexOf(
		"result = await conversation.submit",
		submitStart,
	);
	const beforeSubmitCall = source.slice(submitStart, submitCall);

	assert.ok(submitStart >= 0);
	assert.ok(submitCall > submitStart);
	assert.doesNotMatch(beforeSubmitCall, /refreshAfterSubmitError/);
	assert.doesNotMatch(beforeSubmitCall, /conversation\.dispose\(\)/);
	assert.match(
		source,
		/catch \(error\) \{[\s\S]*void refreshAfterSubmitError\(\);/,
	);
});
