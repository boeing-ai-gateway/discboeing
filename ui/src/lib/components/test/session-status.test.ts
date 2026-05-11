import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

import {
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

test("session status constants include committed", () => {
	assert.equal(SessionStatus.COMMITTED, "committed");
});

test("session transitioning status helper only flags non-resting states", () => {
	assert.equal(isSessionTransitioningStatus(SessionStatus.INITIALIZING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.PENDING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.COMMITTING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.REMOVING), true);
	assert.equal(isSessionTransitioningStatus(SessionStatus.ERROR), false);
	assert.equal(
		isSessionTransitioningStatus(SessionStatus.CREATE_FAILED),
		false,
	);
	assert.equal(isSessionTransitioningStatus(SessionStatus.STOPPED), false);
	assert.equal(isSessionTransitioningStatus(SessionStatus.COMMITTED), false);
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

test("session setup status distinguishes creating from restoring", () => {
	const source = readSessionSetupStatusSource();

	assert.match(
		source,
		/session\.isPending \? "Creating session" : "Restoring session"/,
	);
});
