import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

const CONVERSATION_COMPOSER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ConversationComposer.svelte",
);
const CONVERSATION_COMPOSER_SESSION_SETUP_STATUS_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ConversationComposerSessionSetupStatus.svelte",
);

function readConversationComposerSource() {
	return readFileSync(CONVERSATION_COMPOSER_COMPONENT, "utf-8");
}

function readConversationComposerSessionSetupStatusSource() {
	return readFileSync(
		CONVERSATION_COMPOSER_SESSION_SETUP_STATUS_COMPONENT,
		"utf-8",
	);
}

test("pending composer submit completes the pending session", () => {
	const source = readConversationComposerSource();

	assert.match(source, /const wasPending = isPending;/);
	assert.match(
		source,
		/const result = await context\.commands\.threadComposer\.submitThread\(\s*sessionId,\s*threadId,\s*\{/,
	);
	assert.match(source, /if \(wasPending && result\) \{/);
	assert.match(
		source,
		/await context\.commands\.navigation\.completePendingSession\(\s*sessionId,\s*result\.sessionId,\s*\);/,
	);
	assert.doesNotMatch(source, /context\.commands\.navigation\.openThread/);
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

test("pending composer UI excludes the selected real session", () => {
	const pendingPredicate =
		/context\.view\.selection\.pendingSessionId === sessionId &&\s*context\.view\.selection\.sessionId !== sessionId/;

	assert.match(readConversationComposerSource(), pendingPredicate);
	assert.match(
		readConversationComposerSessionSetupStatusSource(),
		pendingPredicate,
	);
});
