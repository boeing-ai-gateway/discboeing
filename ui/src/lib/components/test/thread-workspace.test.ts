import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const THREAD_WORKSPACE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ThreadWorkspace.svelte",
);
const THREAD_WORKSPACE_ACTIVE_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ThreadWorkspaceActive.svelte",
);

function readThreadWorkspaceSource() {
	return readFileSync(THREAD_WORKSPACE_COMPONENT, "utf-8");
}

function readThreadWorkspaceActiveSource() {
	return readFileSync(THREAD_WORKSPACE_ACTIVE_COMPONENT, "utf-8");
}

test("thread workspace keeps pending sessions on the active conversation view and avoids the loading screen once messages exist", () => {
	const source = readThreadWorkspaceSource();

	assert.doesNotMatch(source, /let sessionsMenuOpen = \$state\(false\)/);
	assert.doesNotMatch(source, /<SessionSidebar/);
	assert.doesNotMatch(source, /<Popover\.Root/);
	assert.match(source, /type Props = \{/);
	assert.match(source, /threadId: string;/);
	assert.match(source, /sidebarOpen\?: boolean;/);
	assert.match(source, /let \{/);
	assert.match(source, /\}: Props = \$props\(\);/);
	assert.match(
		source,
		/const thread = session\.ensureThread\(untrack\(\(\) => threadId\)\);/,
	);
	assert.match(source, /setThreadContext\(thread\);/);
	assert.match(source, /<ThreadWorkspaceActive/);
	assert.match(source, /const hasSelectedThread = \$derived\.by/);
	assert.match(
		source,
		/session\.isPending \|\|\s*\(sandboxReady && session\.threads\.selectedId !== null\)/,
	);
	assert.match(
		source,
		/const hasConversationMessages = \$derived\.by\(\(\) => thread\.messages\.length > 0\);/,
	);
	assert.match(source, /const showActiveConversation = \$derived\.by/);
	assert.match(
		source,
		/\(\) => hasSelectedThread \|\| hasConversationMessages/,
	);
	assert.match(source, /const sandboxReady = \$derived\.by/);
	assert.match(source, /const isLoadingThread = \$derived\.by/);
	assert.match(
		source,
		/import \{ isSessionTransitioningStatus \} from "\$lib\/api-constants"/,
	);
	assert.match(
		source,
		/isSessionTransitioningStatus\(session\.current\?\.status\)/,
	);
	assert.doesNotMatch(
		source,
		/!showActiveConversation && !session\.isPending && !sandboxReady/,
	);
	assert.match(source, /const showThreadSelectionPrompt = \$derived\.by/);
	assert.match(source, /<ConversationComposerSessionSetupStatus/);
	assert.match(
		source,
		/import Loader2Icon from "@lucide\/svelte\/icons\/loader-2"/,
	);
	assert.match(source, /<Loader2Icon class="size-4 animate-spin" \/>/);
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

test("thread context stops sandbox refreshes when a session is not ready", () => {
	const source = readFileSync(
		path.resolve(import.meta.dirname, "../../context/thread-context.svelte.ts"),
		"utf-8",
	);

	assert.match(
		source,
		/const refreshSessionState = async \(\) => \{\s*if \(!hasSession\) \{/,
	);
	assert.match(
		source,
		/retryScheduler\.dispose\(\);\s*conversation\.disconnect\(\);/,
	);
	assert.match(source, /afterTurn: async \(\) => \{\s*if \(!hasSession\) \{/);
});

test("active thread workspace keeps the stream live while inactive conversation nodes are unmounted", () => {
	const source = readThreadWorkspaceActiveSource();

	assert.match(source, /void thread\.connect\(\);/);
	assert.match(
		source,
		/\$effect\(\(\) => \{[\s\S]*void thread\.connect\(\);[\s\S]*\}\);/,
	);
	assert.match(
		source,
		/\{#if props\.visible\}\s*<ConversationPane visible=\{props\.visible\} \/>\s*\{\/if\}/,
	);
	assert.equal(
		source.match(/<ConversationPane visible=\{props\.visible\} \/>/g)?.length,
		2,
	);
});
