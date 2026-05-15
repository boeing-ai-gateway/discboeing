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

test("thread workspace owns only thread context and conversation UI", () => {
	const source = readThreadWorkspaceSource();

	assert.doesNotMatch(source, /let sessionsMenuOpen = \$state\(false\)/);
	assert.doesNotMatch(source, /<SessionSidebar/);
	assert.doesNotMatch(source, /<Popover\.Root/);
	assert.match(source, /type Props = \{/);
	assert.match(source, /threadId: string;/);
	assert.match(source, /visible: boolean;/);
	assert.match(source, /reserveSidebarSpace\?: boolean;/);
	assert.match(source, /mode\?: "conversation" \| "connection-only";/);
	assert.match(source, /let \{/);
	assert.match(source, /mode = "conversation",/);
	assert.match(source, /\}: Props =\s*\$props\(\);/);
	assert.match(
		source,
		/const thread = session\.ensureThread\(untrack\(\(\) => threadId\)\);/,
	);
	assert.match(source, /setThreadContext\(thread\);/);
	assert.match(source, /if \(!visible \|\| !session\.current\) \{/);
	assert.match(source, /void thread\.start\(\);/);
	assert.match(source, /onDestroy\(\(\) => \{/);
	assert.match(source, /thread\.dispose\(\);/);
	assert.match(source, /const headerTitle = \$derived\.by/);
	assert.match(source, /if \(session\.isPending\) \{\s*return "";/);
	assert.match(
		source,
		/isSessionTransitioningStatus\(session\.current\?\.sandboxStatus\)[\s\S]*\? "Loading thread"[\s\S]*: "No thread selected"/,
	);
	assert.match(source, /\{#if mode === "conversation"\}/);
	assert.match(source, /<ThreadWorkspaceHeader/);
	assert.match(source, /title=\{headerTitle\}/);
	assert.match(source, /<ConversationPane \{visible\} \/>/);
	assert.doesNotMatch(source, /ThreadWorkspaceActive/);
	assert.doesNotMatch(source, /DockPanel/);
	assert.doesNotMatch(source, /const hasSelectedThread = \$derived\.by/);
	assert.doesNotMatch(source, /const hasConversationMessages = \$derived\.by/);
	assert.doesNotMatch(source, /const showActiveConversation = \$derived\.by/);
	assert.doesNotMatch(source, /const isLoadingThread = \$derived\.by/);
	assert.doesNotMatch(source, /canLoadSessionThreads/);
	assert.doesNotMatch(source, /session\.threads\.status === "idle"/);
	assert.doesNotMatch(source, /session\.threads\.status === "loading"/);
	assert.doesNotMatch(
		source,
		/const showThreadSelectionPrompt = \$derived\.by/,
	);
	assert.doesNotMatch(source, /Select a thread to continue\./);
	assert.doesNotMatch(source, /<ConversationComposerSessionSetupStatus/);
	assert.doesNotMatch(
		source,
		/import Loader2Icon from "@lucide\/svelte\/icons\/loader-2"/,
	);
	assert.doesNotMatch(source, /<Loader2Icon class="size-4 animate-spin" \/>/);
	assert.doesNotMatch(
		source,
		/title=\{session\.threads\.selected\?\.name \?\?/,
	);
	assert.doesNotMatch(
		source,
		/Loading the selected thread while the session starts\./,
	);
});

test("thread context refreshes session state without its own retry scheduler", () => {
	const source = readFileSync(
		path.resolve(import.meta.dirname, "../../context/thread-context.svelte.ts"),
		"utf-8",
	);

	assert.match(source, /const refreshSessionState = async \(\) => \{/);
	assert.doesNotMatch(source, /hasSession/);
	assert.match(source, /session\.files\.refresh\(\)\.catch\(\(\) => \{\}\)/);
	assert.match(source, /app\.sessions\.reloadSession\(session\.sessionId\)/);
	assert.doesNotMatch(source, /createRetryScheduler/);
	assert.doesNotMatch(source, /retryScheduler/);
	assert.match(source, /conversation\.dispose\(\);/);
	assert.doesNotMatch(source, /conversation\.disconnect\(\);/);
	assert.match(
		source,
		/afterTurn: async \(\) => \{\s*await session\.threads\.refreshThread\(threadId\);/,
	);
});

test("thread workspace keeps the stream live while conversation nodes are hidden", () => {
	const source = readThreadWorkspaceSource();

	assert.match(
		source,
		/\$effect\(\(\) => \{[\s\S]*void thread\.start\(\);[\s\S]*\}\);/,
	);
	assert.match(source, /\{#if mode === "conversation"\}/);
	assert.match(source, /<ConversationPane \{visible\} \/>/);
});
