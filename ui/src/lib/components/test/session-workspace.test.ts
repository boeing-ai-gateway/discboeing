import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const SESSION_WORKSPACE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/SessionWorkspace.svelte",
);

function readSessionWorkspaceSource() {
	return readFileSync(SESSION_WORKSPACE_COMPONENT, "utf-8");
}

test("session workspace is a per-session mounted wrapper around thread workspace", () => {
	const source = readSessionWorkspaceSource();

	assert.match(source, /type Props = \{/);
	assert.match(source, /sessionId: string;/);
	assert.match(source, /visible: boolean;/);
	assert.match(source, /mainClass: string;/);
	assert.match(source, /useSessionContext\(untrack\(\(\) => sessionId\)\)/);
	assert.match(source, /const threadId = \$derived\.by\(/);
	assert.match(source, /\{#key threadId\}/);
	assert.match(source, /<ThreadWorkspace/);
	assert.match(source, /\{threadId\}/);
	assert.match(
		source,
		/mode=\{session\.isPending \? "conversation-only" : undefined\}/,
	);
	assert.doesNotMatch(source, /new ResizeObserver/);
	assert.doesNotMatch(source, /<SessionSidebar/);
});
