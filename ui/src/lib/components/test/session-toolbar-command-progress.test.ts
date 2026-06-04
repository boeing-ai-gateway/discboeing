import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url));
const SESSION_TOOLBAR_COMPONENT = path.resolve(
	TEST_DIR,
	"../app/SessionToolbar.svelte",
);
const SESSION_COMMANDS_DOMAIN = path.resolve(
	TEST_DIR,
	"../../session/domains/session-commands.svelte.ts",
);
const SESSION_CONTEXT = path.resolve(
	TEST_DIR,
	"../../session/create-session-state.svelte.ts",
);
const COMMIT_COMMAND = path.resolve(
	TEST_DIR,
	"../../../../../container-assets/discobot/scripts/discobot-commit",
);
const COMMIT_REMOTE_COMMAND = path.resolve(
	TEST_DIR,
	"../../../../../container-assets/discobot/scripts/discobot-commit-remote",
);
const REBASE_COMMAND = path.resolve(
	TEST_DIR,
	"../../../../../container-assets/discobot/scripts/discobot-rebase",
);

function readSessionToolbarSource() {
	return readFileSync(SESSION_TOOLBAR_COMPONENT, "utf-8");
}

function readSessionCommandsSource() {
	return readFileSync(SESSION_COMMANDS_DOMAIN, "utf-8");
}

function readSessionContextSource() {
	return readFileSync(SESSION_CONTEXT, "utf-8");
}

function readCommandSource(filePath: string) {
	return readFileSync(filePath, "utf-8");
}

test("session toolbar keeps command progress bound to the active command", () => {
	const source = readSessionToolbarSource();

	assert.match(source, /function commandBusy\(command: AgentCommand\)/);
	assert.match(
		source,
		/\{#if commandBusy\(command\)\}[\s\S]*<Loader2Icon class="size-3\.5 animate-spin" \/>/,
	);
	assert.doesNotMatch(source, /session\.commands\.runningName/);
});

test("session toolbar only renders command icons when command metadata specifies one", () => {
	const source = readSessionToolbarSource();

	assert.match(
		source,
		/function commandIcon\(command: AgentCommand\): LucideIcon \| null/,
	);
	assert.match(source, /if \(!iconName\) \{\s*return null;\s*\}/);
	assert.match(
		source,
		/staticCommandIcons\[iconName\] \?\? loadedCommandIcons\[iconName\] \?\? null/,
	);
	assert.match(
		source,
		/\{#if PrimaryIcon\}[\s\S]*<PrimaryIcon class="size-3\.5" \/>[\s\S]*\{\/if\}/,
	);
	assert.match(
		source,
		/\{#if Icon\}[\s\S]*<Icon class="size-3\.5" \/>[\s\S]*\{\/if\}/,
	);
	assert.doesNotMatch(source, /PlayIcon/);
});

test("session toolbar groups dropdown commands from command metadata", () => {
	const source = readSessionToolbarSource();

	assert.match(source, /command\.discobot\?\.group\?\.trim\(\) \|\| null/);
	assert.match(source, /<DropdownMenuSeparator \/>/);
	assert.match(source, /{#if group\.label}/);
	assert.doesNotMatch(source, /Git actions/);
	assert.doesNotMatch(source, /More git actions/);
});

test("session toolbar normalizes empty activeCommand to no running command", () => {
	const source = readSessionToolbarSource();

	assert.match(source, /function normalizeActiveCommandName\(/);
	assert.match(source, /session\.threads\.selected\?\.activeCommand/);
	assert.match(source, /const trimmed = name\?\.trim\(\) \?\? "";/);
	assert.match(source, /return trimmed\.length > 0 \? trimmed : null;/);
});

test("commit and rebase bundled scripts specify the expected lucide icons and Git group", () => {
	const commit = readCommandSource(COMMIT_COMMAND);
	const commitRemote = readCommandSource(COMMIT_REMOTE_COMMAND);
	const rebase = readCommandSource(REBASE_COMMAND);

	assert.match(commit, /discobot-icon: git-commit/);
	assert.match(commitRemote, /discobot-icon: git-commit/);
	assert.match(rebase, /discobot-icon: git-branch/);
	assert.match(commit, /discobot-group: Git/);
	assert.match(commitRemote, /discobot-group: Git/);
	assert.match(rebase, /discobot-group: Git/);
});

test("session toolbar falls back to the active command name while command metadata is still loading", () => {
	const source = readSessionToolbarSource();

	assert.match(
		source,
		/uiCommands\.find\(\(command\) => command\.name === activeCommandName\) \?\? null/,
	);
	assert.match(source, /`\$\{activeCommandName\}\.\.\.`/);
	assert.doesNotMatch(source, /Working\.\.\./);
	assert.doesNotMatch(
		source,
		/session\.commands\.credentialDialog\.command\?\.name/,
	);
});

test("session toolbar does not use committing status alone as command progress", () => {
	const source = readSessionToolbarSource();

	assert.doesNotMatch(source, /session\.current\?\.status === "committing"/);
	assert.match(
		source,
		/const showBusy = activeCommandName !== null \|\| isPending;/,
	);
});

test("session toolbar disables actions while a submission or credential dialog is in flight", () => {
	const source = readSessionToolbarSource();

	assert.match(source, /commandData\?\.isSubmitting \?\? false/);
	assert.match(source, /commandView\?\.credentialDialog\.open \?\? false/);
});

test("session commands only track submit-in-flight state locally", () => {
	const source = readSessionCommandsSource();

	assert.match(source, /let isSubmitting = \$state\(false\);/);
	assert.match(source, /const threadId = args\.getSelectedThreadId\(\);/);
	assert.match(source, /await args\.submit\(text, \{ threadId \}\);/);
	assert.match(source, /isSubmitting = true;/);
	assert.match(source, /isSubmitting = false;/);
	assert.doesNotMatch(source, /runningName/);
	assert.doesNotMatch(source, /runningThreadId/);
	assert.doesNotMatch(source, /hasObservedRunningThread/);
	assert.doesNotMatch(source, /getThreadActivity/);
});

test("session context no longer wires thread activity into command progress tracking", () => {
	const source = readSessionContextSource();

	assert.match(
		source,
		/getSelectedThreadId: \(\) => threads\.selectedId \?\? sessionId,/,
	);
	assert.match(source, /submit,/);
	assert.doesNotMatch(source, /getThreadActivity:/);
});
