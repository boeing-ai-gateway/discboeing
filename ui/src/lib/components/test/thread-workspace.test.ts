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
	assert.match(source, /\}: Props =\s*\$props\(\);/);
	assert.match(
		source,
		/const thread: ThreadContextValue = untrack\(\(\) =>\s*ensureThreadState\(session\.sessionId, threadId\),\s*\);/,
	);
	assert.doesNotMatch(source, /session\.ensureThread/);
	assert.doesNotMatch(source, /legacy-context-bridge/);
	assert.doesNotMatch(source, /setThreadBridge\(thread\);/);
	assert.match(source, /<ThreadWorkspaceActive/);
	assert.match(source, /const hasSelectedThread = \$derived\.by/);
	assert.match(
		source,
		/session\.isPending \|\|\s*session\.threads\.selectedId !== null \|\|\s*isSessionTransitioningStatus\(session\.current\?\.sandboxStatus\)/,
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
	assert.match(source, /const canLoadThreadData = \$derived\.by/);
	assert.match(source, /const isLoadingThread = \$derived\.by/);
	assert.match(
		source,
		/import \{[\s\S]*canLoadSessionThreads,[\s\S]*isSessionTransitioningStatus,[\s\S]*\} from "\$lib\/api-constants"/,
	);
	assert.match(
		source,
		/isSessionTransitioningStatus\(session\.current\?\.sandboxStatus\)/,
	);
	assert.match(source, /session\.threads\.status === "loading"/);
	assert.doesNotMatch(
		source,
		/!showActiveConversation && !session\.isPending && !sandboxReady/,
	);
	assert.match(source, /const showThreadSelectionPrompt = \$derived\.by/);
	assert.match(
		source,
		/\(\) => !isLoadingThread && !showActiveConversation && canLoadThreadData/,
	);
	assert.match(source, /<ConversationComposerSessionSetupStatus/);
	assert.match(
		source,
		/import Loader2Icon from "@lucide\/svelte\/icons\/loader-2"/,
	);
	assert.match(source, /<Loader2Icon class="size-4 animate-spin" \/>/);
	assert.match(
		source,
		/const headerTitle = \$derived\.by\(\(\) => session\.threads\.selected\?\.name \?\? ""\);/,
	);
	assert.match(
		source,
		/const sessionTitle = \$derived\.by\(\s*\(\) => session\.current\?\.displayName \|\| session\.current\?\.name \|\| "Sessions",\s*\);/,
	);
	assert.match(source, /import SessionHeaderDropdown/);
	assert.match(source, /\{#snippet sessionHeaderDropdown\(\)\}/);
	assert.match(source, /<SessionHeaderDropdown label=\{sessionTitle\} \/>/);
	assert.match(source, /titleContent=\{sessionHeaderDropdown\}/);
	assert.doesNotMatch(source, /const isConversationOnly = \$derived/);
	assert.doesNotMatch(source, /const headerTitleContent = \$derived/);
	assert.doesNotMatch(source, /displayThreadDropdown=\{isConversationOnly\}/);
	assert.doesNotMatch(source, /\{threadList\}/);
	assert.doesNotMatch(source, /\{selectedThreadId\}/);
	assert.doesNotMatch(source, /onThreadChange=\{handleThreadChange\}/);
	assert.match(
		source,
		/Loading the selected thread while the session starts\./,
	);
	assert.match(source, /Select a thread to continue\./);
});

test("thread context stops sandbox refreshes when a session is not ready", () => {
	const source = readFileSync(
		path.resolve(
			import.meta.dirname,
			"../../thread/create-thread-state.svelte.ts",
		),
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

	assert.match(source, /if \(!props\.visible \|\| !session\.current\) \{/);
	assert.match(
		source,
		/connectThread\(session\.sessionId, thread\.threadId\);/,
	);
	assert.match(
		source,
		/\$effect\(\(\) => \{[\s\S]*connectThread\(session\.sessionId, thread\.threadId\);[\s\S]*\}\);/,
	);
	assert.match(source, /const headerTitle = \$derived\.by/);
	assert.match(source, /if \(session\.isPending\) \{\s*return "";/);
	assert.match(source, /return "";\s*\}\);/);
	assert.match(source, /title=\{headerTitle\}/);
	assert.match(
		source,
		/const sessionTitle = \$derived\.by\(\s*\(\) => session\.current\?\.displayName \|\| session\.current\?\.name \|\| "Sessions",\s*\);/,
	);
	assert.match(source, /import SessionHeaderDropdown/);
	assert.match(source, /\{#snippet sessionHeaderDropdown\(\)\}/);
	assert.match(source, /<SessionHeaderDropdown label=\{sessionTitle\} \/>/);
	assert.match(source, /titleContent=\{sessionHeaderDropdown\}/);
	assert.doesNotMatch(source, /const isConversationOnly = \$derived/);
	assert.doesNotMatch(source, /const headerTitleContent = \$derived/);
	assert.doesNotMatch(source, /displayThreadDropdown=\{isConversationOnly\}/);
	assert.doesNotMatch(source, /\{threadList\}/);
	assert.doesNotMatch(source, /\{selectedThreadId\}/);
	assert.doesNotMatch(source, /onThreadChange=\{handleThreadChange\}/);
	assert.doesNotMatch(
		source,
		/title=\{session\.threads\.selected\?\.name \?\?/,
	);
	assert.equal(source.match(/<ConversationPane/g)?.length, 2);
	assert.match(
		source,
		/<ConversationPane[\s\S]*\{session\}[\s\S]*\{thread\}[\s\S]*visible=\{props\.visible\}/,
	);
	assert.match(
		source,
		/<ConversationPane \{session\} \{thread\} visible=\{props\.visible\} \/>/,
	);
});
