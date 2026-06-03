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

test("session workspace owns the per-session context for its mount lifetime", () => {
	const source = readSessionWorkspaceSource();

	assert.match(source, /type Props = \{/);
	assert.match(source, /sessionId: string;/);
	assert.match(source, /visible: boolean;/);
	assert.match(source, /mainClass: string;/);
	assert.doesNotMatch(source, /showSidebarToggle\?: boolean;/);
	assert.doesNotMatch(source, /reserveSidebarSpace\?: boolean;/);
	assert.match(source, /import \{ onDestroy, untrack \} from "svelte";/);
	assert.match(
		source,
		/import \{[\s\S]*ensureSessionState,[\s\S]*releaseSessionState,[\s\S]*\} from "\$lib\/context\/commands\/app-view";/,
	);
	assert.doesNotMatch(source, /getAppState/);
	assert.doesNotMatch(source, /legacy-context-bridge/);
	assert.match(
		source,
		/const session = ensureSessionState\(untrack\(\(\) => sessionId\)\);/,
	);
	assert.doesNotMatch(source, /setSessionBridge\(session\);/);
	assert.match(source, /onDestroy\(\(\) => \{/);
	assert.match(source, /releaseSessionState\(session\);/);
	assert.match(source, /const threadId = \$derived\.by\(/);
	assert.match(source, /\{#key threadId\}/);
	assert.match(source, /<ThreadWorkspace/);
	assert.match(source, /\{threadId\}/);
	assert.doesNotMatch(source, /\{reserveSidebarSpace\}/);
	assert.doesNotMatch(source, /\{showSidebarToggle\}/);
	assert.match(
		source,
		/mode=\{session\.isPending \? "conversation-only" : undefined\}/,
	);
	assert.doesNotMatch(source, /new ResizeObserver/);
	assert.doesNotMatch(source, /<SessionSidebar/);
});
