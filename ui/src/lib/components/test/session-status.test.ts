import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

import {
	SessionDisplayStatus,
	SessionStatus,
	isSessionTransitioningStatus,
} from "../../api-constants";

const SESSION_STATUS_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/parts/SessionStatus.svelte",
);
const SESSION_SETUP_STATUS_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/ConversationComposerSessionSetupStatus.svelte",
);

function readSessionStatusSource() {
	return readFileSync(SESSION_STATUS_COMPONENT, "utf-8");
}

function readSessionSetupStatusSource() {
	return readFileSync(SESSION_SETUP_STATUS_COMPONENT, "utf-8");
}

test("session display status constants include committed", () => {
	assert.equal(SessionDisplayStatus.COMMITTED, "committed");
});

test("session transitioning status helper only flags non-resting lifecycle states", () => {
	assert.equal(isSessionTransitioningStatus(SessionStatus.INITIALIZING), true);
	assert.equal(
		isSessionTransitioningStatus(SessionStatus.CREATING_SANDBOX),
		true,
	);
	assert.equal(isSessionTransitioningStatus(SessionStatus.REMOVING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.ERROR), false);
	assert.equal(
		isSessionTransitioningStatus(SessionStatus.CREATE_FAILED),
		false,
	);
	assert.equal(isSessionTransitioningStatus(SessionStatus.STOPPED), false);
	assert.equal(isSessionTransitioningStatus(SessionStatus.READY), false);
	assert.equal(isSessionTransitioningStatus(null), false);
});

test("session status component renders a dedicated git icon for committed", () => {
	const source = readSessionStatusSource();

	assert.match(
		source,
		/import GitCommitIcon from "@lucide\/svelte\/icons\/git-commit"/,
	);
	assert.match(source, /normalizedStatus\(status\) === "committed"/);
	assert.match(source, /<GitCommitIcon class="size-3\.5" \/>/);
});

test("session setup status shows creation while pending chat is starting", () => {
	const source = readSessionSetupStatusSource();

	assert.match(source, /const thread = useThreadContext\(\);/);
	assert.match(
		source,
		/const pendingSessionStarted = \$derived\.by\(\n\t\t\(\) => session\.isPending && thread\.isStreaming,\n\t\);/,
	);
	assert.match(source, /\{#if pendingSessionStarted && !sessionStatus\}/);
	assert.match(source, /<span>Creating session<\/span>/);
	assert.doesNotMatch(source, /Restoring session/);
});

test("session setup status renders sandbox progress below status", () => {
	const source = readSessionSetupStatusSource();

	assert.match(source, /session\.current\?\.sandboxStatusMessage\?\.trim\(\)/);
	assert.match(source, /\{:else if sessionStatusMessage\}/);
	assert.match(source, /title=\{sessionStatusMessage\}/);
	assert.match(source, /\{sessionStatusMessage\}/);
});
