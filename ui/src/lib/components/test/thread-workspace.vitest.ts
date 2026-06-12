import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "vitest";

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
	assert.match(source, /const context = useContext\(\);/);
	assert.match(source, /const sessionRecord = \$derived/);
	assert.match(source, /const threadRecord = \$derived/);
	assert.doesNotMatch(source, /ensureRuntimeThread|getThreadRuntimeSnapshot/);
	assert.doesNotMatch(source, /session\.ensureThread/);
	assert.doesNotMatch(
		source,
		/ThreadRuntimeController|ensureRuntimeThreadState/,
	);
	assert.doesNotMatch(source, /\$lib\/ng/);
	assert.doesNotMatch(source, /setThreadBridge\(thread\);/);
	assert.match(source, /<ThreadWorkspaceActive/);
	assert.match(source, /const hasSelectedThread = \$derived\.by/);
	assert.match(
		source,
		/\(\) =>\s*isPendingSession \|\|\s*selectedThreadId !== null \|\|\s*isSessionTransitioningStatus\(currentSession\?\.sandboxStatus\)/,
	);
	assert.match(
		source,
		/const hasConversationMessages = \$derived\.by\(\s*\(\) => \(threadRecord\?\.content\.messages\.length \?\? 0\) > 0,\s*\);/,
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
		/isSessionTransitioningStatus\(currentSession\?\.sandboxStatus\)/,
	);
	assert.match(
		source,
		/getCacheStatus\(sessionThreads\?\.status\) === "loading"/,
	);
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
		/const headerTitle = \$derived\.by\(\(\) => selectedThread\?\.name \?\? ""\);/,
	);
	assert.match(
		source,
		/const sessionTitle = \$derived\.by\(\s*\(\) => currentSession\?\.displayName \|\| currentSession\?\.name \|\| "Sessions",\s*\);/,
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

test("active thread workspace keeps the stream live while inactive conversation nodes are unmounted", () => {
	const source = readThreadWorkspaceActiveSource();

	assert.match(
		source,
		/import ThreadActivation from "\$lib\/components\/app\/ThreadActivation\.svelte";/,
	);
	assert.match(
		source,
		/\{#if props\.visible && currentSession\}[\s\S]*<ThreadActivation \{sessionId\} \{threadId\} \/>[\s\S]*\{\/if\}/,
	);
	assert.doesNotMatch(source, /activateThread\(sessionId, threadId\)/);
	assert.doesNotMatch(
		source,
		/\$effect\(\(\) => \{[\s\S]*void context\.commands(?:\.threads)?\s*\.activateThread\(sessionId, threadId\)\s*\.catch\(\(\) => undefined\);[\s\S]*\}\);/,
	);
	assert.match(source, /const headerTitle = \$derived\.by/);
	assert.match(source, /if \(!currentSession\) \{\s*return "";/);
	assert.match(source, /return "";\s*\}\);/);
	assert.match(source, /title=\{headerTitle\}/);
	assert.match(
		source,
		/const sessionTitle = \$derived\.by\(\s*\(\) => currentSession\?\.displayName \|\| currentSession\?\.name \|\| "Sessions",\s*\);/,
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
		/<ConversationPane[\s\S]*\{sessionId\}[\s\S]*\{threadId\}[\s\S]*visible=\{props\.visible\}/,
	);
	assert.match(
		source,
		/<ConversationPane \{sessionId\} \{threadId\} visible=\{props\.visible\} \/>/,
	);
});
