import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const THREAD_WORKSPACE_ACTIVE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ThreadWorkspaceActive.svelte",
);
const THREAD_ACTIVATION_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ThreadActivation.svelte",
);

function readThreadWorkspaceActiveSource() {
	return readFileSync(THREAD_WORKSPACE_ACTIVE_COMPONENT, "utf-8");
}

function readThreadActivationSource() {
	return readFileSync(THREAD_ACTIVATION_COMPONENT, "utf-8");
}

test("thread workspace active mounts activation when visible with a session", () => {
	const source = readThreadWorkspaceActiveSource();

	assert.match(source, /import \{ useContext \} from "\$lib\/context";/);
	assert.match(source, /const context = useContext\(\);/);
	assert.match(
		source,
		/import ThreadActivation from "\$lib\/components\/app\/ThreadActivation\.svelte";/,
	);
	assert.match(
		source,
		/\{#if props\.visible && currentSession\}[\s\S]*\{#key `\$\{sessionId\}:\$\{threadId\}`\}[\s\S]*<ThreadActivation \{sessionId\} \{threadId\} \/>[\s\S]*\{\/key\}[\s\S]*\{\/if\}/,
	);
	assert.doesNotMatch(source, /activateThread\(sessionId, threadId\)/);
	assert.doesNotMatch(source, /deactivateThread\(sessionId, threadId\)/);
	assert.doesNotMatch(
		source,
		/\$effect\(\(\) => \{[\s\S]*context\.commands(?:\.threads)?\.activateThread/,
	);
	assert.doesNotMatch(source, /connectRuntimeThread|releaseRuntimeThreadState/);
	assert.doesNotMatch(source, /\$lib\/context\/runtime/);
	assert.doesNotMatch(source, /SessionRuntimeState/);
});

test("thread activation does not deactivate on unmount", () => {
	const source = readThreadActivationSource();

	assert.match(source, /import \{ onMount \} from "svelte";/);
	assert.match(source, /import \{ useContext \} from "\$lib\/context";/);
	assert.match(
		source,
		/void context\.commands\.threads\s*\.activateThread\(sessionId, threadId\)\s*\.catch\(\(\) => undefined\);/,
	);
	assert.doesNotMatch(source, /\$effect/);
	assert.doesNotMatch(source, /onDestroy/);
	assert.doesNotMatch(source, /activationStarted/);
	assert.doesNotMatch(
		source,
		/context\.commands\.threads\s*\.deactivateThread\(sessionId, threadId\)/,
	);
	assert.doesNotMatch(
		source,
		/await context\.commands\.threads\.activateThread/,
	);
});

test("thread workspace active passes ids into dock panel", () => {
	const source = readThreadWorkspaceActiveSource();

	assert.match(source, /<DockPanel \{sessionId\} \{threadId\} \/>/);
	assert.doesNotMatch(source, /sessionView=\{session\.ui\}/);
	assert.doesNotMatch(source, /<DockPanel \{session\} \{thread\}/);
});
