import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const THREAD_WORKSPACE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ThreadWorkspace.svelte",
);

function readThreadWorkspaceSource() {
	return readFileSync(THREAD_WORKSPACE_COMPONENT, "utf-8");
}

test("thread workspace keeps pending sessions on the active conversation view and shows loading only for unresolved ready sessions", () => {
	const source = readThreadWorkspaceSource();

	assert.doesNotMatch(source, /let sessionsMenuOpen = \$state\(false\)/);
	assert.doesNotMatch(source, /<SessionSidebar/);
	assert.doesNotMatch(source, /<Popover\.Root/);
	assert.match(source, /const props: Props = \$props\(\);/);
	assert.match(source, /threadId: string;/);
	assert.match(source, /sidebarOpen\?: boolean;/);
	assert.match(source, /<ThreadWorkspaceActive/);
	assert.match(source, /const hasSelectedThread = \$derived\.by/);
	assert.match(
		source,
		/session\.isPending \|\| session\.threads\.selectedId !== null/,
	);
	assert.match(source, /const sandboxReady = \$derived\.by/);
	assert.match(source, /const isLoadingThread = \$derived\.by/);
	assert.match(
		source,
		/\(\) => !session\.isPending && !hasSelectedThread && !sandboxReady/,
	);
	assert.match(source, /const showThreadSelectionPrompt = \$derived\.by/);
	assert.match(source, /<ConversationComposerSessionSetupStatus/);
	assert.match(
		source,
		/title=\{isLoadingThread \? "Loading thread" : "No thread selected"\}/,
	);
	assert.match(
		source,
		/Loading the selected thread while the session starts\./,
	);
	assert.match(source, /Select a thread to continue\./);
});
