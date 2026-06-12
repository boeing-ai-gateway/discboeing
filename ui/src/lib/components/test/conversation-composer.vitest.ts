import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const CONVERSATION_COMPOSER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ConversationComposer.svelte",
);

function readConversationComposerSource() {
	return readFileSync(CONVERSATION_COMPOSER_COMPONENT, "utf-8");
}

test("pending composer submit opens the created thread", () => {
	const source = readConversationComposerSource();

	assert.match(source, /const wasPending = isPending;/);
	assert.match(
		source,
		/const result = await context\.commands\.threadComposer\.submitThread\(\s*sessionId,\s*threadId,\s*\{/,
	);
	assert.match(source, /if \(wasPending && result\) \{/);
	assert.match(
		source,
		/await context\.commands\.navigation\.openThread\(\s*result\.sessionId,\s*result\.threadId,\s*\);/,
	);
	assert.doesNotMatch(source, /app\.sessions\.openThread/);
	assert.doesNotMatch(source, /\$lib\/context\/runtime/);
});

test("composer updates session UI state through commands", () => {
	const source = readConversationComposerSource();

	assert.match(
		source,
		/context\.commands\.view\.setSessionHooksExpanded\(sessionId, expanded\)/,
	);
	assert.match(
		source,
		/context\.commands\.view\.setPendingWorkspaceSandboxProviderId\(\s*sessionId,\s*providerId,\s*\)/,
	);
	assert.match(
		source,
		/context\.commands\.view\.resetPendingWorkspaceSetup\(sessionId\)/,
	);
	assert.doesNotMatch(source, /sessionView\.hooks\.expanded =/);
	assert.doesNotMatch(source, /sessionView\.pendingWorkspace\.[a-zA-Z]+ =/);
});
